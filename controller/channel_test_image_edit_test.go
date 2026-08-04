package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareImageEditTestRequestCreatesOpenAIMultipartPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)

	require.NoError(t, prepareImageEditTestRequest(context, "gpt-image-2"))
	require.NoError(t, context.Request.ParseMultipartForm(1<<20))

	assert.Equal(t, "gpt-image-2", context.Request.FormValue("model"))
	assert.NotEmpty(t, context.Request.FormValue("prompt"))
	files := context.Request.MultipartForm.File["image"]
	require.Len(t, files, 1)
	assert.Equal(t, "channel-test.png", files[0].Filename)
	assert.Greater(t, files[0].Size, int64(32))
}
