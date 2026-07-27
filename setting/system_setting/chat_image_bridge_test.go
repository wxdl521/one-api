package system_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChatImageBridgeSettingNormalizesBounds(t *testing.T) {
	got := (ChatImageBridgeSetting{MaxWaitSeconds: 999, MediaTTLSeconds: 1}).Normalized()

	assert.Equal(t, 600, got.MaxWaitSeconds)
	assert.Equal(t, 300, got.MediaTTLSeconds)
}
