package constant

import "github.com/QuantumNous/the-one/relaykit/types"

// Finish reasons moved to types with the conversion kit.
var (
	FinishReasonStop          = types.FinishReasonStop
	FinishReasonToolCalls     = types.FinishReasonToolCalls
	FinishReasonLength        = types.FinishReasonLength
	FinishReasonFunctionCall  = types.FinishReasonFunctionCall
	FinishReasonContentFilter = types.FinishReasonContentFilter
)
