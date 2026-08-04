package openai

import (
	"testing"

	"github.com/QuantumNous/the-one/constant"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	relayconstant "github.com/QuantumNous/the-one/relay/constant"
	"github.com/QuantumNous/the-one/relaykit/dto"
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

func TestAzureImageEditURLUsesExplicitFoundryV1API(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeImagesEdits,
		RequestURLPath: "/v1/images/edits",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeAzure,
			ChannelBaseUrl:    "https://resource.services.ai.azure.com",
			UpstreamModelName: "gpt-image-2",
			ApiVersion:        "preview",
			ChannelOtherSettings: dto.ChannelOtherSettings{
				AzureImageAPIStyle: dto.AzureImageAPIStyleFoundryV1,
			},
		},
	}

	url, err := (&Adaptor{}).GetRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, "https://resource.services.ai.azure.com/openai/v1/images/edits?api-version=preview", url)
}

func TestAzureImageGenerationURLUsesExplicitDeploymentAPI(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeImagesGenerations,
		RequestURLPath: "/v1/images/generations",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeAzure,
			ChannelBaseUrl:    "https://resource.services.ai.azure.com",
			UpstreamModelName: "gpt-image-2",
			ApiVersion:        "2025-04-01-preview",
			ChannelOtherSettings: dto.ChannelOtherSettings{
				AzureImageAPIStyle: dto.AzureImageAPIStyleDeployment,
			},
		},
	}

	url, err := (&Adaptor{}).GetRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, "https://resource.services.ai.azure.com/openai/deployments/gpt-image-2/images/generations?api-version=2025-04-01-preview", url)
}
