package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeChatVideoTaskFailureDoesNotExposeUpstreamDetails(t *testing.T) {
	reason := sanitizeChatVideoTaskFailure("upstream https://provider.example/v1 failed with key sk-secret")

	assert.Equal(t, "Video generation failed", reason)
}

func TestPublicChatVideoContentErrorDoesNotExposeUpstreamDetails(t *testing.T) {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Set(chatVideoPublicContentContextKey, true)

	videoProxyError(context, http.StatusForbidden, "server_error", "request blocked: https://provider.example/?key=sk-secret")

	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.Equal(t, "Video content is unavailable", payload.Error.Message)
}

func TestPublicChatVideoContentDisablesSharedCaching(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set(chatVideoPublicContentContextKey, true)

	assert.Equal(t, "private, no-store", videoProxyCacheControl(context))
}
