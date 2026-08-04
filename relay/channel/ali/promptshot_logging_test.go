package ali

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/the-one/common"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func captureAliErrorLog(t *testing.T, run func()) string {
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

func TestAliImageHandlerDoesNotLogPromptShotUpstreamMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	context.Request = context.Request.WithContext(service.WithPromptShotRequestContext(context.Request.Context()))
	const upstreamMessage = "ali-upstream-image-payload-must-not-be-logged"

	logs := captureAliErrorLog(t, func() {
		_, _ = aliImageHandler(&Adaptor{}, context, &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewBufferString(`{"message":"` + upstreamMessage + `"}`)),
		}, &relaycommon.RelayInfo{})
	})

	assert.NotContains(t, logs, upstreamMessage)
}

func TestAliImageHandlerKeepsNormalUpstreamMessageLogging(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	const upstreamMessage = "ali-normal-upstream-message"

	logs := captureAliErrorLog(t, func() {
		_, _ = aliImageHandler(&Adaptor{}, context, &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewBufferString(`{"message":"` + upstreamMessage + `"}`)),
		}, &relaycommon.RelayInfo{})
	})

	assert.Contains(t, logs, upstreamMessage)
}

func TestAliImageHandlerDoesNotLogPromptShotSyncImageResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	context.Request = context.Request.WithContext(service.WithPromptShotRequestContext(context.Request.Context()))
	const upstreamImage = "ali-promptshot-upstream-response-must-not-be-logged"

	logs := captureAliErrorLog(t, func() {
		relayErr, _ := aliImageHandler(&Adaptor{IsSyncImageModel: true}, context, &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"output":{"results":[{"b64_image":"` + upstreamImage + `"}]}}`)),
		}, &relaycommon.RelayInfo{})
		assert.Nil(t, relayErr)
	})

	assert.NotContains(t, logs, upstreamImage)
}

func TestAliImageHandlerKeepsNormalSyncImageResponseDebugLogging(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", nil)
	const upstreamImage = "ali-normal-upstream-response"

	logs := captureAliErrorLog(t, func() {
		relayErr, _ := aliImageHandler(&Adaptor{IsSyncImageModel: true}, context, &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"output":{"results":[{"b64_image":"` + upstreamImage + `"}]}}`)),
		}, &relaycommon.RelayInfo{})
		assert.Nil(t, relayErr)
	})

	assert.Contains(t, logs, upstreamImage)
}
