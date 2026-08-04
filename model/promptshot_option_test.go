package model

import (
	"testing"

	"github.com/QuantumNous/the-one/common"
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
