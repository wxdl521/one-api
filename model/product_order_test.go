package model

import (
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useProductOrderTestExchangeRate(t *testing.T) {
	t.Helper()
	previousQuotaPerUnit := common.QuotaPerUnit
	previousExchangeRate := operation_setting.USDExchangeRate
	common.QuotaPerUnit = 100
	operation_setting.USDExchangeRate = 1
	t.Cleanup(func() {
		common.QuotaPerUnit = previousQuotaPerUnit
		operation_setting.USDExchangeRate = previousExchangeRate
	})
}

func TestCreateQRCodeProductOrderSnapshotsProduct(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Product{}, &ProductOrder{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&ProductOrder{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Product{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&ProductOrder{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Product{}).Error)
	})

	product := Product{
		Name:        "Agent plan",
		ProductType: ProductTypeManual,
		PriceCents:  9900,
		Enabled:     true,
	}
	require.NoError(t, DB.Create(&product).Error)

	order, err := CreateQRCodeProductOrder(42, product.Id)

	require.NoError(t, err)
	assert.Equal(t, ProductOrderPaymentPending, order.PaymentStatus)
	assert.Equal(t, "Agent plan", order.ProductName)
	assert.Equal(t, 9900, order.PriceCents)
}

func TestCreateWalletProductOrderDebitsBalance(t *testing.T) {
	useProductOrderTestExchangeRate(t)

	require.NoError(t, DB.AutoMigrate(&User{}, &Product{}, &ProductOrder{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&ProductOrder{}).Error)
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Product{}).Error)
	require.NoError(t, DB.Where("username = ?", "product-wallet-buyer").Delete(&User{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&ProductOrder{}).Error)
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Product{}).Error)
		require.NoError(t, DB.Where("username = ?", "product-wallet-buyer").Delete(&User{}).Error)
	})

	user := User{Username: "product-wallet-buyer", AffCode: "product-wallet-buyer", Status: common.UserStatusEnabled, Quota: 9900}
	product := Product{Name: "Wallet CDK", ProductType: ProductTypeManual, PriceCents: 9900, Enabled: true}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&product).Error)

	order, err := CreateWalletProductOrder(user.Id, product.Id)

	require.NoError(t, err)
	assert.Equal(t, ProductOrderPaymentPaid, order.PaymentStatus)
	assert.Equal(t, ProductOrderFulfillmentPending, order.FulfillmentStatus)
	assert.Equal(t, 9900, order.PaidQuota)
	var quota int
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Select("quota").Find(&quota).Error)
	assert.Zero(t, quota)
}

func TestCreateWalletProductOrderDoesNotCreateOrderWhenBalanceIsInsufficient(t *testing.T) {
	useProductOrderTestExchangeRate(t)
	user := User{Username: "product-wallet-insufficient", AffCode: "product-wallet-insufficient", Status: common.UserStatusEnabled, Quota: 1}
	product := Product{Name: "Expensive CDK", ProductType: ProductTypeManual, PriceCents: 9900, Enabled: true}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&product).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Where("username = ?", "product-wallet-insufficient").Delete(&User{}).Error)
		require.NoError(t, DB.Where("id = ?", product.Id).Delete(&Product{}).Error)
	})

	_, err := CreateWalletProductOrder(user.Id, product.Id)

	require.ErrorIs(t, err, ErrProductOrderInsufficientBalance)
	var count int64
	require.NoError(t, DB.Model(&ProductOrder{}).Where("user_id = ?", user.Id).Count(&count).Error)
	assert.Zero(t, count)
}

func TestConfirmAndShipQRCodeProductOrder(t *testing.T) {
	product := Product{Name: "QR CDK", ProductType: ProductTypeManual, PriceCents: 9900, Enabled: true}
	require.NoError(t, DB.Create(&product).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Where("id = ?", product.Id).Delete(&Product{}).Error)
	})

	order, err := CreateQRCodeProductOrder(42, product.Id)
	require.NoError(t, err)
	require.NoError(t, ConfirmQRCodeProductOrder(order.Id))
	require.NoError(t, ShipProductOrder(order.Id))

	var stored ProductOrder
	require.NoError(t, DB.First(&stored, order.Id).Error)
	assert.Equal(t, ProductOrderPaymentPaid, stored.PaymentStatus)
	assert.Equal(t, ProductOrderFulfillmentShipped, stored.FulfillmentStatus)
}

func TestCancelPaidProductOrderRefundsWalletOnlyOnce(t *testing.T) {
	useProductOrderTestExchangeRate(t)
	user := User{Username: "product-wallet-refund", AffCode: "product-wallet-refund", Status: common.UserStatusEnabled, Quota: 9900}
	product := Product{Name: "Refund CDK", ProductType: ProductTypeManual, PriceCents: 9900, Enabled: true}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&product).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Where("username = ?", "product-wallet-refund").Delete(&User{}).Error)
		require.NoError(t, DB.Where("id = ?", product.Id).Delete(&Product{}).Error)
	})

	order, err := CreateWalletProductOrder(user.Id, product.Id)
	require.NoError(t, err)
	require.NoError(t, CancelProductOrder(order.Id))
	require.Error(t, CancelProductOrder(order.Id))

	var quota int
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Select("quota").Find(&quota).Error)
	assert.Equal(t, 9900, quota)
}
