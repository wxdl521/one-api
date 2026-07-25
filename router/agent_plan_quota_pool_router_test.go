package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentPlanQuotaPoolRoutesRequireAdminAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, route := range []string{
		http.MethodGet + " /api/subscription/admin/agent-plan-pools",
		http.MethodPost + " /api/subscription/admin/agent-plan-pools",
		http.MethodPut + " /api/subscription/admin/agent-plan-pools/:id",
		http.MethodDelete + " /api/subscription/admin/agent-plan-pools/:id",
		http.MethodPost + " /api/subscription/admin/agent-plan-pools/:id/sync",
		http.MethodGet + " /api/subscription/admin/agent-plan-pools/eligible-source-channels",
	} {
		_, ok := routes[route]
		require.Truef(t, ok, "missing route %s", route)
	}

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/subscription/admin/agent-plan-pools", nil))
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
