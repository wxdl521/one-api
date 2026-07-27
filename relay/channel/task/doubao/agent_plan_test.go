package doubao

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/the-one/constant"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentPlanTaskAdaptorUsesPlanVideoTaskRoutes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/api/plan/v3/contents/generations/tasks/upstream-task", request.URL.Path)
		assert.Equal(t, "Bearer agent-plan-api-key", request.Header.Get("Authorization"))
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"upstream-task"}`))
	}))
	defer server.Close()

	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType:    constant.ChannelTypeVolcEngineAgentPlan,
		ChannelBaseUrl: server.URL + "/api/plan/v3",
		ApiKey:         "agent-plan-api-key",
	}})

	url, err := adaptor.BuildRequestURL(nil)
	require.NoError(t, err)
	assert.Equal(t, server.URL+"/api/plan/v3/contents/generations/tasks", url)

	response, err := adaptor.FetchTask(server.URL+"/api/plan/v3", "agent-plan-api-key", map[string]any{
		"task_id": "upstream-task",
	}, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	_ = response.Body.Close()
}
