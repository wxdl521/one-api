package router

import (
	"github.com/QuantumNous/the-one/controller"
	"github.com/QuantumNous/the-one/middleware"
	"github.com/gin-gonic/gin"
)

func SetMCPRouter(router *gin.Engine) {
	router.Any("/mcp",
		middleware.RouteTag("mcp"),
		middleware.CriticalRateLimit(),
		middleware.MCPBearerAuth(),
		middleware.TokenAuth(),
		controller.ServeMCP,
	)
}
