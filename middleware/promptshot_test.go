package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptShotBodyTokenAuthMovesCredentialToAuthorizationAndPreservesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/validate", PromptShotBodyTokenAuth(), func(c *gin.Context) {
		var body promptShotBodyTokenRequest
		require.NoError(t, common.UnmarshalBodyReusable(c, &body))
		assert.Equal(t, "Bearer example-token", c.GetHeader("Authorization"))
		assert.Equal(t, "example-token", body.Token)
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBufferString(`{"token":"example-token"}`))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "example-token")
}
