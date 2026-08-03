package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacyProduct struct {
	Id                 int
	Name               string
	Summary            string
	Description        string
	ImageURL           string
	ProductType        string
	SubscriptionPlanId *int
	Enabled            bool
	SortOrder          int
	CreatedAt          int64
	UpdatedAt          int64
}

func (legacyProduct) TableName() string {
	return "products"
}

func TestListPublishedProductsExcludesUnpublished(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Product{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Product{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&Product{}).Error)
	})

	require.NoError(t, DB.Create(&Product{Name: "Published CDK", ProductType: ProductTypeManual, Enabled: true}).Error)
	require.NoError(t, DB.Create(&Product{Name: "Draft CDK", ProductType: ProductTypeManual, Enabled: false}).Error)

	products, err := ListPublishedProducts()

	require.NoError(t, err)
	require.Len(t, products, 1)
	assert.Equal(t, "Published CDK", products[0].Name)
}

func TestProductNormalizeRejectsNonPositivePrice(t *testing.T) {
	product := Product{
		Name:        "Manual CDK",
		ProductType: ProductTypeManual,
		PriceCents:  0,
	}

	require.EqualError(t, product.Normalize(), "product price must be greater than zero")
}

func TestProductMigrationAddsPriceToLegacySQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:legacy_product?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyProduct{}))
	require.NoError(t, db.Create(&legacyProduct{Id: 1, Name: "Legacy", ProductType: ProductTypeManual}).Error)

	require.NoError(t, db.AutoMigrate(&Product{}))

	var product Product
	require.NoError(t, db.First(&product, 1).Error)
	assert.Zero(t, product.PriceCents)
}
