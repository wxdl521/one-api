package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/the-one/setting/ratio_setting"
	"github.com/QuantumNous/the-one/setting/system_setting"
)

var (
	ErrPromptShotUnsupportedOperation          = errors.New("promptshot operation is unsupported")
	ErrPromptShotGroupUnavailable              = errors.New("promptshot group is unavailable")
	ErrPromptShotNoConfiguredModel             = errors.New("promptshot operation has no configured model")
	ErrPromptShotTokenModelForbidden           = errors.New("promptshot token cannot access configured models")
	ErrPromptShotCapabilityResolverUnavailable = errors.New("promptshot capability resolver is unavailable")
	ErrPromptShotCapabilityCheckFailed         = errors.New("promptshot capability check failed")
	ErrPromptShotNoAvailableCapability         = errors.New("promptshot has no available channel capability")
)

// PromptShotSelectionRequest contains only the authorization state required to
// make a server-side model selection. Its Group must be the effective, concrete
// token group; auto is rejected so this policy cannot silently cross groups.
type PromptShotSelectionRequest struct {
	Group                   string
	TokenModelLimitsEnabled bool
	TokenModelLimits        map[string]bool
}

// PromptShotChannelCapabilityResolver verifies that an administrator-verified
// channel capability is still usable for a specific group, model, and path.
type PromptShotChannelCapabilityResolver interface {
	IsAvailable(group, model, requestPath string, channelID int) (bool, error)
}

type PromptShotModelSelection struct {
	Group       string
	Model       string
	Operation   system_setting.PromptShotOperation
	RequestPath string
	ChannelID   int
}

// SelectPromptShotModel selects only configured candidates whose token access,
// group, and verified channel capability all match. It deliberately does not
// inspect the public model list: discovery is not a capability assertion.
func SelectPromptShotModel(
	request PromptShotSelectionRequest,
	operation system_setting.PromptShotOperation,
	policy system_setting.PromptShotSetting,
	resolver PromptShotChannelCapabilityResolver,
) (*PromptShotModelSelection, error) {
	operation = operation.Canonical()
	if operation == "" {
		return nil, ErrPromptShotUnsupportedOperation
	}

	group := strings.TrimSpace(request.Group)
	if group == "" || group == "auto" {
		return nil, ErrPromptShotGroupUnavailable
	}

	policy = policy.Normalized()
	candidates := policy.ModelsForOperation(operation)
	if len(candidates) == 0 {
		return nil, ErrPromptShotNoConfiguredModel
	}

	hasTokenAllowedCandidate := false
	for _, candidate := range candidates {
		if request.TokenModelLimitsEnabled {
			modelLimit := ratio_setting.FormatMatchingModelName(candidate)
			if !request.TokenModelLimits[modelLimit] {
				continue
			}
		}
		hasTokenAllowedCandidate = true

		for _, capability := range policy.Capabilities {
			if capability.Model != candidate || capability.Operation != operation {
				continue
			}
			if resolver == nil {
				return nil, ErrPromptShotCapabilityResolverUnavailable
			}
			available, err := resolver.IsAvailable(group, candidate, capability.RequestPath, capability.ChannelID)
			if err != nil {
				return nil, ErrPromptShotCapabilityCheckFailed
			}
			if !available {
				continue
			}
			return &PromptShotModelSelection{
				Group:       group,
				Model:       candidate,
				Operation:   operation,
				RequestPath: capability.RequestPath,
				ChannelID:   capability.ChannelID,
			}, nil
		}
	}

	if !hasTokenAllowedCandidate {
		return nil, ErrPromptShotTokenModelForbidden
	}
	return nil, ErrPromptShotNoAvailableCapability
}
