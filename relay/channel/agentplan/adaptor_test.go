package agentplan

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/the-one/constant"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	relayconstant "github.com/QuantumNous/the-one/relay/constant"
	"github.com/QuantumNous/the-one/relaykit/dto"
	"github.com/QuantumNous/the-one/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptorUsesAgentPlanFixedRoutesAndBearerAuthentication(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType:    constant.ChannelTypeVolcEngineAgentPlan,
		ChannelBaseUrl: "https://ignored.example",
		ApiKey:         "agent-plan-api-key",
	}}

	tests := []struct {
		name string
		mode int
		want string
	}{
		{name: "chat", mode: relayconstant.RelayModeChatCompletions, want: "/chat/completions"},
		{name: "responses", mode: relayconstant.RelayModeResponses, want: "/responses"},
		{name: "responses compact", mode: relayconstant.RelayModeResponsesCompact, want: "/responses/compact"},
		{name: "embeddings", mode: relayconstant.RelayModeEmbeddings, want: "/embeddings"},
		{name: "images generations", mode: relayconstant.RelayModeImagesGenerations, want: "/images/generations"},
		{name: "images edits", mode: relayconstant.RelayModeImagesEdits, want: "/images/generations"},
		{name: "rerank", mode: relayconstant.RelayModeRerank, want: "/rerank"},
	}

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info.RelayMode = tt.mode
			info.RelayFormat = types.RelayFormatOpenAI

			url, err := adaptor.GetRequestURL(info)
			require.NoError(t, err)
			assert.Equal(t, "https://ark.cn-beijing.volces.com/api/plan/v3"+tt.want, url)

			headers := http.Header{}
			require.NoError(t, adaptor.SetupRequestHeader(ctx, &headers, info))
			assert.Equal(t, "Bearer agent-plan-api-key", headers.Get("Authorization"))
		})
	}
}

func TestAdaptorRejectsModelRequestsWithoutAnAPIKey(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType: constant.ChannelTypeVolcEngineAgentPlan,
	}}

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	headers := http.Header{}

	require.ErrorContains(t, adaptor.SetupRequestHeader(ctx, &headers, info), "API key is required")
}

func TestAdaptorPreservesRerankRequest(t *testing.T) {
	adaptor := &Adaptor{}
	request := dto.RerankRequest{Model: "rerank-model", Query: "test", Documents: []any{"document"}}

	converted, err := adaptor.ConvertRerankRequest(nil, relayconstant.RelayModeRerank, request)

	require.NoError(t, err)
	assert.Equal(t, request, converted)
}
