package aws

import (
	"testing"

	"github.com/QuantumNous/the-one/relaykit/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertToNovaRequestPreservesExplicitZeroValues guards the relay DTO
// contract for sampling params: an explicit 0 for temperature/topP/topK is a
// valid point in each range (0-1 / 0-128), so it must survive the conversion
// and be sent upstream rather than silently dropped.
func TestConvertToNovaRequestPreservesExplicitZeroValues(t *testing.T) {
	zeroFloat := 0.0
	zeroInt := 0

	req := &dto.GeneralOpenAIRequest{
		Model:       "amazon.nova-lite-v1:0",
		Temperature: &zeroFloat,
		TopP:        &zeroFloat,
		TopK:        &zeroInt,
	}

	novaReq := convertToNovaRequest(req)

	require.NotNil(t, novaReq.InferenceConfig)
	require.NotNil(t, novaReq.InferenceConfig.Temperature)
	assert.Equal(t, 0.0, *novaReq.InferenceConfig.Temperature)
	require.NotNil(t, novaReq.InferenceConfig.TopP)
	assert.Equal(t, 0.0, *novaReq.InferenceConfig.TopP)
	require.NotNil(t, novaReq.InferenceConfig.TopK)
	assert.Equal(t, 0, *novaReq.InferenceConfig.TopK)
}

// TestConvertToNovaRequestOmitsZeroMaxTokens locks the deliberate exception:
// max_tokens=0 is not a valid value (Nova requires maxTokens>=1), so it is
// treated as unset and omitted to fall back to the provider default, avoiding
// an upstream ValidationException. Unlike sampling params, 0 is meaningless here.
func TestConvertToNovaRequestOmitsZeroMaxTokens(t *testing.T) {
	zeroUint := uint(0)
	nonZeroFloat := 0.7

	// max_tokens=0 alone must not even create an InferenceConfig.
	onlyZeroMax := convertToNovaRequest(&dto.GeneralOpenAIRequest{
		Model:     "amazon.nova-lite-v1:0",
		MaxTokens: &zeroUint,
	})
	assert.Nil(t, onlyZeroMax.InferenceConfig)

	// With other params present, max_tokens=0 is dropped while the rest survive.
	mixed := convertToNovaRequest(&dto.GeneralOpenAIRequest{
		Model:       "amazon.nova-lite-v1:0",
		MaxTokens:   &zeroUint,
		Temperature: &nonZeroFloat,
	})
	require.NotNil(t, mixed.InferenceConfig)
	assert.Nil(t, mixed.InferenceConfig.MaxTokens)
	require.NotNil(t, mixed.InferenceConfig.Temperature)
	assert.Equal(t, 0.7, *mixed.InferenceConfig.Temperature)
}

func TestConvertToNovaRequestOmitsAbsentInferenceConfig(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{Model: "amazon.nova-lite-v1:0"}

	novaReq := convertToNovaRequest(req)

	assert.Nil(t, novaReq.InferenceConfig)
}
