package relay

import (
	"testing"

	"github.com/QuantumNous/the-one/constant"
	"github.com/QuantumNous/the-one/relay/channel/agentplan"
	"github.com/QuantumNous/the-one/relay/channel/task/doubao"
	"github.com/stretchr/testify/require"
)

func TestAgentPlanChannelUsesDedicatedAdaptors(t *testing.T) {
	require.IsType(t, &agentplan.Adaptor{}, GetAdaptor(constant.APITypeVolcEngineAgentPlan))
	require.IsType(t, &doubao.TaskAdaptor{}, GetTaskAdaptor(constant.TaskPlatform("60")))
}
