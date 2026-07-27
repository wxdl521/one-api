package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/constant"
	"github.com/QuantumNous/the-one/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestValidateChannelRejectsMultipleAgentPlanAPIKeys(t *testing.T) {
	channel := &model.Channel{
		Type: constant.ChannelTypeVolcEngineAgentPlan,
		Key:  "first-agent-plan-key\nsecond-agent-plan-key",
	}

	require.ErrorContains(t, validateChannel(channel, true), "single API key")
}

func TestAddChannelRejectsAgentPlanBatchMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	payload, err := common.Marshal(AddChannelRequest{
		Mode: "batch",
		Channel: &model.Channel{
			Type: constant.ChannelTypeVolcEngineAgentPlan,
			Key:  "agent-plan-key",
		},
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")

	AddChannel(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success)
	require.Contains(t, response.Message, "do not support batch creation")
}
