package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/constant"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/service"
	"github.com/QuantumNous/the-one/setting"
	"github.com/QuantumNous/the-one/setting/config"
	"github.com/QuantumNous/the-one/setting/operation_setting"
	"github.com/QuantumNous/the-one/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPromptShotPipelineRateLimitsThenDistributesAndRelays(t *testing.T) {
	gin.SetMode(gin.TestMode)

	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousRedisEnabled := common.RedisEnabled
	previousRateLimitEnabled := setting.ModelRequestRateLimitEnabled
	previousRateLimitDuration := setting.ModelRequestRateLimitDurationMinutes
	previousRateLimitCount := setting.ModelRequestRateLimitCount
	previousRateLimitSuccessCount := setting.ModelRequestRateLimitSuccessCount
	previousFreeModelPreConsume := operation_setting.GetQuotaSetting().EnableFreeModelPreConsume

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, model.InitLogDB())
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	setting.ModelRequestRateLimitEnabled = true
	setting.ModelRequestRateLimitDurationMinutes = 1
	setting.ModelRequestRateLimitCount = 0
	setting.ModelRequestRateLimitSuccessCount = 1
	operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = false
	service.InitHttpClient()
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousDatabaseType, previousLogDatabaseType)
		_ = model.InitLogDB() // restores the SQL-column quoting for subsequent tests
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.RedisEnabled = previousRedisEnabled
		setting.ModelRequestRateLimitEnabled = previousRateLimitEnabled
		setting.ModelRequestRateLimitDurationMinutes = previousRateLimitDuration
		setting.ModelRequestRateLimitCount = previousRateLimitCount
		setting.ModelRequestRateLimitSuccessCount = previousRateLimitSuccessCount
		operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = previousFreeModelPreConsume
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.Ability{}, &model.Log{}))

	const group = "promptshot-integration-group"
	const modelName = "promptshot-test-model"
	const tokenKey = "promptshotintegrationtoken"
	previousGroupRatios := ratio_setting.GroupRatio2JSONString()
	groupRatios := ratio_setting.GetGroupRatioCopy()
	groupRatios[group] = 0
	groupRatiosJSON, err := common.Marshal(groupRatios)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(string(groupRatiosJSON)))
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroupRatios)) })
	user := &model.User{Id: 74123, Username: "promptshot-integration", Password: "unused", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: group, Quota: 1_000_000, AuthVersion: 1, Setting: `{"accept_unset_model_ratio_model":true}`}
	require.NoError(t, db.Create(user).Error)
	token := &model.Token{UserId: user.Id, Key: tokenKey, Status: common.TokenStatusEnabled, Name: "pipeline", ExpiredTime: -1, UnlimitedQuota: true}
	require.NoError(t, db.Create(token).Error)

	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		upstreamCalls.Add(1)
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "/v1/images/generations", request.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"created":1,"data":[{"b64_json":"YWJj"}],"usage":{"total_tokens":1}}`))
	}))
	t.Cleanup(upstream.Close)

	autoBan := 0
	channel := &model.Channel{Type: constant.ChannelTypeOpenAI, Key: "upstream-key", Status: common.ChannelStatusEnabled, Name: "promptshot-verified", BaseURL: &upstream.URL, Models: modelName, Group: group, AutoBan: &autoBan}
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, db.Create(&model.Ability{Group: group, Model: modelName, ChannelId: channel.Id, Enabled: true}).Error)

	promptShotConfig, ok := config.GlobalConfig.Get("promptshot").(config.CustomConfig)
	require.True(t, ok)
	previousPromptShotConfig, err := promptShotConfig.ConfigToMap()
	require.NoError(t, err)
	require.NoError(t, promptShotConfig.UpdateConfigFromMap(map[string]string{
		"generate_models": fmt.Sprintf(`["%s"]`, modelName),
		"capabilities":    fmt.Sprintf(`[{"channel_id":%d,"model":"%s","operation":"generate","request_path":"/v1/images/generations"}]`, channel.Id, modelName),
	}))
	t.Cleanup(func() { require.NoError(t, promptShotConfig.UpdateConfigFromMap(previousPromptShotConfig)) })

	engine := gin.New()
	SetRelayRouter(engine)
	promptShotRequest := func(path string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"prompt":"a safe integration image"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+tokenKey)
		engine.ServeHTTP(recorder, request)
		return recorder
	}

	first := promptShotRequest("/v1/generate-image")
	require.Equalf(t, http.StatusOK, first.Code, "response: %s", first.Body.String())
	assert.Contains(t, first.Body.String(), `"image_b64":"YWJj"`)
	assert.EqualValues(t, 1, upstreamCalls.Load(), "Relay must execute before the compatibility response adapter returns success")

	second := promptShotRequest("/v1/generate-image")
	require.Equal(t, http.StatusTooManyRequests, second.Code)
	assert.EqualValues(t, 1, upstreamCalls.Load(), "the second PromptShot request must be stopped by ModelRequestRateLimit before Distribute/Relay")

	// Standard OpenAI image endpoints retain their own contract and are not
	// replaced by the compatibility layer.
	setting.ModelRequestRateLimitEnabled = false
	standard := httptest.NewRecorder()
	standardRequest := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"`+modelName+`","prompt":"standard image"}`))
	standardRequest.Header.Set("Content-Type", "application/json")
	standardRequest.Header.Set("Authorization", "Bearer "+tokenKey)
	engine.ServeHTTP(standard, standardRequest)
	require.Equal(t, http.StatusOK, standard.Code)
	assert.Contains(t, standard.Body.String(), `"data":[{"b64_json":"YWJj"}]`)
	assert.EqualValues(t, 2, upstreamCalls.Load())
}
