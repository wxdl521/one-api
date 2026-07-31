package controller

import (
	"testing"

	"github.com/QuantumNous/the-one/constant"
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
}
