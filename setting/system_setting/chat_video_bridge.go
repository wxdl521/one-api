package system_setting

import (
	"slices"
	"strings"

	"github.com/QuantumNous/the-one/setting/config"
)

const (
	defaultChatVideoBridgeWaitSeconds      = 300
	maxChatVideoBridgeWaitSeconds          = 600
	defaultChatVideoBridgeTicketTTLSeconds = 24 * 60 * 60
	minChatVideoBridgeTicketTTLSeconds     = 5 * 60
	maxChatVideoBridgeTicketTTLSeconds     = 7 * 24 * 60 * 60
)

// ChatVideoBridgeSetting controls the opt-in adapter that presents supported
// async video models through the OpenAI chat completions endpoint.
type ChatVideoBridgeSetting struct {
	Enabled            bool     `json:"enabled"`
	Models             []string `json:"models"`
	MaxWaitSeconds     int      `json:"max_wait_seconds"`
	TaskPageTTLSeconds int      `json:"task_page_ttl_seconds"`
}

var chatVideoBridgeSetting = ChatVideoBridgeSetting{
	Models:             []string{},
	MaxWaitSeconds:     defaultChatVideoBridgeWaitSeconds,
	TaskPageTTLSeconds: defaultChatVideoBridgeTicketTTLSeconds,
}

func init() {
	config.GlobalConfig.Register("chat_video_bridge", &chatVideoBridgeSetting)
}

func (s ChatVideoBridgeSetting) Normalized() ChatVideoBridgeSetting {
	models := make([]string, 0, len(s.Models))
	for _, model := range s.Models {
		model = strings.TrimSpace(model)
		if model != "" && !slices.Contains(models, model) {
			models = append(models, model)
		}
	}
	s.Models = models

	if s.MaxWaitSeconds < 0 {
		s.MaxWaitSeconds = 0
	} else if s.MaxWaitSeconds > maxChatVideoBridgeWaitSeconds {
		s.MaxWaitSeconds = maxChatVideoBridgeWaitSeconds
	}
	if s.TaskPageTTLSeconds < minChatVideoBridgeTicketTTLSeconds {
		s.TaskPageTTLSeconds = minChatVideoBridgeTicketTTLSeconds
	} else if s.TaskPageTTLSeconds > maxChatVideoBridgeTicketTTLSeconds {
		s.TaskPageTTLSeconds = maxChatVideoBridgeTicketTTLSeconds
	}
	return s
}

func GetChatVideoBridgeSetting() ChatVideoBridgeSetting {
	return chatVideoBridgeSetting.Normalized()
}
