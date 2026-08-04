package system_setting

import (
	"testing"

	"github.com/QuantumNous/the-one/setting/config"
	"github.com/stretchr/testify/require"
)

func TestPromptShotSettingNormalizesConfiguredModelsAndCapabilities(t *testing.T) {
	setting := PromptShotSetting{
		ReverseModels:  []string{" vision-primary ", "vision-primary", "", "vision-fallback"},
		GenerateModels: []string{" image-generate ", "image-generate"},
		EditModels:     []string{" image-edit ", "image-edit"},
		Capabilities: []PromptShotChannelCapability{
			{ChannelID: 12, Model: " image-edit ", Operation: PromptShotOperationClean},
			{ChannelID: 12, Model: "image-edit", Operation: PromptShotOperationEdit},
			{ChannelID: 0, Model: "ignored", Operation: PromptShotOperationGenerate},
			{ChannelID: 34, Model: "vision-primary", Operation: PromptShotOperationReverse, RequestPath: "/v1/responses"},
		},
	}

	normalized := setting.Normalized()

	require.Equal(t, []string{"vision-primary", "vision-fallback"}, normalized.ReverseModels)
	require.Equal(t, []string{"image-generate"}, normalized.GenerateModels)
	require.Equal(t, []string{"image-edit"}, normalized.EditModels)
	require.Equal(t, []PromptShotChannelCapability{
		{ChannelID: 12, Model: "image-edit", Operation: PromptShotOperationEdit, RequestPath: "/v1/images/edits"},
		{ChannelID: 34, Model: "vision-primary", Operation: PromptShotOperationReverse, RequestPath: "/v1/responses"},
	}, normalized.Capabilities)
}

func TestPromptShotSettingLoadsPersistedPolicy(t *testing.T) {
	setting := PromptShotSetting{}
	err := config.UpdateConfigFromMap(&setting, map[string]string{
		"reverse_models":  `["vision-primary", "vision-fallback"]`,
		"generate_models": `["image-generate"]`,
		"edit_models":     `["image-edit"]`,
		"capabilities":    `[{"channel_id":8,"model":"image-edit","operation":"edit","request_path":"/v1/images/edits"}]`,
	})

	require.NoError(t, err)
	require.Equal(t, PromptShotSetting{
		ReverseModels:  []string{"vision-primary", "vision-fallback"},
		GenerateModels: []string{"image-generate"},
		EditModels:     []string{"image-edit"},
		Capabilities: []PromptShotChannelCapability{
			{ChannelID: 8, Model: "image-edit", Operation: PromptShotOperationEdit, RequestPath: "/v1/images/edits"},
		},
	}, setting.Normalized())
}
