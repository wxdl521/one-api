package vertex

import (
	"testing"

	"github.com/QuantumNous/the-one/constant"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/QuantumNous/the-one/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskAdaptorBuildsVertexVeoURLWithAPIKey(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType:       constant.ChannelTypeVertexAi,
		ChannelBaseUrl:    "https://aiplatform.googleapis.com",
		ApiKey:            "AIza-test-key",
		ApiVersion:        `{"default":"global"}`,
		UpstreamModelName: "veo-3.1-fast-generate-preview",
		ChannelOtherSettings: dto.ChannelOtherSettings{
			VertexKeyType: dto.VertexKeyTypeAPIKey,
		},
	}}
	adaptor.Init(info)

	url, err := adaptor.BuildRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, "https://aiplatform.googleapis.com/v1/publishers/google/models/veo-3.1-fast-generate-preview:predictLongRunning?key=AIza-test-key", url)
}
