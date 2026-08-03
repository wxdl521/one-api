package controller

import (
	"errors"
	"strconv"
	"unicode/utf8"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AdminUpdateProductStatusRequest struct {
	Enabled *bool `json:"enabled"`
}

func validateProduct(product *model.Product) error {
	if err := product.Normalize(); err != nil {
		return err
	}
	if utf8.RuneCountInString(product.Name) > 128 {
		return errors.New("product name is too long")
	}
	if utf8.RuneCountInString(product.Summary) > 255 {
		return errors.New("product summary is too long")
	}
	if utf8.RuneCountInString(product.ImageURL) > 2048 {
		return errors.New("product image URL is too long")
	}
	if product.ProductType != model.ProductTypeSubscription {
		return nil
	}
	var plan model.SubscriptionPlan
	return model.DB.First(&plan, "id = ?", *product.SubscriptionPlanId).Error
}

func ListProducts(c *gin.Context) {
	products, err := model.ListPublishedProducts()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, products)
}

func GetProduct(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	product, err := model.GetPublishedProductById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "product not found")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, product)
}

func AdminListProducts(c *gin.Context) {
	products := make([]model.Product, 0)
	if err := model.DB.Order("sort_order desc, id desc").Find(&products).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, products)
}

func AdminCreateProduct(c *gin.Context) {
	var product model.Product
	if err := c.ShouldBindJSON(&product); err != nil {
		common.ApiErrorMsg(c, "invalid product payload")
		return
	}
	product.Id = 0
	if err := validateProduct(&product); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DB.Create(&product).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, product)
}

func AdminUpdateProduct(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "invalid product id")
		return
	}
	var product model.Product
	if err := c.ShouldBindJSON(&product); err != nil {
		common.ApiErrorMsg(c, "invalid product payload")
		return
	}
	product.Id = id
	if err := validateProduct(&product); err != nil {
		common.ApiError(c, err)
		return
	}
	updates := map[string]interface{}{
		"name":                 product.Name,
		"summary":              product.Summary,
		"description":          product.Description,
		"image_url":            product.ImageURL,
		"product_type":         product.ProductType,
		"subscription_plan_id": product.SubscriptionPlanId,
		"enabled":              product.Enabled,
		"sort_order":           product.SortOrder,
		"updated_at":           common.GetTimestamp(),
	}
	result := model.DB.Model(&model.Product{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		common.ApiError(c, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		common.ApiErrorMsg(c, "product not found")
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminUpdateProductStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "invalid product id")
		return
	}
	var req AdminUpdateProductStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		common.ApiErrorMsg(c, "invalid product status")
		return
	}
	result := model.DB.Model(&model.Product{}).Where("id = ?", id).Updates(map[string]interface{}{
		"enabled":    *req.Enabled,
		"updated_at": common.GetTimestamp(),
	})
	if result.Error != nil {
		common.ApiError(c, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		common.ApiErrorMsg(c, "product not found")
		return
	}
	common.ApiSuccess(c, nil)
}
