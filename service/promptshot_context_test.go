package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestPromptShotRequestContextMarksErrorsForRedaction(t *testing.T) {
	assert.False(t, IsPromptShotRequestContext(context.Background()))
	assert.True(t, IsPromptShotRequestContext(WithPromptShotRequestContext(context.Background())))
}

func TestIsPromptShotCompatibleRequestRecognizesGinAndRequestMarkers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/generate-image", nil)

	assert.False(t, IsPromptShotCompatibleRequest(context))

	context.Set(PromptShotCompatContextKey, true)
	assert.True(t, IsPromptShotCompatibleRequest(context))

	context.Set(PromptShotCompatContextKey, false)
	context.Request = context.Request.WithContext(WithPromptShotRequestContext(context.Request.Context()))
	assert.True(t, IsPromptShotCompatibleRequest(context))
}
