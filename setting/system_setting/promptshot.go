package system_setting

import (
	"strconv"
	"strings"

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

var promptShotSetting = PromptShotSetting{
	ReverseModels:  []string{},
	GenerateModels: []string{},
	EditModels:     []string{},
	Capabilities:   []PromptShotChannelCapability{},
}

func init() {
	config.GlobalConfig.Register("promptshot", &promptShotSetting)
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
			continue
		}
		if capability.RequestPath == "" {
			capability.RequestPath = capability.Operation.DefaultRequestPath()
		}
		if !capability.Operation.SupportsRequestPath(capability.RequestPath) {
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
	return s
}

func GetPromptShotSetting() PromptShotSetting {
	return promptShotSetting.Normalized()
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
