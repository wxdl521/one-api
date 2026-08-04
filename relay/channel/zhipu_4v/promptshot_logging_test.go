package zhipu_4v

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/the-one/common"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/QuantumNous/the-one/service"
	"github.com/QuantumNous/the-one/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureZhipuPromptShotLogs(t *testing.T, run func()) string {
	t.Helper()
	var output bytes.Buffer
	common.LogWriterMu.Lock()
	originalWriter := gin.DefaultWriter
	originalErrorWriter := gin.DefaultErrorWriter
	gin.DefaultWriter = &output
	gin.DefaultErrorWriter = &output
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = originalWriter
		gin.DefaultErrorWriter = originalErrorWriter
		common.LogWriterMu.Unlock()
	})
	run()
	return output.String()
}

func TestZhipuPromptShotImageDownloadDoesNotLogSignedURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	fetchSetting := system_setting.GetFetchSetting()
	originalFetchSetting := *fetchSetting
	fetchSetting.EnableSSRFProtection = false
	t.Cleanup(func() {
		*fetchSetting = originalFetchSetting
	})
	worker := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer worker.Close()
	originalWorkerURL := system_setting.WorkerUrl
	originalWorkerKey := system_setting.WorkerValidKey
	system_setting.WorkerUrl = worker.URL
	system_setting.WorkerValidKey = "worker-key"
	t.Cleanup(func() {
		system_setting.WorkerUrl = originalWorkerURL
		system_setting.WorkerValidKey = originalWorkerKey
	})
	const signedURL = "https://example.com/render.png?signature=zhipu-promptshot-download-secret"
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	context.Request = context.Request.WithContext(service.WithPromptShotRequestContext(context.Request.Context()))

	logs := captureZhipuPromptShotLogs(t, func() {
		_, relayErr := zhipu4vImageHandler(context, &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"data":[{"url":"` + signedURL + `"}]}`)),
		}, &relaycommon.RelayInfo{})
		require.Nil(t, relayErr)
	})

	assert.NotContains(t, logs, signedURL)
	assert.NotContains(t, logs, "zhipu-promptshot-download-secret")
}
