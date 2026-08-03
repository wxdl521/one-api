package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMiniAppRouterKeepsTheBFFAndBrowserConfirmationRoutesSeparate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetMiniAppRouter(engine)

	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}

	for _, route := range []string{
		http.MethodPost + " /api/miniapp/v1/auth/wechat",
		http.MethodPost + " /api/miniapp/v1/auth/register",
		http.MethodPost + " /api/miniapp/v1/bindings",
		http.MethodGet + " /api/miniapp/v1/bindings/:id",
		http.MethodPost + " /api/miniapp/v1/auth/renew",
		http.MethodPost + " /api/miniapp/v1/auth/logout",
		http.MethodPost + " /api/miniapp/bindings/confirm",
	} {
		_, found := routes[route]
		assert.True(t, found, route)
	}

	for _, route := range []string{
		http.MethodGet + " /api/miniapp/v1/user/self",
		http.MethodGet + " /api/miniapp/v1/token/",
		http.MethodPost + " /api/miniapp/v1/v1/chat/completions",
	} {
		_, found := routes[route]
		assert.False(t, found, route)
	}
}
