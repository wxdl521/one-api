package controller

import (
	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/middleware"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
)

func MiniAppListPlans(c *gin.Context) {
	plans, err := service.ListMiniAppPlans()
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, plans)
}

func MiniAppListProducts(c *gin.Context) {
	products, err := service.ListMiniAppProducts()
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, products)
}

func MiniAppListOrders(c *gin.Context) {
	orders, err := service.ListMiniAppOrders(c.GetInt("id"))
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, orders)
}

func MiniAppStartCheckout(c *gin.Context) {
	fields, ok := decodeMiniAppRequest(c, "target_type", "target_id")
	if !ok {
		return
	}
	targetType, ok := miniAppRequiredString(c, fields, "target_type", 16)
	if !ok {
		return
	}
	rawTargetID, ok := fields["target_id"]
	if !ok {
		writeMiniAppInvalidRequest(c)
		return
	}
	var targetID int
	if err := common.Unmarshal(rawTargetID, &targetID); err != nil || targetID <= 0 {
		writeMiniAppInvalidRequest(c)
		return
	}
	checkout, err := service.StartMiniAppCheckout(c.GetInt("id"), c.GetString("session_id"), targetType, targetID)
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, checkout)
}

func ConfirmMiniAppBrowserCheckout(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		writeMiniAppBrowserSessionRequired(c)
		return
	}
	fields, ok := decodeMiniAppRequest(c, "checkout_ticket")
	if !ok {
		return
	}
	checkoutTicket, ok := miniAppRequiredString(c, fields, "checkout_ticket", miniAppMaxTicketLength)
	if !ok {
		return
	}
	confirmation, err := service.ConfirmMiniAppBrowserCheckout(checkoutTicket, identity)
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, confirmation)
}
