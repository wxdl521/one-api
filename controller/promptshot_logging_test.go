package controller

import (
	"errors"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptShotRelayLogMessageDoesNotIncludeUpstreamDetails(t *testing.T) {
	context, _ := gin.CreateTestContext(nil)
	context.Set(promptShotContextKey, true)

	message := promptShotRelayLogMessage(context, errors.New("provider key=secret image=data:image/png;base64,YWJj"))

	require.NotEmpty(t, message)
	assert.NotContains(t, strings.ToLower(message), "provider")
	assert.NotContains(t, strings.ToLower(message), "secret")
	assert.NotContains(t, strings.ToLower(message), "base64")
}
