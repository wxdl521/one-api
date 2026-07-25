package controller

import (
	"bytes"
	"context"
	"errors"
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

func TestGetAgentPlanChannelUsageReturnsOnlyEnabledEligibleChannels(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	channel := model.Channel{Id: 1, Type: constant.ChannelTypeAdvancedCustom, Key: "agent-plan-key", AgentPlanAccessKey: "agent-plan-access-key", AgentPlanSecretKey: "agent-plan-secret-key", Name: "Volcano"}
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		AgentPlanUsageEnabled: true,
		AdvancedCustom:        &dto.AdvancedCustomConfig{},
	})
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Channel{Id: 2, Type: constant.ChannelTypeOpenAI, Key: "unrelated-key", Name: "Other"}).Error)
	multiKeyChannel := model.Channel{Id: 3, Type: constant.ChannelTypeVolcEngine, Key: "multi-key", Name: "Multi key", ChannelInfo: model.ChannelInfo{IsMultiKey: true}}
	multiKeyChannel.SetOtherSettings(dto.ChannelOtherSettings{AgentPlanUsageEnabled: true})
	require.NoError(t, db.Create(&multiKeyChannel).Error)
	failingChannel := model.Channel{Id: 4, Type: constant.ChannelTypeVolcEngine, Key: "failing-key", AgentPlanAccessKey: "failing-access-key", AgentPlanSecretKey: "failing-secret-key", Name: "Failing"}
	failingChannel.SetOtherSettings(dto.ChannelOtherSettings{AgentPlanUsageEnabled: true})
	require.NoError(t, db.Create(&failingChannel).Error)
	disabledChannel := model.Channel{Id: 5, Type: constant.ChannelTypeVolcEngine, Key: "disabled-key", Name: "Disabled"}
	require.NoError(t, db.Create(&disabledChannel).Error)
	credentialsRequiredChannel := model.Channel{Id: 6, Type: constant.ChannelTypeVolcEngine, Key: "relay-key", Name: "Credentials required"}
	credentialsRequiredChannel.SetOtherSettings(dto.ChannelOtherSettings{AgentPlanUsageEnabled: true})
	require.NoError(t, db.Create(&credentialsRequiredChannel).Error)

	originalFetch := fetchAgentPlanUsageForChannel
	fetchAgentPlanUsageForChannel = func(_ context.Context, channel *model.Channel) (service.AgentPlanUsage, int64, bool, error) {
		if channel.Id == 6 {
			t.Fatal("channels without VolcEngine AccessKey and SecretKey must not call the upstream API")
		}
		if channel.Id == 4 {
			return service.AgentPlanUsage{}, 0, false, errors.New("upstream unavailable")
		}
		assert.Equal(t, "agent-plan-key", channel.Key)
		return service.AgentPlanUsage{
			FiveHour: service.AgentPlanUsageWindow{Remaining: 10},
			Weekly:   service.AgentPlanUsageWindow{Remaining: 20},
			Monthly:  service.AgentPlanUsageWindow{Remaining: 30},
		}, 1778806800, false, nil
	}
	t.Cleanup(func() { fetchAgentPlanUsageForChannel = originalFetch })

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/agent-plan-usage", bytes.NewBufferString(`{"channel_ids":[1,2,3,4,5,6]}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	GetAgentPlanChannelUsage(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    map[string]struct {
			FiveHour  service.AgentPlanUsageWindow `json:"five_hour"`
			Weekly    service.AgentPlanUsageWindow `json:"weekly"`
			Monthly   service.AgentPlanUsageWindow `json:"monthly"`
			UpdatedAt int64                        `json:"updated_at"`
			Stale     bool                         `json:"stale"`
			Status    string                       `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	require.Contains(t, response.Data, "1")
	assert.NotContains(t, response.Data, "2")
	assert.NotContains(t, response.Data, "3")
	assert.NotContains(t, response.Data, "5")
	require.Contains(t, response.Data, "6")
	require.Contains(t, response.Data, "4")
	assert.Equal(t, 10.0, response.Data["1"].FiveHour.Remaining)
	assert.Equal(t, int64(1778806800), response.Data["1"].UpdatedAt)
	assert.Equal(t, "available", response.Data["1"].Status)
	assert.Equal(t, "unavailable", response.Data["4"].Status)
	assert.Equal(t, "credentials_required", response.Data["6"].Status)
	assert.NotContains(t, recorder.Body.String(), "agent-plan-key")
	assert.NotContains(t, recorder.Body.String(), "agent-plan-access-key")
	assert.NotContains(t, recorder.Body.String(), "agent-plan-secret-key")
	assert.NotContains(t, recorder.Body.String(), "failing-key")
}
