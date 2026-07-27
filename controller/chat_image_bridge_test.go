package controller

import (
	"testing"

	"github.com/QuantumNous/the-one/setting/system_setting"
	"github.com/stretchr/testify/assert"
)

func TestIsChatImageBridgeModelRequiresEnabledAllowListedGoogleImageModel(t *testing.T) {
	settings := system_setting.ChatImageBridgeSetting{Enabled: true, Models: []string{"gemini-2.5-flash-image"}}

	assert.True(t, IsChatImageBridgeModel(settings, "gemini-2.5-flash-image"))
	assert.False(t, IsChatImageBridgeModel(settings, "gemini-2.5-flash"))
}
