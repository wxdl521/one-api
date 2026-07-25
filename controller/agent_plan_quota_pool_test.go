package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAgentPlanQuotaPoolControllerTestDB(t *testing.T) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.UserSubscription{},
		&model.AgentPlanQuotaPool{},
		&model.AgentPlanPackagePlan{},
		&model.AgentPlanPackageAllocation{},
	))
}

func createAgentPlanQuotaPoolSource(t *testing.T, id int, multiKey bool, accessKey string, secretKey string) *model.Channel {
	t.Helper()
	channel := &model.Channel{
		Id:                 id,
		Type:               constant.ChannelTypeVolcEngine,
		Name:               "Agent Plan source",
		Key:                "relay-key",
		AgentPlanAccessKey: accessKey,
		AgentPlanSecretKey: secretKey,
		ChannelInfo:        model.ChannelInfo{IsMultiKey: multiKey},
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{AgentPlanUsageEnabled: true})
	require.NoError(t, model.DB.Create(channel).Error)
	return channel
}

func TestAdminCreateAgentPlanQuotaPoolRejectsInvalidSources(t *testing.T) {
	tests := []struct {
		name       string
		multiKey   bool
		access     string
		secret     string
		multiplier string
	}{
		{name: "multi key", multiKey: true, access: "access-key", secret: "secret-key", multiplier: "1.5"},
		{name: "missing credentials", multiplier: "1.5"},
		{name: "invalid multiplier", access: "access-key", secret: "secret-key", multiplier: "-1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAgentPlanQuotaPoolControllerTestDB(t)
			channel := createAgentPlanQuotaPoolSource(t, 1, test.multiKey, test.access, test.secret)

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/subscription/admin/agent-plan-pools", bytes.NewBufferString(fmt.Sprintf(`{"name":"Pool","source_channel_id":%d,"display_multiplier":%s}`, channel.Id, test.multiplier)))
			ctx.Request.Header.Set("Content-Type", "application/json")

			AdminCreateAgentPlanQuotaPool(ctx)

			require.Equal(t, http.StatusOK, recorder.Code)
			assert.Contains(t, recorder.Body.String(), `"success":false`)
			var count int64
			require.NoError(t, model.DB.Model(&model.AgentPlanQuotaPool{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestAdminListAgentPlanQuotaPoolsHidesCredentialsAndReturnsStock(t *testing.T) {
	setupAgentPlanQuotaPoolControllerTestDB(t)
	channel := createAgentPlanQuotaPoolSource(t, 1, false, "agent-plan-access-key", "agent-plan-secret-key")
	pool := &model.AgentPlanQuotaPool{
		SourceChannelId:                   channel.Id,
		Name:                              "Pool",
		DisplayMultiplierMicros:           service.AgentPlanMultiplierMicros,
		OfficialMonthlyRemainingAFPMicros: 100 * service.AgentPlanAFPMicros,
		SyncedAt:                          1,
		SyncStatus:                        model.AgentPlanQuotaPoolSyncStatusAvailable,
	}
	require.NoError(t, model.DB.Create(pool).Error)
	subscription := &model.UserSubscription{UserId: 1, PlanId: 1, AmountTotal: 100, AmountUsed: 25, EndTime: 4_102_444_800, Status: "active"}
	require.NoError(t, model.DB.Create(subscription).Error)
	require.NoError(t, model.DB.Create(&model.AgentPlanPackageAllocation{UserSubscriptionId: subscription.Id, PoolId: pool.Id, AllocationAFPMicros: 40 * service.AgentPlanAFPMicros, DisplayMultiplierMicros: service.AgentPlanMultiplierMicros}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/subscription/admin/agent-plan-pools", nil)

	AdminListAgentPlanQuotaPools(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    []struct {
			Id    int                             `json:"id"`
			Stock service.AgentPlanQuotaPoolStock `json:"stock"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 1)
	assert.Equal(t, pool.Id, response.Data[0].Id)
	assert.Equal(t, int64(30*service.AgentPlanAFPMicros), response.Data[0].Stock.ActiveReservationAFPMicros)
	assert.Equal(t, int64(70*service.AgentPlanAFPMicros), response.Data[0].Stock.SellableAFPMicros)
	assert.NotContains(t, recorder.Body.String(), "relay-key")
	assert.NotContains(t, recorder.Body.String(), "agent-plan-access-key")
	assert.NotContains(t, recorder.Body.String(), "agent-plan-secret-key")
}

func TestAdminSyncAgentPlanQuotaPoolSafelyReportsFailure(t *testing.T) {
	setupAgentPlanQuotaPoolControllerTestDB(t)
	channel := createAgentPlanQuotaPoolSource(t, 1, false, "agent-plan-access-key", "agent-plan-secret-key")
	pool := &model.AgentPlanQuotaPool{SourceChannelId: channel.Id, Name: "Pool", DisplayMultiplierMicros: service.AgentPlanMultiplierMicros}
	require.NoError(t, model.DB.Create(pool).Error)
	require.NoError(t, model.DB.Model(channel).Update("agent_plan_secret_key", "").Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/subscription/admin/agent-plan-pools/1/sync", nil)

	AdminSyncAgentPlanQuotaPool(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":false`)
	assert.NotContains(t, recorder.Body.String(), "agent-plan-access-key")
	assert.NotContains(t, recorder.Body.String(), "agent-plan-secret-key")
	var updated model.AgentPlanQuotaPool
	require.NoError(t, model.DB.First(&updated, pool.Id).Error)
	assert.Equal(t, model.AgentPlanQuotaPoolSyncStatusError, updated.SyncStatus)
	assert.NotEmpty(t, updated.SyncError)
}

func TestAdminListAgentPlanQuotaPoolEligibleSourceChannelsDoesNotExposeSecrets(t *testing.T) {
	setupAgentPlanQuotaPoolControllerTestDB(t)
	createAgentPlanQuotaPoolSource(t, 1, false, "agent-plan-access-key", "agent-plan-secret-key")
	createAgentPlanQuotaPoolSource(t, 2, false, "", "")

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/subscription/admin/agent-plan-pools/eligible-source-channels", nil)

	AdminListAgentPlanQuotaPoolEligibleSourceChannels(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	assert.Contains(t, recorder.Body.String(), `"id":1`)
	assert.NotContains(t, recorder.Body.String(), `"id":2`)
	assert.NotContains(t, recorder.Body.String(), "relay-key")
	assert.NotContains(t, recorder.Body.String(), "agent-plan-access-key")
	assert.NotContains(t, recorder.Body.String(), "agent-plan-secret-key")
}

func TestValidateAgentPlanPackagePlanRejectsSubMicroAFP(t *testing.T) {
	setupAgentPlanQuotaPoolControllerTestDB(t)
	channel := createAgentPlanQuotaPoolSource(t, 1, false, "access-key", "secret-key")
	pool := &model.AgentPlanQuotaPool{SourceChannelId: channel.Id, Name: "Pool", DisplayMultiplierMicros: service.AgentPlanMultiplierMicros}
	require.NoError(t, model.DB.Create(pool).Error)

	_, err := validateAgentPlanPackagePlan(&AdminUpsertSubscriptionPlanRequest{
		Plan: model.SubscriptionPlan{TotalAmount: 1, QuotaResetPeriod: model.SubscriptionResetNever},
		AgentPlanPackage: &AgentPlanPackagePlanDTO{
			PoolId:        pool.Id,
			AllocationAFP: 0.0000004,
			ScopeGroup:    "agent",
			ScopeModels:   []string{"glm-5.2"},
		},
	})

	require.Error(t, err)
}

func TestAdminDeleteAgentPlanQuotaPoolRejectsAnyReferences(t *testing.T) {
	for _, reference := range []string{"package plan", "allocation"} {
		t.Run(reference, func(t *testing.T) {
			setupAgentPlanQuotaPoolControllerTestDB(t)
			channel := createAgentPlanQuotaPoolSource(t, 1, false, "access-key", "secret-key")
			pool := &model.AgentPlanQuotaPool{SourceChannelId: channel.Id, Name: "Pool", DisplayMultiplierMicros: service.AgentPlanMultiplierMicros}
			require.NoError(t, model.DB.Create(pool).Error)
			if reference == "package plan" {
				require.NoError(t, model.DB.Create(&model.AgentPlanPackagePlan{SubscriptionPlanId: 1, PoolId: pool.Id, AllocationAFPMicros: service.AgentPlanAFPMicros, ScopeGroup: "agent", ScopeModels: `["glm-5.2"]`}).Error)
			} else {
				require.NoError(t, model.DB.Create(&model.AgentPlanPackageAllocation{UserSubscriptionId: 1, PoolId: pool.Id, AllocationAFPMicros: service.AgentPlanAFPMicros, DisplayMultiplierMicros: service.AgentPlanMultiplierMicros, ScopeGroup: "agent", ScopeModels: `["glm-5.2"]`}).Error)
			}

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(pool.Id)}}
			ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/subscription/admin/agent-plan-pools/1", nil)
			AdminDeleteAgentPlanQuotaPool(ctx)

			assert.Contains(t, recorder.Body.String(), `"success":false`)
			var persisted model.AgentPlanQuotaPool
			require.NoError(t, model.DB.First(&persisted, pool.Id).Error)
		})
	}
}

func TestAdminUpdateAgentPlanQuotaPoolRejectsSourceChangeAfterReferences(t *testing.T) {
	setupAgentPlanQuotaPoolControllerTestDB(t)
	first := createAgentPlanQuotaPoolSource(t, 1, false, "access-key-1", "secret-key-1")
	second := createAgentPlanQuotaPoolSource(t, 2, false, "access-key-2", "secret-key-2")
	pool := &model.AgentPlanQuotaPool{SourceChannelId: first.Id, Name: "Pool", DisplayMultiplierMicros: service.AgentPlanMultiplierMicros}
	require.NoError(t, model.DB.Create(pool).Error)
	require.NoError(t, model.DB.Create(&model.AgentPlanPackagePlan{SubscriptionPlanId: 1, PoolId: pool.Id, AllocationAFPMicros: service.AgentPlanAFPMicros, ScopeGroup: "agent", ScopeModels: `["glm-5.2"]`}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(pool.Id)}}
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/subscription/admin/agent-plan-pools/1", bytes.NewBufferString(fmt.Sprintf(`{"source_channel_id":%d}`, second.Id)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	AdminUpdateAgentPlanQuotaPool(ctx)

	assert.Contains(t, recorder.Body.String(), `"success":false`)
	var persisted model.AgentPlanQuotaPool
	require.NoError(t, model.DB.First(&persisted, pool.Id).Error)
	assert.Equal(t, first.Id, persisted.SourceChannelId)
}

func TestAgentPlanQuotaPoolRejectsDuplicateSourceChannel(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		setupAgentPlanQuotaPoolControllerTestDB(t)
		channel := createAgentPlanQuotaPoolSource(t, 1, false, "access-key", "secret-key")
		require.NoError(t, model.DB.Create(&model.AgentPlanQuotaPool{SourceChannelId: channel.Id, Name: "Existing", DisplayMultiplierMicros: service.AgentPlanMultiplierMicros}).Error)

		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/subscription/admin/agent-plan-pools", bytes.NewBufferString(`{"name":"Duplicate","source_channel_id":1,"display_multiplier":1}`))
		ctx.Request.Header.Set("Content-Type", "application/json")
		AdminCreateAgentPlanQuotaPool(ctx)

		assert.Contains(t, recorder.Body.String(), `"success":false`)
		var count int64
		require.NoError(t, model.DB.Model(&model.AgentPlanQuotaPool{}).Count(&count).Error)
		assert.EqualValues(t, 1, count)
	})

	t.Run("update", func(t *testing.T) {
		setupAgentPlanQuotaPoolControllerTestDB(t)
		first := createAgentPlanQuotaPoolSource(t, 1, false, "access-key-1", "secret-key-1")
		second := createAgentPlanQuotaPoolSource(t, 2, false, "access-key-2", "secret-key-2")
		require.NoError(t, model.DB.Create(&model.AgentPlanQuotaPool{SourceChannelId: first.Id, Name: "First", DisplayMultiplierMicros: service.AgentPlanMultiplierMicros}).Error)
		secondPool := &model.AgentPlanQuotaPool{SourceChannelId: second.Id, Name: "Second", DisplayMultiplierMicros: service.AgentPlanMultiplierMicros}
		require.NoError(t, model.DB.Create(secondPool).Error)

		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(secondPool.Id)}}
		ctx.Request = httptest.NewRequest(http.MethodPut, "/api/subscription/admin/agent-plan-pools/2", bytes.NewBufferString(fmt.Sprintf(`{"source_channel_id":%d}`, first.Id)))
		ctx.Request.Header.Set("Content-Type", "application/json")
		AdminUpdateAgentPlanQuotaPool(ctx)

		assert.Contains(t, recorder.Body.String(), `"success":false`)
		var persisted model.AgentPlanQuotaPool
		require.NoError(t, model.DB.First(&persisted, secondPool.Id).Error)
		assert.Equal(t, second.Id, persisted.SourceChannelId)
	})
}

func TestGetSubscriptionSelfReturnsAgentPlanWalletFallbackPreference(t *testing.T) {
	setupAgentPlanQuotaPoolControllerTestDB(t)
	user := &model.User{Id: 9, Username: "subscription-user", Status: common.UserStatusEnabled}
	user.SetSetting(dto.UserSetting{AgentPlanWalletFallbackEnabled: true})
	require.NoError(t, model.DB.Create(user).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/subscription/self", nil)
	ctx.Set("id", user.Id)
	GetSubscriptionSelf(ctx)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			AgentPlanWalletFallbackEnabled bool `json:"agent_plan_wallet_fallback_enabled"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	assert.True(t, response.Data.AgentPlanWalletFallbackEnabled)
}
