package openai

import (
	"testing"
	"time"

	"github.com/QuantumNous/the-one/constant"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	relayconstant "github.com/QuantumNous/the-one/relay/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAzureRealtimeURLUsesGAOrPreviewProtocolForItsModel(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		createdAt int64
		want      string
	}{
		{
			name:      "GA model",
			model:     "gpt-realtime-2.1",
			createdAt: time.Now().Unix(),
			want:      "wss://resource.openai.azure.com/openai/v1/realtime?model=gpt-realtime-2.1",
		},
		{
			name:  "GA model on a legacy channel",
			model: "gpt-realtime-2.1",
			want:  "wss://resource.openai.azure.com/openai/v1/realtime?model=gpt-realtime-2.1",
		},
		{
			name:      "legacy preview model",
			model:     "gpt-4o-realtime-preview",
			createdAt: time.Now().Unix(),
			want:      "wss://resource.openai.azure.com/openai/realtime?deployment=gpt-4o-realtime-preview&api-version=2025-04-01-preview",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				RelayMode: relayconstant.RelayModeRealtime,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:       constant.ChannelTypeAzure,
					ChannelBaseUrl:    "https://resource.openai.azure.com",
					UpstreamModelName: test.model,
					ApiVersion:        "2025-04-01-preview",
					ChannelCreateTime: test.createdAt,
				},
			}

			url, err := (&Adaptor{}).GetRequestURL(info)

			require.NoError(t, err)
			assert.Equal(t, test.want, url)
		})
	}
}
