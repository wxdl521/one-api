package system_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChatVideoBridgeSettingNormalizesUnsafeValues(t *testing.T) {
	setting := ChatVideoBridgeSetting{
		Models:             []string{" model-a ", "model-a", "", "model-b"},
		MaxWaitSeconds:     999,
		TaskPageTTLSeconds: 1,
	}

	normalized := setting.Normalized()

	assert.Equal(t, []string{"model-a", "model-b"}, normalized.Models)
	assert.Equal(t, maxChatVideoBridgeWaitSeconds, normalized.MaxWaitSeconds)
	assert.Equal(t, minChatVideoBridgeTicketTTLSeconds, normalized.TaskPageTTLSeconds)
}

func TestChatVideoBridgeSettingAllowsAnImmediateFallback(t *testing.T) {
	normalized := (ChatVideoBridgeSetting{}).Normalized()

	assert.Equal(t, 0, normalized.MaxWaitSeconds)
	assert.Equal(t, minChatVideoBridgeTicketTTLSeconds, normalized.TaskPageTTLSeconds)
}
