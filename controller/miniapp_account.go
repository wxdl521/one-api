package controller

import (
	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
)

func MiniAppAccountOverview(c *gin.Context) {
	overview, err := service.GetMiniAppAccountOverview(c.GetInt("id"))
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, overview)
}
