package service

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func capturePromptShotDownloadLog(t *testing.T, run func()) string {
	t.Helper()
	var output bytes.Buffer
	common.LogWriterMu.Lock()
	originalWriter := gin.DefaultWriter
	gin.DefaultWriter = &output
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = originalWriter
		common.LogWriterMu.Unlock()
	})
	run()
	return output.String()
}

func disableSSRFProtectionForWorkerDownloadFixture(t *testing.T) {
	t.Helper()
	fetchSetting := system_setting.GetFetchSetting()
	originalFetchSetting := *fetchSetting
	fetchSetting.EnableSSRFProtection = false
	t.Cleanup(func() {
		*fetchSetting = originalFetchSetting
	})
}

func TestPromptShotWorkerDownloadDoesNotLogSignedImageURL(t *testing.T) {
	disableSSRFProtectionForWorkerDownloadFixture(t)
	InitHttpClient()
	worker := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
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
	const signedURL = "https://example.com/render.png?signature=promptshot-download-secret"

	logs := capturePromptShotDownloadLog(t, func() {
		response, err := DoDownloadRequestWithContext(WithPromptShotRequestContext(context.Background()), signedURL, "zhipu_image")
		require.NoError(t, err)
		response.Body.Close()
	})

	assert.NotContains(t, logs, signedURL)
	assert.NotContains(t, logs, "promptshot-download-secret")
}

func TestPromptShotOriginDownloadDoesNotLogSignedImageURL(t *testing.T) {
	disableSSRFProtectionForWorkerDownloadFixture(t)
	InitHttpClient()
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer origin.Close()
	originalWorkerURL := system_setting.WorkerUrl
	system_setting.WorkerUrl = ""
	t.Cleanup(func() {
		system_setting.WorkerUrl = originalWorkerURL
	})
	signedURL := origin.URL + "/render.png?signature=promptshot-origin-download-secret"

	logs := capturePromptShotDownloadLog(t, func() {
		response, err := DoDownloadRequestWithContext(WithPromptShotRequestContext(context.Background()), signedURL, "zhipu_image")
		require.NoError(t, err)
		response.Body.Close()
	})

	assert.NotContains(t, logs, signedURL)
	assert.NotContains(t, logs, "promptshot-origin-download-secret")
}

func TestNormalWorkerDownloadRetainsURLInDiagnosticLog(t *testing.T) {
	disableSSRFProtectionForWorkerDownloadFixture(t)
	InitHttpClient()
	worker := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
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
	const signedURL = "https://example.com/render.png?signature=normal-download-secret"

	logs := capturePromptShotDownloadLog(t, func() {
		response, err := DoDownloadRequest(signedURL, "diagnostic")
		require.NoError(t, err)
		response.Body.Close()
	})

	assert.Contains(t, logs, signedURL)
}
