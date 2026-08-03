package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		http.MethodGet + " /api/miniapp/v1/me/overview",
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

func TestMiniAppFeatureGateRunsBeforeBodyLimitsAndAuthentication(t *testing.T) {
	previousEnabled, previousTextEnabled := common.GetMiniProgramFeatureFlags()
	common.OptionMapRWMutex.Lock()
	common.MiniProgramEnabled = false
	common.MiniProgramTextTestEnabled = false
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.MiniProgramEnabled = previousEnabled
		common.MiniProgramTextTestEnabled = previousTextEnabled
		common.OptionMapRWMutex.Unlock()
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetMiniAppRouter(engine)
	for _, target := range []string{
		"/api/miniapp/v1/auth/wechat",
		"/api/miniapp/bindings/confirm",
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(strings.Repeat("x", 16<<10)))
		engine.ServeHTTP(response, request)

		require.Equal(t, http.StatusNotFound, response.Code)
		assert.Contains(t, response.Body.String(), "MINIAPP_DISABLED")
	}
}
