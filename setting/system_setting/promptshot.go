package system_setting

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/setting/config"
)

type PromptShotOperation string

const (
	PromptShotOperationReverse  PromptShotOperation = "reverse"
	PromptShotOperationGenerate PromptShotOperation = "generate"
	PromptShotOperationEdit     PromptShotOperation = "edit"
	PromptShotOperationClean    PromptShotOperation = "clean"
)

const (
	promptShotChatCompletionsPath = "/v1/chat/completions"
	promptShotResponsesPath       = "/v1/responses"
	promptShotImageGenerationPath = "/v1/images/generations"
	promptShotImageEditPath       = "/v1/images/edits"
)

// PromptShotChannelCapability is an administrator-verified channel capability.
// A model listed by /v1/models is intentionally not sufficient to create one.
type PromptShotChannelCapability struct {
	ChannelID   int                 `json:"channel_id"`
	Model       string              `json:"model"`
	Operation   PromptShotOperation `json:"operation"`
	RequestPath string              `json:"request_path"`
}

// PromptShotSetting controls the server-side model policy used by the
// PromptShot compatibility API. Clients never select these models or provide
// upstream credentials.
type PromptShotSetting struct {
	ReverseModels  []string                      `json:"reverse_models"`
	GenerateModels []string                      `json:"generate_models"`
	EditModels     []string                      `json:"edit_models"`
	Capabilities   []PromptShotChannelCapability `json:"capabilities"`
}

type promptShotConfig struct {
	snapshot atomic.Pointer[PromptShotSetting]
}

var promptShotConfigStore = newPromptShotConfig()

func init() {
	config.GlobalConfig.Register("promptshot", promptShotConfigStore)
}

func newPromptShotConfig() *promptShotConfig {
	store := &promptShotConfig{}
	setting := promptShotDefaultSetting()
	store.snapshot.Store(&setting)
	return store
}

func promptShotDefaultSetting() PromptShotSetting {
	return PromptShotSetting{
		ReverseModels:  []string{},
		GenerateModels: []string{},
		EditModels:     []string{},
		Capabilities:   []PromptShotChannelCapability{},
	}
}

// Snapshot returns an independent copy, so callers cannot mutate the
// configuration published to other requests.
func (store *promptShotConfig) Snapshot() PromptShotSetting {
	setting := store.snapshot.Load()
	if setting == nil {
		return promptShotDefaultSetting()
	}
	return setting.Normalized()
}

func (store *promptShotConfig) ConfigToMap() (map[string]string, error) {
	setting := store.Snapshot()
	values := map[string]any{
		"reverse_models":  setting.ReverseModels,
		"generate_models": setting.GenerateModels,
		"edit_models":     setting.EditModels,
		"capabilities":    setting.Capabilities,
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		encoded, err := common.Marshal(value)
		if err != nil {
			return nil, err
		}
		result[key] = string(encoded)
	}
	return result, nil
}

func (store *promptShotConfig) ValidateConfigUpdate(values map[string]string) error {
	_, err := promptShotSettingFromConfigMap(store.Snapshot(), values)
	return err
}

func (store *promptShotConfig) UpdateConfigFromMap(values map[string]string) error {
	setting, err := promptShotSettingFromConfigMap(store.Snapshot(), values)
	if err != nil {
		return err
	}
	store.snapshot.Store(&setting)
	return nil
}

func promptShotSettingFromConfigMap(current PromptShotSetting, values map[string]string) (PromptShotSetting, error) {
	setting := current
	for key, rawValue := range values {
		switch key {
		case "reverse_models", "generate_models", "edit_models":
			models, err := parsePromptShotModels(key, rawValue)
			if err != nil {
				return PromptShotSetting{}, err
			}
			switch key {
			case "reverse_models":
				setting.ReverseModels = models
			case "generate_models":
				setting.GenerateModels = models
			case "edit_models":
				setting.EditModels = models
			}
		case "capabilities":
			capabilities, err := parsePromptShotCapabilities(rawValue)
			if err != nil {
				return PromptShotSetting{}, err
			}
			setting.Capabilities = capabilities
		}
	}
	return setting.validateAndNormalize()
}

func parsePromptShotModels(key, rawValue string) ([]string, error) {
	if strings.TrimSpace(rawValue) == "" || strings.TrimSpace(rawValue) == "null" {
		return nil, fmt.Errorf("promptshot.%s must be a JSON array", key)
	}
	var models []string
	if err := common.UnmarshalJsonStr(rawValue, &models); err != nil {
		return nil, fmt.Errorf("promptshot.%s must be a JSON array: %w", key, err)
	}
	return models, nil
}

