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
		ApiVersion:        `{"default":"us-central1"}`,
		UpstreamModelName: "veo-3.1-fast-generate-001",
		ChannelOtherSettings: dto.ChannelOtherSettings{
			VertexKeyType:   dto.VertexKeyTypeAPIKey,
			VertexProjectID: "test-project",
		},
	}}
	adaptor.Init(info)

	url, err := adaptor.BuildRequestURL(info)

	require.NoError(t, err)
	assert.Equal(t, "https://us-central1-aiplatform.googleapis.com/v1/projects/test-project/locations/us-central1/publishers/google/models/veo-3.1-fast-generate-001:predictLongRunning?key=AIza-test-key", url)
}

func TestTaskAdaptorListsStableVeo31Models(t *testing.T) {
	adaptor := &TaskAdaptor{}

	assert.Contains(t, adaptor.GetModelList(), "veo-3.1-generate-001")
	assert.Contains(t, adaptor.GetModelList(), "veo-3.1-fast-generate-001")
}
