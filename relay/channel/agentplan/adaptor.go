package agentplan

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	channelconstant "github.com/QuantumNous/the-one/constant"
	"github.com/QuantumNous/the-one/relay/channel"
	"github.com/QuantumNous/the-one/relay/channel/volcengine"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	relayconstant "github.com/QuantumNous/the-one/relay/constant"
	"github.com/QuantumNous/the-one/relaykit/dto"

	"github.com/gin-gonic/gin"
)

// Adaptor implements the fixed Ark Agent Plan gateway protocol. It shares the
// Ark-compatible request conversion and response handling from VolcEngine,
// while keeping the Agent Plan path and credential rules isolated here.
type Adaptor struct {
	volcengine.Adaptor
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeVolcEngineAgentPlan]

	switch info.RelayMode {
	case relayconstant.RelayModeChatCompletions:
		return baseURL + "/chat/completions", nil
	case relayconstant.RelayModeEmbeddings:
		return baseURL + "/embeddings", nil
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		return baseURL + "/images/generations", nil
	case relayconstant.RelayModeRerank:
		return baseURL + "/rerank", nil
	case relayconstant.RelayModeResponses:
		return baseURL + "/responses", nil
	case relayconstant.RelayModeResponsesCompact:
		return baseURL + "/responses/compact", nil
	default:
		return "", fmt.Errorf("unsupported Agent Plan relay mode: %d", info.RelayMode)
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	if strings.TrimSpace(info.ApiKey) == "" {
		return errors.New("Agent Plan API key is required")
	}

	channel.SetupApiRequestHeader(info, c, header)
	if info.RelayMode == relayconstant.RelayModeImagesEdits {
		header.Set("Content-Type", gin.MIMEJSON)
	}
	header.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertAudioRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("Agent Plan channel does not support audio relay")
}

func (a *Adaptor) ConvertRerankRequest(_ *gin.Context, _ int, request dto.RerankRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) GetChannelName() string {
	return "volcengine-agent-plan"
}
