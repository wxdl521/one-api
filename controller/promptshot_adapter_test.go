package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/constant"
	"github.com/QuantumNous/the-one/service"
	"github.com/QuantumNous/the-one/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPromptShotResponseAdapterNormalizesImageResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/promptshot", func(c *gin.Context) {
		c.Set(promptShotResponseKindContextKey, "image")
	}, PromptShotResponseAdapter(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": []gin.H{{"b64_json": "YWJj"}}, "usage": gin.H{"total_tokens": 3}})
	})

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/promptshot", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "YWJj", gjson.Get(recorder.Body.String(), "image_b64").String())
	assert.Equal(t, "image/png", gjson.Get(recorder.Body.String(), "mime").String())
	assert.True(t, gjson.Get(recorder.Body.String(), "usage").Exists())
}

func TestPromptShotResponseAdapterHidesUpstreamFailureDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/promptshot", func(c *gin.Context) {
		c.Set(promptShotResponseKindContextKey, "image")
	}, PromptShotResponseAdapter(), func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": "provider-secret-channel-17"}})
	})

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/promptshot", nil))

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	assert.NotContains(t, strings.ToLower(recorder.Body.String()), "provider")
	assert.NotContains(t, recorder.Body.String(), "channel-17")
	assert.NotEmpty(t, gjson.Get(recorder.Body.String(), "error.message").String())
}

func TestPromptShotRelayErrorReportsUnavailableEditCapabilityClearly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	promptShotRelayError(context, system_setting.PromptShotOperationEdit, service.ErrPromptShotNoConfiguredModel)

	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	assert.Equal(t, "当前 Token 无可用参考图编辑能力", gjson.Get(recorder.Body.String(), "error.message").String())
	assert.NotContains(t, recorder.Body.String(), "云端服务不可用")
}

func TestPromptShotAuthValidateReturnsOnlySafeLabels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/auth/validate", nil)

	PromptShotAuthValidate(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	for _, field := range []string{"account_label", "plan_label", "quota_label", "price_note", "privacy_note", "model_note"} {
		assert.NotEmpty(t, gjson.Get(recorder.Body.String(), field).String())
	}
	assert.NotContains(t, strings.ToLower(recorder.Body.String()), "sk-")
}

func TestPromptShotPrepareCleanRejectsInvalidQualityBeforeCapabilityLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/promptshot", PromptShotPrepareClean(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/promptshot", strings.NewReader(`{"image_b64":"YWJj","mime":"image/png","quality":"high"}`))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, "请求参数无效", gjson.Get(recorder.Body.String(), "error.message").String())
}

func TestSelectPromptShotModelRejectsClientSpecifiedChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(context, constant.ContextKeyTokenSpecificChannelId, "17")

	_, err := selectPromptShotModel(context, system_setting.PromptShotOperationGenerate)

	require.ErrorIs(t, err, service.ErrPromptShotGroupUnavailable)
}
