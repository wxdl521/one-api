package openai

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	relaycommon "github.com/QuantumNous/the-one/relay/common"
	relayconstant "github.com/QuantumNous/the-one/relay/constant"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIAdaptorRejectsOversizedPromptShotResponseBeforeHandlerReadsIt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	releaseUpstream := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Content-Length", strconv.FormatInt(service.PromptShotMaxUpstreamResponseBytes+1, 10))
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("x"))
		writer.(http.Flusher).Flush()
		<-releaseUpstream
	}))
	t.Cleanup(func() {
		close(releaseUpstream)
		upstream.Close()
	})
	service.InitHttpClient()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	context.Request = context.Request.WithContext(service.WithPromptShotRequestContext(context.Request.Context()))
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: upstream.URL},
		RelayMode:   relayconstant.RelayModeImagesGenerations,
	}

	responseValue, err := (&Adaptor{}).DoRequest(context, info, nil)
	require.Error(t, err)
	assert.Nil(t, responseValue)
	assert.True(t, errors.Is(err, service.ErrPromptShotUpstreamResponseTooLarge))
}
