package model

import (
	"errors"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	ProductOrderPaymentWallet  = "wallet"
	ProductOrderPaymentQRCode  = "qr_code"
	ProductOrderPaymentPending = "pending"
	ProductOrderPaymentPaid    = "paid"

	ProductOrderFulfillmentPending   = "pending"
	ProductOrderFulfillmentShipped   = "shipped"
	ProductOrderFulfillmentCancelled = "cancelled"
)

var (
	ErrProductOrderInsufficientBalance = errors.New("insufficient wallet balance")
	ErrProductOrderInvalidState        = errors.New("invalid product order state")
)

type ProductOrder struct {
	Id                int    `json:"id"`
	UserId            int    `json:"user_id" gorm:"index;not null"`
	ProductId         int    `json:"product_id" gorm:"index;not null"`
	ProductName       string `json:"product_name" gorm:"type:varchar(128);not null"`
	PriceCents        int    `json:"price_cents" gorm:"type:int;not null"`
	PaymentMethod     string `json:"payment_method" gorm:"type:varchar(32);not null"`
	PaymentStatus     string `json:"payment_status" gorm:"type:varchar(32);not null"`
	FulfillmentStatus string `json:"fulfillment_status" gorm:"type:varchar(32);not null"`
	PaidQuota         int    `json:"paid_quota" gorm:"type:int;not null"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt         int64  `json:"updated_at" gorm:"bigint"`
}

func (order *ProductOrder) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	order.CreatedAt = now
	order.UpdatedAt = now
	return nil
}

func (order *ProductOrder) BeforeUpdate(tx *gorm.DB) error {
	order.UpdatedAt = common.GetTimestamp()
	return nil
}

func CreateQRCodeProductOrder(userId, productId int) (*ProductOrder, error) {
	if userId <= 0 || productId <= 0 {
		return nil, errors.New("invalid product order")
	}
	order := &ProductOrder{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		product := &Product{}
		if err := lockForUpdate(tx).Where("id = ? AND enabled = ?", productId, true).First(product).Error; err != nil {
			return err
		}
		*order = ProductOrder{
			UserId:            userId,
			ProductId:         product.Id,
			ProductName:       product.Name,
			PriceCents:        product.PriceCents,
			PaymentMethod:     ProductOrderPaymentQRCode,
			PaymentStatus:     ProductOrderPaymentPending,
			FulfillmentStatus: ProductOrderFulfillmentPending,
		}
		return tx.Create(order).Error
	})
	if err != nil {
		return nil, err
	}
	return order, nil
}

func productPriceCentsToQuota(priceCents int) (int, error) {
	if priceCents <= 0 || common.QuotaPerUnit <= 0 || operation_setting.USDExchangeRate <= 0 {
		return 0, errors.New("invalid product payment configuration")
	}
	quotaDecimal := decimal.NewFromInt(int64(priceCents)).
		Div(decimal.NewFromInt(100)).
		Div(decimal.NewFromFloat(operation_setting.USDExchangeRate)).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit))
	quota, clamp := common.QuotaFromDecimalChecked(quotaDecimal)
	if clamp != nil || quota <= 0 {
		return 0, errors.New("invalid product payment configuration")
	}
	return quota, nil
}

func CreateWalletProductOrder(userId, productId int) (*ProductOrder, error) {
	if userId <= 0 || productId <= 0 {
		return nil, errors.New("invalid product order")
	}
	order := &ProductOrder{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		product := &Product{}
		if err := lockForUpdate(tx).Where("id = ? AND enabled = ?", productId, true).First(product).Error; err != nil {
			return err
		}
		quota, err := productPriceCentsToQuota(product.PriceCents)
		if err != nil {
			return err
		}
		result := tx.Model(&User{}).
			Where("id = ? AND quota >= ?", userId, quota).
			Update("quota", gorm.Expr("quota - ?", quota))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrProductOrderInsufficientBalance
		}
		*order = ProductOrder{
			UserId:            userId,
			ProductId:         product.Id,
			ProductName:       product.Name,
			PriceCents:        product.PriceCents,
			PaymentMethod:     ProductOrderPaymentWallet,
			PaymentStatus:     ProductOrderPaymentPaid,
			FulfillmentStatus: ProductOrderFulfillmentPending,
			PaidQuota:         quota,
		}
		return tx.Create(order).Error
	})
	if err != nil {
		return nil, err
	}
	if common.RedisEnabled {
		if err := cacheIncrUserQuota(userId, -int64(order.PaidQuota)); err != nil {
			common.SysError("failed to update product order wallet cache: " + err.Error())
		}
	}
	return order, nil
}

func ConfirmQRCodeProductOrder(orderId int) error {
	if orderId <= 0 {
		return ErrProductOrderInvalidState
	}
	result := DB.Model(&ProductOrder{}).
		Where("id = ? AND payment_method = ? AND payment_status = ? AND fulfillment_status = ?", orderId, ProductOrderPaymentQRCode, ProductOrderPaymentPending, ProductOrderFulfillmentPending).
		Updates(map[string]interface{}{
			"payment_status": ProductOrderPaymentPaid,
			"updated_at":     common.GetTimestamp(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrProductOrderInvalidState
	}
	return nil
}

func ShipProductOrder(orderId int) error {
	if orderId <= 0 {
		return ErrProductOrderInvalidState
	}
	result := DB.Model(&ProductOrder{}).
		Where("id = ? AND payment_status = ? AND fulfillment_status = ?", orderId, ProductOrderPaymentPaid, ProductOrderFulfillmentPending).
		Updates(map[string]interface{}{
			"fulfillment_status": ProductOrderFulfillmentShipped,
			"updated_at":         common.GetTimestamp(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrProductOrderInvalidState
	}
	return nil
}

func CancelProductOrder(orderId int) error {
	if orderId <= 0 {
		return ErrProductOrderInvalidState
	}
	var refundedOrder ProductOrder
	err := DB.Transaction(func(tx *gorm.DB) error {
		order := &ProductOrder{}
		if err := lockForUpdate(tx).Where("id = ?", orderId).First(order).Error; err != nil {
			return err
		}
		if order.FulfillmentStatus != ProductOrderFulfillmentPending {
			return ErrProductOrderInvalidState
		}
		if order.PaymentMethod == ProductOrderPaymentWallet && order.PaymentStatus == ProductOrderPaymentPaid && order.PaidQuota > 0 {
			if err := tx.Model(&User{}).Where("id = ?", order.UserId).Update("quota", gorm.Expr("quota + ?", order.PaidQuota)).Error; err != nil {
				return err
			}
		}
		result := tx.Model(order).Updates(map[string]interface{}{
			"fulfillment_status": ProductOrderFulfillmentCancelled,
			"updated_at":         common.GetTimestamp(),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrProductOrderInvalidState
		}
		refundedOrder = *order
		return nil
	})
	if err != nil {
		return err
	}
	if refundedOrder.PaymentMethod == ProductOrderPaymentWallet && refundedOrder.PaymentStatus == ProductOrderPaymentPaid && refundedOrder.PaidQuota > 0 && common.RedisEnabled {
		if err := cacheIncrUserQuota(refundedOrder.UserId, int64(refundedOrder.PaidQuota)); err != nil {
			common.SysError("failed to update product order refund cache: " + err.Error())
		}
	}
	return nil
}

func ListProductOrdersByUser(userId int) ([]ProductOrder, error) {
	orders := make([]ProductOrder, 0)
	err := DB.Where("user_id = ?", userId).Order("id desc").Find(&orders).Error
	return orders, err
}

func ListProductOrders() ([]ProductOrder, error) {
	orders := make([]ProductOrder, 0)
	err := DB.Order("id desc").Find(&orders).Error
	return orders, err
}