func parsePromptShotCapabilities(rawValue string) ([]PromptShotChannelCapability, error) {
	if strings.TrimSpace(rawValue) == "" || strings.TrimSpace(rawValue) == "null" {
		return nil, fmt.Errorf("promptshot.capabilities must be a JSON array")
	}
	var capabilities []PromptShotChannelCapability
	if err := common.UnmarshalJsonStr(rawValue, &capabilities); err != nil {
		return nil, fmt.Errorf("promptshot.capabilities must be a JSON array: %w", err)
	}
	return capabilities, nil
}

func (operation PromptShotOperation) Canonical() PromptShotOperation {
	switch PromptShotOperation(strings.ToLower(strings.TrimSpace(string(operation)))) {
	case PromptShotOperationReverse:
		return PromptShotOperationReverse
	case PromptShotOperationGenerate:
		return PromptShotOperationGenerate
	case PromptShotOperationEdit, PromptShotOperationClean:
		return PromptShotOperationEdit
	default:
		return ""
	}
}

func (operation PromptShotOperation) DefaultRequestPath() string {
	switch operation.Canonical() {
	case PromptShotOperationReverse:
		return promptShotChatCompletionsPath
	case PromptShotOperationGenerate:
		return promptShotImageGenerationPath
	case PromptShotOperationEdit:
		return promptShotImageEditPath
	default:
		return ""
	}
}

func (operation PromptShotOperation) SupportsRequestPath(requestPath string) bool {
	requestPath = strings.TrimSpace(requestPath)
	switch operation.Canonical() {
	case PromptShotOperationReverse:
		return requestPath == promptShotChatCompletionsPath || requestPath == promptShotResponsesPath
	case PromptShotOperationGenerate:
		return requestPath == promptShotImageGenerationPath
	case PromptShotOperationEdit:
		return requestPath == promptShotImageEditPath
	default:
		return false
	}
}

func (s PromptShotSetting) ModelsForOperation(operation PromptShotOperation) []string {
	var models []string
	switch operation.Canonical() {
	case PromptShotOperationReverse:
		models = s.ReverseModels
	case PromptShotOperationGenerate:
		models = s.GenerateModels
	case PromptShotOperationEdit:
		models = s.EditModels
	}
	return append([]string{}, models...)
}

func (s PromptShotSetting) Normalized() PromptShotSetting {
	normalized, _ := s.normalize(false)
	return normalized
}

func (s PromptShotSetting) validateAndNormalize() (PromptShotSetting, error) {
	return s.normalize(true)
}

func (s PromptShotSetting) normalize(strict bool) (PromptShotSetting, error) {
	s.ReverseModels = normalizePromptShotModels(s.ReverseModels)
	s.GenerateModels = normalizePromptShotModels(s.GenerateModels)
	s.EditModels = normalizePromptShotModels(s.EditModels)

	capabilities := make([]PromptShotChannelCapability, 0, len(s.Capabilities))
	seen := make(map[string]struct{}, len(s.Capabilities))
	for _, capability := range s.Capabilities {
		capability.Model = strings.TrimSpace(capability.Model)
		capability.Operation = capability.Operation.Canonical()
		capability.RequestPath = strings.TrimSpace(capability.RequestPath)
		if capability.ChannelID <= 0 || capability.Model == "" || capability.Operation == "" {
			if strict {
				return PromptShotSetting{}, fmt.Errorf("promptshot capability must include a positive channel_id, model, and operation")
			}
			continue
		}
		if capability.RequestPath == "" {
			capability.RequestPath = capability.Operation.DefaultRequestPath()
		}
		if !capability.Operation.SupportsRequestPath(capability.RequestPath) {
			if strict {
				return PromptShotSetting{}, fmt.Errorf("promptshot capability path is invalid for operation %s", capability.Operation)
			}
			continue
		}

		key := strings.Join([]string{
			strconv.Itoa(capability.ChannelID),
			capability.Model,
			string(capability.Operation),
			capability.RequestPath,
		}, "\x00")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		capabilities = append(capabilities, capability)
	}
	s.Capabilities = capabilities
	return s, nil
}

func GetPromptShotSetting() PromptShotSetting {
	return promptShotConfigStore.Snapshot()
}

func normalizePromptShotModels(models []string) []string {
	normalized := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, exists := seen[model]; exists {
			continue
		}
		seen[model] = struct{}{}
		normalized = append(normalized, model)
	}
	return normalized
}
