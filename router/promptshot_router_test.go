package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayRouterRegistersPromptShotCompatibilityRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)

	routes := make(map[string]struct{})
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}

	for _, path := range []string{
		"/v1/auth/validate",
		"/v1/reverse-prompt",
		"/v1/generate-image",
		"/v1/clean-image",
	} {
		_, found := routes[http.MethodPost+" "+path]
		require.Truef(t, found, "missing PromptShot compatibility route %s", path)
	}

	_, standardImageRouteExists := routes[http.MethodPost+" /v1/images/edits"]
	assert.True(t, standardImageRouteExists, "PromptShot routes must not replace the standard OpenAI image endpoint")
}

func TestPromptShotAuthValidateRejectsMissingBodyTokenWithoutEchoingCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/validate", bytes.NewBufferString(`{"token":""}`))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.NotContains(t, strings.ToLower(recorder.Body.String()), "token")
}
