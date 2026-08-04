package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/constant"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/service"
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

func TestAzureChannelImageEditTestSendsMultipartRequest(t *testing.T) {
	service.InitHttpClient()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	require.NoError(t, db.Create(&model.User{
		Id:       1,
		Username: "channel-test-user",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Setting:  `{"accept_unset_model_ratio_model":true}`,
	}).Error)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/openai/deployments/gpt-image-1/images/edits", r.URL.Path)
		assert.Equal(t, "2025-04-01-preview", r.URL.Query().Get("api-version"))
		assert.Equal(t, "channel-test-key", r.Header.Get("api-key"))
		require.NoError(t, r.ParseMultipartForm(1<<20))
		assert.Equal(t, "gpt-image-1", r.FormValue("model"))
		assert.NotEmpty(t, r.FormValue("prompt"))
		files := r.MultipartForm.File["image"]
		require.Len(t, files, 1)

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"data":[{"b64_json":"ZmFrZQ=="}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
		require.NoError(t, err)
	}))
	t.Cleanup(upstream.Close)

	result := testChannel(t.Context(), &model.Channel{
		Type:    constant.ChannelTypeAzure,
		Key:     "channel-test-key",
		BaseURL: common.GetPointer(upstream.URL),
		Other:   "2025-04-01-preview",
	}, 1, "gpt-image-1", string(constant.EndpointTypeImageEdit), false)

	require.NoError(t, result.localErr)
	require.Nil(t, result.theOneError)
}
