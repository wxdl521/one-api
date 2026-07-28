package openai

import (
	"testing"

	"github.com/QuantumNous/the-one/constant"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	relayconstant "github.com/QuantumNous/the-one/relay/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAzureGPTImage2GenerationURLUsesV1API(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeImagesGenerations,
		RequestURLPath: "/v1/images/generations",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeAzure,
			ChannelBaseUrl:    "https://resource.services.ai.azure.com",
			UpstreamModelName: "gpt-image-2",
			ApiVersion:        "2025-04-01-preview",
		},
	}

	url, err := (&Adaptor{}).GetRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, "https://resource.services.ai.azure.com/openai/v1/images/generations", url)
}
