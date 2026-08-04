package service

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMiniTextTestService(t *testing.T) (*gorm.DB, *model.User) {
	t.Helper()
	previousDB := model.DB
	previousDatabaseType := common.MainDatabaseType()
	previousAllowedModels := common.MiniAppAllowedModels
	previousMiniProgramEnabled, previousTextTestEnabled := common.GetMiniProgramFeatureFlags()
	previousConfigOnce := miniAppConfigOnce
	previousConfig := cachedMiniAppConfig
	previousConfigErr := cachedMiniAppConfigErr

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Ability{}, &model.MiniTextTestAttempt{}))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.MiniAppAllowedModels = "gpt-mini"
	common.OptionMapRWMutex.Lock()
	common.MiniProgramEnabled = true
	common.MiniProgramTextTestEnabled = true
	common.OptionMapRWMutex.Unlock()
	miniAppConfigOnce = sync.Once{}
	cachedMiniAppConfig = MiniAppConfig{}
	cachedMiniAppConfigErr = nil
	cachedMiniAppConfig, cachedMiniAppConfigErr = newMiniAppConfig(
		"wx-miniapp-text-test", "app-secret", "text-test-hmac-key", "https://console.example.com/miniapp-bind", time.Second, false,
	)
	miniAppConfigOnce.Do(func() {})

	user := &model.User{
		Username: "mini-text-user", Password: "placeholder", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", Quota: 100000, AuthVersion: 1,
		AffCode: "mini-text-user-aff",
	}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "gpt-mini", ChannelId: 1, Enabled: true}).Error)
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
		common.MiniAppAllowedModels = previousAllowedModels
		common.OptionMapRWMutex.Lock()
		common.MiniProgramEnabled = previousMiniProgramEnabled
		common.MiniProgramTextTestEnabled = previousTextTestEnabled
		common.OptionMapRWMutex.Unlock()
		miniAppConfigOnce = previousConfigOnce
		cachedMiniAppConfig = previousConfig
		cachedMiniAppConfigErr = previousConfigErr
	})
	return db, user
}

func TestStartMiniTextTestIsIdempotentAndDoesNotPersistPrompt(t *testing.T) {
	db, user := setupMiniTextTestService(t)
	request := MiniTextTestRequest{
		ClientRequestID: "miniapp-req-12345678",
		Model:           "gpt-mini",
		Input:           "the prompt must never persist",
	}

	first, created, err := StartMiniTextTest(user.Id, request)
	require.NoError(t, err)
	require.True(t, created)
	assert.Equal(t, model.MiniTextTestAttemptStateRunning, first.State)
	assert.NotContains(t, common.GetJsonString(first), request.Input)

	second, created, err := StartMiniTextTest(user.Id, request)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, first.RequestID, second.RequestID)

	_, _, err = StartMiniTextTest(user.Id, MiniTextTestRequest{
		ClientRequestID: request.ClientRequestID,
		Model:           request.Model,
		Input:           "a different prompt",
	})
	require.ErrorIs(t, err, ErrMiniTextTestRequestConflict)

	var stored model.MiniTextTestAttempt
	require.NoError(t, db.First(&stored).Error)
	assert.NotEqual(t, request.Input, stored.InputHMAC)
	assert.Len(t, stored.InputHMAC, 64)
	assert.NotContains(t, common.GetJsonString(stored), request.Input)
}

func TestStartMiniTextTestRejectsOverlongInputAndUnavailableModels(t *testing.T) {
	_, user := setupMiniTextTestService(t)
	_, _, err := StartMiniTextTest(user.Id, MiniTextTestRequest{
		ClientRequestID: "miniapp-req-overlong",
		Model:           "gpt-mini",
		Input:           strings.Repeat("好", 4001),
	})
	require.ErrorIs(t, err, ErrMiniTextTestInvalid)

	_, _, err = StartMiniTextTest(user.Id, MiniTextTestRequest{
		ClientRequestID: "miniapp-req-forbidden",
		Model:           "gpt-not-allowed",
		Input:           "hello",
	})
	require.True(t, errors.Is(err, ErrMiniTextTestModelUnavailable))
}

func TestCompleteMiniTextTestStoresOnlySafeTerminalMetadata(t *testing.T) {
	_, user := setupMiniTextTestService(t)
	request := MiniTextTestRequest{
		ClientRequestID: "miniapp-req-complete",
		Model:           "gpt-mini",
		Input:           "prompt text that must not be retained",
	}
	_, created, err := StartMiniTextTest(user.Id, request)
	require.NoError(t, err)
	require.True(t, created)

	status, err := CompleteMiniTextTest(user.Id, request.ClientRequestID, MiniTextTestCompletion{
		State:           model.MiniTextTestAttemptStateSucceeded,
		ChargeReference: "server-request-123",
		ChargedQuota:    4321,
	})
	require.NoError(t, err)
	assert.Equal(t, model.MiniTextTestAttemptStateSucceeded, status.State)
	assert.Equal(t, "server-request-123", status.ChargeReference)
	assert.Equal(t, 4321, status.ChargedQuota)
	assert.Empty(t, status.ErrorCode)
	assert.NotContains(t, common.GetJsonString(status), request.Input)
}

func TestGetMiniTextTestStatusReturnsSafeNotFoundError(t *testing.T) {
	_, user := setupMiniTextTestService(t)

	status, err := GetMiniTextTestStatus(user.Id, "miniapp-req-missing")
	require.Nil(t, status)
	require.ErrorIs(t, err, ErrMiniTextTestNotFound)

	status, err = GetMiniTextTestStatus(user.Id, "not valid")
	require.Nil(t, status)
	require.ErrorIs(t, err, ErrMiniTextTestInvalid)
}

func TestCompleteMiniTextTestRequiresChargeReference(t *testing.T) {
	_, user := setupMiniTextTestService(t)
	request := MiniTextTestRequest{
		ClientRequestID: "miniapp-req-charge-ref",
		Model:           "gpt-mini",
		Input:           "safe input",
	}
	_, created, err := StartMiniTextTest(user.Id, request)
	require.NoError(t, err)
	require.True(t, created)

	_, err = CompleteMiniTextTest(user.Id, request.ClientRequestID, MiniTextTestCompletion{
		State:     model.MiniTextTestAttemptStateFailed,
		ErrorCode: "MINIAPP_TEXT_TEST_UNAVAILABLE",
	})
	require.ErrorIs(t, err, ErrMiniTextTestInvalid)
}
