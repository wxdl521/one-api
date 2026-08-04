package baidu

import (
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/relaykit/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRequestOpenAI2BaiduPreservesExplicitZeroValues guards the relay DTO
// contract: absent client fields are omitted upstream, while explicit
// 0 / false values survive the conversion instead of being dropped by
// omitempty. penalty_score is the exception: Baidu ERNIE only accepts
// [1.0, 2.0], so an out-of-domain value (including an explicit 0) is dropped
// rather than forwarded, which would otherwise 400 upstream.
func TestRequestOpenAI2BaiduPreservesExplicitZeroValues(t *testing.T) {
	zeroFloat := 0.0
	falseVal := false

	explicit := requestOpenAI2Baidu(dto.GeneralOpenAIRequest{
		TopP:             &zeroFloat,
		FrequencyPenalty: &zeroFloat,
		Stream:           &falseVal,
	})
	explicitJSON, err := common.Marshal(explicit)
	require.NoError(t, err)
	assert.Contains(t, string(explicitJSON), `"top_p":0`)
	assert.Contains(t, string(explicitJSON), `"stream":false`)
	// frequency_penalty:0 is outside Baidu's [1.0, 2.0] domain -> omitted.
	assert.NotContains(t, string(explicitJSON), "penalty_score")

	inDomain := 1.5
	valid := requestOpenAI2Baidu(dto.GeneralOpenAIRequest{FrequencyPenalty: &inDomain})
	validJSON, err := common.Marshal(valid)
	require.NoError(t, err)
	assert.Contains(t, string(validJSON), `"penalty_score":1.5`)

	absent := requestOpenAI2Baidu(dto.GeneralOpenAIRequest{})
	absentJSON, err := common.Marshal(absent)
	require.NoError(t, err)
	assert.NotContains(t, string(absentJSON), "top_p")
	assert.NotContains(t, string(absentJSON), "penalty_score")
	assert.NotContains(t, string(absentJSON), "stream")
}
