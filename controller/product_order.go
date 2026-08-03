package controller

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/gin-gonic/gin"
)

type CreateProductOrderRequest struct {
	ProductId     int    `json:"product_id"`
	PaymentMethod string `json:"payment_method"`
}

type CreateProductOrderResponse struct {
	model.ProductOrder
	PaymentQRCodeURL    string `json:"payment_qr_code_url,omitempty"`
	PaymentInstructions string `json:"payment_instructions,omitempty"`
}

func CreateProductOrder(c *gin.Context) {
	var req CreateProductOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ProductId <= 0 {
		common.ApiErrorMsg(c, "invalid product order")
		return
	}
	userId := c.GetInt("id")
	var (
		order *model.ProductOrder
		err   error
	)
	switch req.PaymentMethod {
	case model.ProductOrderPaymentWallet:
		order, err = model.CreateWalletProductOrder(userId, req.ProductId)
	case model.ProductOrderPaymentQRCode:
		order, err = model.CreateQRCodeProductOrder(userId, req.ProductId)
	default:
		common.ApiErrorMsg(c, "invalid product payment method")
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	response := CreateProductOrderResponse{ProductOrder: *order}
	if order.PaymentMethod == model.ProductOrderPaymentQRCode {
		var product model.Product
		if err := model.DB.First(&product, order.ProductId).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		response.PaymentQRCodeURL = product.PaymentQRCodeURL
		response.PaymentInstructions = product.PaymentInstructions
	}
	common.ApiSuccess(c, response)
}

func ListSelfProductOrders(c *gin.Context) {
	orders, err := model.ListProductOrdersByUser(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, orders)
}

func AdminListProductOrders(c *gin.Context) {
	orders, err := model.ListProductOrders()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, orders)
}

func parseProductOrderId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid product order id")
		return 0, false
	}
	return id, true
}

func AdminConfirmProductOrder(c *gin.Context) {
	id, ok := parseProductOrderId(c)
	if !ok {
		return
	}
	if err := model.ConfirmQRCodeProductOrder(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminShipProductOrder(c *gin.Context) {
	id, ok := parseProductOrderId(c)
	if !ok {
		return
	}
	if err := model.ShipProductOrder(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminCancelProductOrder(c *gin.Context) {
	id, ok := parseProductOrderId(c)
	if !ok {
		return
	}
	if err := model.CancelProductOrder(id); err != nil {
		if errors.Is(err, model.ErrProductOrderInvalidState) {
			common.ApiErrorMsg(c, "product order cannot be cancelled")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
