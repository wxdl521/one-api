package system_setting

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/the-one/setting/config"
	"github.com/stretchr/testify/require"
)

func TestPromptShotConfigPublishesCoherentImmutableSnapshots(t *testing.T) {
	store := newPromptShotConfig()
	first := promptShotPolicyValues("one", 1)
	second := promptShotPolicyValues("two", 2)
	require.NoError(t, store.UpdateConfigFromMap(first))

	writerErr := make(chan error, 1)
	go func() {
		for i := 0; i < 200; i++ {
			values := first
			if i%2 == 1 {
				values = second
			}
			if err := store.UpdateConfigFromMap(values); err != nil {
				writerErr <- err
				return
			}
		}
		writerErr <- nil
	}()

	for i := 0; i < 200; i++ {
		snapshot := store.Snapshot()
		version := snapshot.ReverseModels[0]
		require.True(t, version == "vision-one" || version == "vision-two")
		if version == "vision-one" {
			require.Equal(t, []string{"image-one"}, snapshot.GenerateModels)
			require.Equal(t, []string{"edit-one"}, snapshot.EditModels)
			require.Equal(t, 1, snapshot.Capabilities[0].ChannelID)
		} else {
			require.Equal(t, []string{"image-two"}, snapshot.GenerateModels)
			require.Equal(t, []string{"edit-two"}, snapshot.EditModels)
			require.Equal(t, 2, snapshot.Capabilities[0].ChannelID)
		}
	}
	require.NoError(t, <-writerErr)

	returned := store.Snapshot()
	returned.ReverseModels[0] = "mutated"
	returned.Capabilities[0].Model = "mutated"
	stable := store.Snapshot()
	require.NotEqual(t, "mutated", stable.ReverseModels[0])
	require.NotEqual(t, "mutated", stable.Capabilities[0].Model)
}

func TestPromptShotConfigRejectsMalformedUpdateWithoutReplacingSnapshot(t *testing.T) {
	store := newPromptShotConfig()
	require.NoError(t, store.UpdateConfigFromMap(promptShotPolicyValues("one", 1)))

	err := store.UpdateConfigFromMap(map[string]string{"capabilities": `{not-json`})

	require.Error(t, err)
	require.Equal(t, "vision-one", store.Snapshot().ReverseModels[0])
	require.Equal(t, 1, store.Snapshot().Capabilities[0].ChannelID)
}

func promptShotPolicyValues(version string, channelID int) map[string]string {
	return map[string]string{
		"reverse_models":  `["vision-` + version + `"]`,
		"generate_models": `["image-` + version + `"]`,
		"edit_models":     `["edit-` + version + `"]`,
		"capabilities":    `[{"channel_id":` + strconv.Itoa(channelID) + `,"model":"edit-` + version + `","operation":"edit","request_path":"/v1/images/edits"}]`,
	}
}

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
