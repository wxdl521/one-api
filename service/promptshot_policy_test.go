package service

import (
	"testing"

	"github.com/QuantumNous/the-one/setting/system_setting"
	"github.com/stretchr/testify/require"
)

type promptShotCapabilityCall struct {
	group     string
	model     string
	path      string
	channelID int
}

type promptShotCapabilityResolverStub struct {
	available map[promptShotCapabilityCall]bool
	calls     []promptShotCapabilityCall
}

func (s *promptShotCapabilityResolverStub) IsAvailable(group, model, requestPath string, channelID int) (bool, error) {
	call := promptShotCapabilityCall{group: group, model: model, path: requestPath, channelID: channelID}
	s.calls = append(s.calls, call)
	return s.available[call], nil
}

func TestSelectPromptShotModelHonorsConfiguredCandidateOrder(t *testing.T) {
	policy := system_setting.PromptShotSetting{
		ReverseModels: []string{"vision-primary", "vision-fallback"},
		Capabilities: []system_setting.PromptShotChannelCapability{
			{ChannelID: 11, Model: "vision-primary", Operation: system_setting.PromptShotOperationReverse, RequestPath: "/v1/chat/completions"},
			{ChannelID: 22, Model: "vision-fallback", Operation: system_setting.PromptShotOperationReverse, RequestPath: "/v1/responses"},
		},
	}
	resolver := &promptShotCapabilityResolverStub{available: map[promptShotCapabilityCall]bool{
		{group: "member", model: "vision-fallback", path: "/v1/responses", channelID: 22}: true,
	}}

	selection, err := SelectPromptShotModel(PromptShotSelectionRequest{Group: "member"}, system_setting.PromptShotOperationReverse, policy, resolver)

	require.NoError(t, err)
	require.Equal(t, &PromptShotModelSelection{
		Group:       "member",
		Model:       "vision-fallback",
		Operation:   system_setting.PromptShotOperationReverse,
		RequestPath: "/v1/responses",
		ChannelID:   22,
	}, selection)
	require.Equal(t, []promptShotCapabilityCall{
		{group: "member", model: "vision-primary", path: "/v1/chat/completions", channelID: 11},
		{group: "member", model: "vision-fallback", path: "/v1/responses", channelID: 22},
	}, resolver.calls)
}

func TestSelectPromptShotModelRejectsEmptyCandidatePolicy(t *testing.T) {
	selection, err := SelectPromptShotModel(
		PromptShotSelectionRequest{Group: "member"},
		system_setting.PromptShotOperationEdit,
		system_setting.PromptShotSetting{},
		&promptShotCapabilityResolverStub{},
	)

	require.Nil(t, selection)
	require.ErrorIs(t, err, ErrPromptShotNoConfiguredModel)
}

func TestSelectPromptShotModelRejectsTokenModelWhitelistBypass(t *testing.T) {
	policy := system_setting.PromptShotSetting{
		EditModels: []string{"image-edit"},
		Capabilities: []system_setting.PromptShotChannelCapability{
			{ChannelID: 7, Model: "image-edit", Operation: system_setting.PromptShotOperationEdit},
		},
	}
	resolver := &promptShotCapabilityResolverStub{}

	selection, err := SelectPromptShotModel(PromptShotSelectionRequest{
		Group:                   "member",
		TokenModelLimitsEnabled: true,
		TokenModelLimits:        map[string]bool{"another-model": true},
	}, system_setting.PromptShotOperationEdit, policy, resolver)

	require.Nil(t, selection)
	require.ErrorIs(t, err, ErrPromptShotTokenModelForbidden)
	require.Empty(t, resolver.calls)
}

func TestSelectPromptShotModelDoesNotTreatEditCapabilityAsGenerationCapability(t *testing.T) {
	policy := system_setting.PromptShotSetting{
		GenerateModels: []string{"image-model"},
		Capabilities: []system_setting.PromptShotChannelCapability{
			{ChannelID: 9, Model: "image-model", Operation: system_setting.PromptShotOperationEdit},
		},
	}
	resolver := &promptShotCapabilityResolverStub{}

	selection, err := SelectPromptShotModel(
		PromptShotSelectionRequest{Group: "member"},
		system_setting.PromptShotOperationGenerate,
		policy,
		resolver,
	)

	require.Nil(t, selection)
	require.ErrorIs(t, err, ErrPromptShotNoAvailableCapability)
	require.Empty(t, resolver.calls)
}

func TestSelectPromptShotModelRejectsAutoGroupToPreventCrossGroupFallback(t *testing.T) {
	policy := system_setting.PromptShotSetting{
		GenerateModels: []string{"image-model"},
		Capabilities: []system_setting.PromptShotChannelCapability{
			{ChannelID: 9, Model: "image-model", Operation: system_setting.PromptShotOperationGenerate},
		},
	}
	resolver := &promptShotCapabilityResolverStub{}

	selection, err := SelectPromptShotModel(
		PromptShotSelectionRequest{Group: "auto"},
		system_setting.PromptShotOperationGenerate,
		policy,
		resolver,
	)

	require.Nil(t, selection)
	require.ErrorIs(t, err, ErrPromptShotGroupUnavailable)
	require.Empty(t, resolver.calls)
}
