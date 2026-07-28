package model

import (
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPricingChannelGroupsExposeOnlyAccessibleEnabledChannels(t *testing.T) {
	resetPricingEndpointTestTables(t)

	priority := int64(10)
	require.NoError(t, DB.Create(&[]Channel{
		{Id: 1, Type: constant.ChannelTypeOpenAI, Key: "key-1", Status: common.ChannelStatusEnabled, Name: "Volcengine", Priority: &priority},
		{Id: 2, Type: constant.ChannelTypeOpenAI, Key: "key-2", Status: common.ChannelStatusEnabled, Name: "Volcengine", Priority: &priority},
		{Id: 3, Type: constant.ChannelTypeOpenAI, Key: "key-3", Status: common.ChannelStatusEnabled, Name: "Mobile"},
		{Id: 4, Type: constant.ChannelTypeOpenAI, Key: "key-4", Status: 2, Name: "Disabled"},
	}).Error)
	require.NoError(t, DB.Create(&[]Ability{
		{Group: "default", Model: "shared-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "volcengine-only", ChannelId: 2, Enabled: true},
		{Group: "vip", Model: "mobile-vip-only", ChannelId: 3, Enabled: true},
		{Group: "default", Model: "disabled-channel-model", ChannelId: 4, Enabled: true},
		{Group: "default", Model: "disabled-ability-model", ChannelId: 1, Enabled: false},
	}).Error)

	visibleModels := GetPricing()
	groups := GetPricingChannelGroups(visibleModels, map[string]string{"default": "Default"})

	assert.Equal(t, []PricingChannelGroup{
		{Name: "Volcengine", Models: []string{"shared-model", "volcengine-only"}},
	}, groups)
}
