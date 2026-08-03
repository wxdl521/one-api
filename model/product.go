package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/the-one/common"
	"gorm.io/gorm"
)

const (
	ProductTypeManual       = "manual"
	ProductTypeSubscription = "subscription"
)

type Product struct {
	Id                  int    `json:"id"`
	Name                string `json:"name" gorm:"type:varchar(128);not null"`
	Summary             string `json:"summary" gorm:"type:varchar(255);default:''"`
	Description         string `json:"description" gorm:"type:text"`
	ImageURL            string `json:"image_url" gorm:"type:varchar(2048);default:''"`
	PriceCents          int    `json:"price_cents" gorm:"type:int;not null;default:0"`
	PaymentQRCodeURL    string `json:"payment_qr_code_url" gorm:"type:varchar(2048);default:''"`
	PaymentInstructions string `json:"payment_instructions" gorm:"type:text"`
	ProductType         string `json:"product_type" gorm:"type:varchar(32);not null"`
	SubscriptionPlanId  *int   `json:"subscription_plan_id,omitempty" gorm:"index"`
	Enabled             bool   `json:"enabled"`
	SortOrder           int    `json:"sort_order" gorm:"type:int;default:0"`
	CreatedAt           int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt           int64  `json:"updated_at" gorm:"bigint"`
}

func (p *Product) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *Product) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

func IsProductTypeValid(productType string) bool {
	return productType == ProductTypeManual || productType == ProductTypeSubscription
}

func (p *Product) Normalize() error {
	p.Name = strings.TrimSpace(p.Name)
	p.Summary = strings.TrimSpace(p.Summary)
	p.Description = strings.TrimSpace(p.Description)
	p.ImageURL = strings.TrimSpace(p.ImageURL)
	p.PaymentQRCodeURL = strings.TrimSpace(p.PaymentQRCodeURL)
	p.PaymentInstructions = strings.TrimSpace(p.PaymentInstructions)
	p.ProductType = strings.TrimSpace(p.ProductType)
	if p.Name == "" {
		return errors.New("product name is required")
	}
	if !IsProductTypeValid(p.ProductType) {
		return errors.New("invalid product type")
	}
	if p.PriceCents <= 0 {
		return errors.New("product price must be greater than zero")
	}
	if p.ProductType == ProductTypeSubscription {
		if p.SubscriptionPlanId == nil || *p.SubscriptionPlanId <= 0 {
			return errors.New("subscription product requires a subscription plan")
		}
		return nil
	}
	p.SubscriptionPlanId = nil
	return nil
}

func ListPublishedProducts() ([]Product, error) {
	products := make([]Product, 0)
	err := DB.Where("enabled = ?", true).Order("sort_order desc, id desc").Find(&products).Error
	return products, err
}

func GetPublishedProductById(id int) (*Product, error) {
	if id <= 0 {
		return nil, errors.New("invalid product id")
	}
	product := &Product{}
	if err := DB.Where("id = ? AND enabled = ?", id, true).First(product).Error; err != nil {
		return nil, err
	}
	return product, nil
}
