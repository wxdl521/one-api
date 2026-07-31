package controller

import (
	"testing"

	"github.com/QuantumNous/the-one/constant"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/relaykit/dto"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeChannelTestEndpointUsesModelSpecificEndpoint(t *testing.T) {
	tests := []struct {
		model string
		want  constant.EndpointType
	}{
		{model: "gpt-5.4-pro", want: constant.EndpointTypeOpenAIResponse},
		{model: "gpt-image-2", want: constant.EndpointTypeImageGeneration},
	}

	for _, test := range tests {
		t.Run(test.model, func(t *testing.T) {
			assert.Equal(t, string(test.want), normalizeChannelTestEndpoint(nil, test.model, ""))
		})
	}

	channel := &model.Channel{Type: constant.ChannelTypeAdvancedCustom}
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		AdvancedCustom: &dto.AdvancedCustomConfig{
			Routes: []dto.AdvancedCustomRoute{
				{
					IncomingPath: "/v1/images/generations",
					UpstreamPath: "/v1/aigc/multimodal-generation/generation",
					Converter:    "openai_image_to_moma_qwen_image",
					Models:       []string{"qwen/qwen-image-2.0-pro"},
				},
			},
		},
	})
	assert.Equal(t, string(constant.EndpointTypeImageGeneration), normalizeChannelTestEndpoint(channel, "qwen/qwen-image-2.0-pro", ""))
}
