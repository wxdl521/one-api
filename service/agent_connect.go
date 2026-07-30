package service

import (
	"sort"

	"github.com/QuantumNous/the-one/model"
)

// AgentConnectGroupOption is a group that a signed-in user can connect to a
// native agent, together with the models currently routable in that group.
type AgentConnectGroupOption struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Models      []string `json:"models"`
}

func GetAgentConnectGroupOptions(userGroup string) []AgentConnectGroupOption {
	usableGroups := GetUserUsableGroups(userGroup)
	options := make([]AgentConnectGroupOption, 0, len(usableGroups))
	for group, description := range usableGroups {
		// A connection key must be pinned to one model and one concrete group.
		if group == "auto" {
			continue
		}
		models := model.GetGroupEnabledModels(group)
		if len(models) == 0 {
			continue
		}
		sort.Strings(models)
		options = append(options, AgentConnectGroupOption{
			ID:          group,
			Description: description,
			Models:      models,
		})
	}
	sort.Slice(options, func(i int, j int) bool {
		return options[i].ID < options[j].ID
	})
	return options
}

func IsAgentConnectSelectionAllowed(userGroup string, group string, modelName string) bool {
	for _, option := range GetAgentConnectGroupOptions(userGroup) {
		if option.ID != group {
			continue
		}
		for _, candidate := range option.Models {
			if candidate == modelName {
				return true
			}
		}
		return false
	}
	return false
}
