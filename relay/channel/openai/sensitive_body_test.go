package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/the-one/common"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenaiHandlerBoundsSensitiveRelaySuccessBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).
		WithContext(common.WithSensitiveRelayPayloadLogging(t.Context()))
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(strings.Repeat("large-success-body", common.SensitiveRelayResponseBodyMaxBytes))),
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"}}

	_, err := OpenaiHandler(c, info, resp)

	require.ErrorIs(t, err, common.ErrSensitiveRelayResponseBodyTooLarge)
	require.Empty(t, recorder.Body.String())
}
