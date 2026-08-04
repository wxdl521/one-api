package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestPromptShotResponseLimiterRejectsChunkedBodyPastConfiguredLimit(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("1234"))
		writer.(http.Flusher).Flush()
		_, _ = writer.Write([]byte("5"))
		writer.(http.Flusher).Flush()
	}))
	defer upstream.Close()

	response, err := upstream.Client().Get(upstream.URL)
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, int64(-1), response.ContentLength)
	require.NoError(t, limitPromptShotHTTPResponse(response, 4))

	contents, err := io.ReadAll(response.Body)
	assert.Equal(t, []byte("1234"), contents)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPromptShotUpstreamResponseTooLarge))
}
