package controller

import (
	"slices"
	"strings"

	"github.com/QuantumNous/the-one/common"
	openai "github.com/QuantumNous/the-one/relaykit/dto"
	"github.com/QuantumNous/the-one/setting/system_setting"
	"github.com/gin-gonic/gin"
)

var chatImageBridgeModels = []string{
	"gemini-2.5-flash-image",
	"gemini-3-pro-image-preview",
	"nano-banana-pro-preview",
	"gemini-3.1-flash-image-preview",
}

func IsChatImageBridgeModel(settings system_setting.ChatImageBridgeSetting, model string) bool {
	model = strings.TrimSpace(model)
	return settings.Enabled && slices.Contains(settings.Models, model) && slices.Contains(chatImageBridgeModels, model)
}

// PrepareChatImageBridge recognizes explicitly enabled Google image models. They
// intentionally retain the normal Gemini chat relay: it already converts OpenAI
// text/image_url content into Gemini inline data and converts generated images
// back to OpenAI-compatible Markdown image content.
func PrepareChatImageBridge() gin.HandlerFunc {
	return func(c *gin.Context) {
		settings := system_setting.GetChatImageBridgeSetting()
		if !settings.Enabled {
			return
		}
		request := &openai.GeneralOpenAIRequest{}
		if err := common.UnmarshalBodyReusable(c, request); err != nil || !IsChatImageBridgeModel(settings, request.Model) {
			return
		}
		c.Set("chat_image_bridge", true)
	}
}
