package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvancedCustomChannelRequiresModelListRouteOnlyWhenUpdateChecksEnabled(t *testing.T) {
	inferenceRoute := dto.AdvancedCustomRoute{
		IncomingPath: "/v1/chat/completions",
		UpstreamPath: "/v1/chat/completions",
		Converter:    "none",
	}

	tests := []struct {
		name          string
		checksEnabled bool
		routes        []dto.AdvancedCustomRoute
		wantErr       string
	}{
		{
			name:   "legacy channel without discovery route remains valid",
			routes: []dto.AdvancedCustomRoute{inferenceRoute},
		},
		{
			name:          "enabled checks require discovery route",
			checksEnabled: true,
			routes:        []dto.AdvancedCustomRoute{inferenceRoute},
			wantErr:       dto.AdvancedCustomModelListPath,
		},
		{
			name:          "enabled checks accept discovery route",
			checksEnabled: true,
			routes: []dto.AdvancedCustomRoute{
				inferenceRoute,
				{
					IncomingPath: dto.AdvancedCustomModelListPath,
					UpstreamPath: dto.AdvancedCustomModelListPath,
					Converter:    "none",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &Channel{Type: constant.ChannelTypeAdvancedCustom}
			channel.SetOtherSettings(dto.ChannelOtherSettings{
				UpstreamModelUpdateCheckEnabled: tt.checksEnabled,
				AdvancedCustom: &dto.AdvancedCustomConfig{
					Routes: tt.routes,
				},
			})

			err := channel.ValidateSettings()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestChannelOtherSettingsPreservesAgentPlanUsageEnabled(t *testing.T) {
	channel := &Channel{}
	channel.SetOtherSettings(dto.ChannelOtherSettings{AgentPlanUsageEnabled: true})

	assert.True(t, channel.GetOtherSettings().AgentPlanUsageEnabled)
}

func TestAgentPlanUsageRequiresEligibleSingleKeyChannel(t *testing.T) {
	tests := []struct {
		name    string
		channel Channel
		wantErr string
	}{
		{
			name: "VolcEngine single-key channel is eligible",
			channel: Channel{
				Type: constant.ChannelTypeVolcEngine,
			},
		},
		{
			name: "unsupported channel type is rejected",
			channel: Channel{
				Type: constant.ChannelTypeOpenAI,
			},
			wantErr: "VolcEngine or Advanced Custom",
		},
		{
			name: "multi-key channel is rejected",
			channel: Channel{
				Type:        constant.ChannelTypeVolcEngine,
				ChannelInfo: ChannelInfo{IsMultiKey: true},
			},
			wantErr: "single-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.channel.SetOtherSettings(dto.ChannelOtherSettings{
				AgentPlanUsageEnabled: true,
			})

			err := tt.channel.ValidateSettings()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
