package openai

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/the-one/common"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureOpenAIDebugLog(t *testing.T, run func()) string {
	t.Helper()
	originalDebugEnabled := common.DebugEnabled
	common.DebugEnabled = true
	t.Cleanup(func() { common.DebugEnabled = originalDebugEnabled })

	var output bytes.Buffer
	common.LogWriterMu.Lock()
	originalWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &output
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = originalWriter
		common.LogWriterMu.Unlock()
	})

	run()
	return output.String()
}

func TestOpenAIAdaptorDoesNotLogPromptShotUpstreamImagePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	context.Request = context.Request.WithContext(service.WithPromptShotRequestContext(context.Request.Context()))

	const imagePayload = "promptshot-upstream-b64-json-must-not-be-logged"
	response := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl-test","object":"chat.completion","model":"vision","choices":[{"index":0,"message":{"role":"assistant","content":"reverse content"},"finish_reason":"stop"}],"b64_json":"` + imagePayload + `"}`)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	logs := captureOpenAIDebugLog(t, func() {
		_, relayErr := OpenaiHandler(context, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}, response)
		require.Nil(t, relayErr)
	})

	assert.NotContains(t, logs, imagePayload)
	assert.NotContains(t, logs, "reverse content")
}

func TestOpenAIAdaptorKeepsNormalDebugResponseLogging(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	const normalContent = "normal-upstream-debug-content"
	response := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl-test","object":"chat.completion","model":"vision","choices":[{"index":0,"message":{"role":"assistant","content":"` + normalContent + `"},"finish_reason":"stop"}]}`)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	logs := captureOpenAIDebugLog(t, func() {
		_, relayErr := OpenaiHandler(context, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}, response)
		require.Nil(t, relayErr)
	})

	assert.Contains(t, logs, normalContent)
}
