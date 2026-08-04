package controller

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMiniTextTestController(t *testing.T) (*gorm.DB, *model.User) {
	t.Helper()
	previousDB := model.DB
	previousDatabaseType := common.MainDatabaseType()
	previousAllowedModels := common.MiniAppAllowedModels
	previousAppID := common.WeChatMiniAppAppID
	previousAppSecret := common.WeChatMiniAppAppSecret
	previousSubjectKey := common.WeChatMiniAppSubjectHMACKey
	previousBindURL := common.MiniAppBindWebBaseURL
	previousMiniProgramEnabled, previousTextTestEnabled := common.GetMiniProgramFeatureFlags()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Ability{}, &model.MiniTextTestAttempt{}))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.MiniAppAllowedModels = "gpt-mini"
	common.WeChatMiniAppAppID = "wx-miniapp-text-controller"
	common.WeChatMiniAppAppSecret = "controller-app-secret"
	common.WeChatMiniAppSubjectHMACKey = "controller-subject-key"
	common.MiniAppBindWebBaseURL = "https://console.example.com/miniapp-bind"
	common.OptionMapRWMutex.Lock()
	common.MiniProgramEnabled = true
	common.MiniProgramTextTestEnabled = true
	common.OptionMapRWMutex.Unlock()
	user := &model.User{
		Username: "mini-text-controller", Password: "placeholder", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", Quota: 100000, AuthVersion: 1,
		AffCode: "mini-text-controller-aff",
	}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "gpt-mini", ChannelId: 1, Enabled: true}).Error)
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
		common.MiniAppAllowedModels = previousAllowedModels
		common.WeChatMiniAppAppID = previousAppID
		common.WeChatMiniAppAppSecret = previousAppSecret
		common.WeChatMiniAppSubjectHMACKey = previousSubjectKey
		common.MiniAppBindWebBaseURL = previousBindURL
		common.OptionMapRWMutex.Lock()
		common.MiniProgramEnabled = previousMiniProgramEnabled
		common.MiniProgramTextTestEnabled = previousTextTestEnabled
		common.OptionMapRWMutex.Unlock()
	})
	return db, user
}

func miniTextTestControllerContext(t *testing.T, body string, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/miniapp/v1/text-tests", bytes.NewBufferString(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("id", userID)
	return context, recorder
}

func TestMiniAppTextTestDuplicateDoesNotRepeatRelayOrCharge(t *testing.T) {
	_, user := setupMiniTextTestController(t)
	previousRunner := miniAppTextTestRelay
	relayCalls := 0
	miniAppTextTestRelay = func(*gin.Context, service.MiniTextTestRequest) service.MiniTextTestCompletion {
		relayCalls++
		return service.MiniTextTestCompletion{
			State:           model.MiniTextTestAttemptStateSucceeded,
			ChargeReference: "server-request-123",
			ChargedQuota:    2468,
		}
	}
	t.Cleanup(func() { miniAppTextTestRelay = previousRunner })

	body := `{"client_request_id":"miniapp-req-controller","model":"gpt-mini","input":"do not persist this prompt"}`
	firstContext, firstRecorder := miniTextTestControllerContext(t, body, user.Id)
	MiniAppStartTextTest(firstContext)
	require.Equal(t, http.StatusOK, firstRecorder.Code)
	assert.NotContains(t, firstRecorder.Body.String(), "do not persist this prompt")

	secondContext, secondRecorder := miniTextTestControllerContext(t, body, user.Id)
	MiniAppStartTextTest(secondContext)
	require.Equal(t, http.StatusOK, secondRecorder.Code)
	assert.Equal(t, 1, relayCalls)
	assert.NotContains(t, secondRecorder.Body.String(), "do not persist this prompt")
}

func TestMiniAppTextTestRejectsClientRelayControls(t *testing.T) {
	_, user := setupMiniTextTestController(t)
	previousRunner := miniAppTextTestRelay
	relayCalls := 0
	miniAppTextTestRelay = func(*gin.Context, service.MiniTextTestRequest) service.MiniTextTestCompletion {
		relayCalls++
		return service.MiniTextTestCompletion{}
	}
	t.Cleanup(func() { miniAppTextTestRelay = previousRunner })

	for _, body := range []string{
		`{"client_request_id":"miniapp-req-stream","model":"gpt-mini","input":"hello","stream":false}`,
		`{"client_request_id":"miniapp-req-tools","model":"gpt-mini","input":"hello","tools":[]}`,
		`{"client_request_id":"miniapp-req-params","model":"gpt-mini","input":"hello","parameters":{}}`,
		`{"client_request_id":"miniapp-req-files","model":"gpt-mini","input":"hello","files":[]}`,
	} {
		context, recorder := miniTextTestControllerContext(t, body, user.Id)
		MiniAppStartTextTest(context)
		require.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "MINIAPP_INVALID_REQUEST")
	}
	assert.Zero(t, relayCalls)
}

func TestMiniAppTextTestRelayResponseWriterDiscardsUpstreamOutput(t *testing.T) {
	writer := newMiniAppTextTestRelayResponseWriter()
	payload := bytes.Repeat([]byte("sensitive upstream output"), 4096)

	n, err := writer.Write(payload)

	require.NoError(t, err)
	assert.Equal(t, len(payload), n)
	assert.Equal(t, http.StatusOK, writer.StatusCode())
	assert.Zero(t, writer.BufferedBytes())
}

func TestMiniAppTextTestRelayContextIgnoresClientCancellationAndRetainsSensitiveLogging(t *testing.T) {
	const requestContextValueKey = "miniapp-relay-context-value"
	callerContext, cancel := context.WithCancel(context.WithValue(context.Background(), requestContextValueKey, "preserved"))
	t.Cleanup(cancel)
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = httptest.NewRequest(http.MethodPost, "/api/miniapp/v1/text-tests", nil).WithContext(callerContext)

	relayContext, stop := miniAppTextTestRelayContext(ginContext)
	t.Cleanup(stop)
	assert.True(t, common.SensitiveRelayPayloadLoggingSuppressed(relayContext))
	assert.Equal(t, "preserved", relayContext.Value(requestContextValueKey))
	cancel()
	select {
	case <-relayContext.Done():
		t.Fatal("the server relay context must outlive client cancellation")
	default:
	}

	completion, terminal := miniAppTextTestContextCompletion("relay-request-123", relayContext.Err())
	assert.False(t, terminal)
	assert.Equal(t, service.MiniTextTestCompletion{}, completion)
}
