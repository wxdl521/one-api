package system_setting

import (
	"slices"
	"strings"

	"github.com/QuantumNous/the-one/setting/config"
)

const (
	defaultChatImageBridgeWaitSeconds = 300
	defaultChatImageBridgeMediaTTL    = 24 * 60 * 60
	minChatImageBridgeMediaTTL        = 5 * 60
	maxChatImageBridgeMediaTTL        = 7 * 24 * 60 * 60
)

type ChatImageBridgeSetting struct {
	Enabled         bool     `json:"enabled"`
	Models          []string `json:"models"`
	MaxWaitSeconds  int      `json:"max_wait_seconds"`
	MediaTTLSeconds int      `json:"media_ttl_seconds"`
}

var chatImageBridgeSetting = ChatImageBridgeSetting{
	Models:          []string{},
	MaxWaitSeconds:  defaultChatImageBridgeWaitSeconds,
	MediaTTLSeconds: defaultChatImageBridgeMediaTTL,
}

func init() { config.GlobalConfig.Register("chat_image_bridge", &chatImageBridgeSetting) }

func (s ChatImageBridgeSetting) Normalized() ChatImageBridgeSetting {
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
	if s.MediaTTLSeconds < minChatImageBridgeMediaTTL {
		s.MediaTTLSeconds = minChatImageBridgeMediaTTL
	} else if s.MediaTTLSeconds > maxChatImageBridgeMediaTTL {
		s.MediaTTLSeconds = maxChatImageBridgeMediaTTL
	}
	return s
}

func GetChatImageBridgeSetting() ChatImageBridgeSetting { return chatImageBridgeSetting.Normalized() }
