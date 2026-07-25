package relay

import (
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/QuantumNous/the-one/relaykit/types"
)

func theOneErrorFromParamOverride(err error) *types.TheOneError {
	if fixedErr, ok := relaycommon.AsParamOverrideReturnError(err); ok {
		return relaycommon.TheOneErrorFromParamOverride(fixedErr)
	}
	return types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid, types.ErrOptionWithSkipRetry())
}
