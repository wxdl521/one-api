package logger

import (
	"bytes"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSensitiveRelayContextRedactsPayloadLogs(t *testing.T) {
	previousDebugEnabled := common.DebugEnabled
	common.DebugEnabled = true
	t.Cleanup(func() { common.DebugEnabled = previousDebugEnabled })

	var logBuffer bytes.Buffer
	common.LogWriterMu.Lock()
	previousErrorWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = previousErrorWriter
		common.LogWriterMu.Unlock()
	})

	ctx := common.WithSensitiveRelayPayloadLogging(t.Context())
	LogDebug(ctx, "request body: %s", "prompt-that-must-not-be-logged")
	LogError(ctx, "upstream response body: output-that-must-not-be-logged")

	assert.NotContains(t, logBuffer.String(), "prompt-that-must-not-be-logged")
	assert.NotContains(t, logBuffer.String(), "output-that-must-not-be-logged")
	assert.Contains(t, logBuffer.String(), "sensitive relay payload redacted")
}
