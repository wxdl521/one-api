package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/setting/config"
	"github.com/QuantumNous/the-one/setting/system_setting"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionRejectsMalformedPromptShotConfigBeforePersisting(t *testing.T) {
	db := useFrontendOptionMigrationDB(t)
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	for _, key := range []string{
		"promptshot.reverse_models",
		"promptshot.generate_models",
		"promptshot.edit_models",
		"promptshot.capabilities",
	} {
		err := UpdateOption(key, `{not-json`)

		require.Error(t, err, key)
		requireOptionMissing(t, db, key)
	}
}

func TestUpdateOptionsBulkPublishesOnlyCompletePromptShotPolicies(t *testing.T) {
	useFrontendOptionMigrationDB(t)
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	promptShotConfig := config.GlobalConfig.Get("promptshot")
	previousConfig, err := config.ConfigToMap(promptShotConfig)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, config.UpdateConfigFromMap(promptShotConfig, previousConfig))
	})

	oldValues := promptShotOptionValues(t, "old", 101)
	newValues := promptShotOptionValues(t, "new", 202)
	require.NoError(t, UpdateOptionsBulk(oldValues))
	require.NoError(t, validatePromptShotSnapshotVersion(system_setting.GetPromptShotSetting(), "old", 101))

	started := make(chan struct{})
	done := make(chan struct{})
	readErr := make(chan error, 1)
	go func() {
		close(started)
		for {
			select {
			case <-done:
				return
			default:
			}
			snapshot := system_setting.GetPromptShotSetting()
			if err := validatePromptShotSnapshotVersion(snapshot, "old", 101); err != nil {
				if err := validatePromptShotSnapshotVersion(snapshot, "new", 202); err != nil {
					readErr <- fmt.Errorf("observed mixed PromptShot policy: %w", err)
					return
				}
			}
		}
	}()
	<-started

	require.NoError(t, UpdateOptionsBulk(newValues))
	close(done)
	select {
	case err := <-readErr:
		require.NoError(t, err)
	default:
	}
	require.NoError(t, validatePromptShotSnapshotVersion(system_setting.GetPromptShotSetting(), "new", 202))
}

func promptShotOptionValues(t *testing.T, version string, channelID int) map[string]string {
	t.Helper()
	models := func(kind string) []string {
		values := make([]string, 0, 2048)
		for i := 0; i < 2048; i++ {
			values = append(values, fmt.Sprintf("%s-%s-%d", version, kind, i))
		}
		return values
	}
	encode := func(value any) string {
		encoded, err := common.Marshal(value)
		require.NoError(t, err)
		return string(encoded)
	}
	return map[string]string{
		"promptshot.reverse_models":  encode(models("reverse")),
		"promptshot.generate_models": encode(models("generate")),
		"promptshot.edit_models":     encode(models("edit")),
		"promptshot.capabilities": encode([]system_setting.PromptShotChannelCapability{
			{ChannelID: channelID, Model: version + "-edit-0", Operation: system_setting.PromptShotOperationEdit},
		}),
	}
}

func validatePromptShotSnapshotVersion(setting system_setting.PromptShotSetting, version string, channelID int) error {
	if len(setting.ReverseModels) != 2048 || setting.ReverseModels[0] != version+"-reverse-0" {
		return fmt.Errorf("reverse models do not match %s", version)
	}
	if len(setting.GenerateModels) != 2048 || setting.GenerateModels[0] != version+"-generate-0" {
		return fmt.Errorf("generate models do not match %s", version)
	}
	if len(setting.EditModels) != 2048 || setting.EditModels[0] != version+"-edit-0" {
		return fmt.Errorf("edit models do not match %s", version)
	}
	if len(setting.Capabilities) != 1 || setting.Capabilities[0].ChannelID != channelID || setting.Capabilities[0].Model != version+"-edit-0" {
		return fmt.Errorf("capabilities do not match %s", version)
	}
	return nil
}
