package relay

import (
	"testing"

	"github.com/QuantumNous/the-one/constant"
	"github.com/QuantumNous/the-one/relay/channel/task/aiccseedance"
	"github.com/stretchr/testify/require"
)

func TestGetTaskAdaptorReturnsAICCSeedanceAdaptor(t *testing.T) {
	adaptor := GetTaskAdaptor(constant.TaskPlatform("59"))

	require.IsType(t, &aiccseedance.TaskAdaptor{}, adaptor)
}
