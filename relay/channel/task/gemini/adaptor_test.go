package gemini

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoResponseSuppressesTaskResponseForChatBridge(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("chat_video_bridge_response_suppressed", true)
	response := &http.Response{
		Body: io.NopCloser(strings.NewReader(`{"name":"models/veo-3.1-generate-preview/operations/example"}`)),
	}

	taskID, _, taskErr := (&TaskAdaptor{}).DoResponse(context, response, &relaycommon.RelayInfo{
		OriginModelName: "veo-3.1-generate-preview",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task-1"},
	})

	require.Nil(t, taskErr)
	assert.NotEmpty(t, taskID)
	assert.False(t, recorder.Flushed)
	assert.Empty(t, recorder.Body.String())
}
