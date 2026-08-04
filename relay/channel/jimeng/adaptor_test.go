package jimeng

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/QuantumNous/the-one/relaykit/dto"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdaptorDoRequestPropagatesRequestContextCancellation(t *testing.T) {
	service.InitHttpClient()
	upstreamStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(upstreamStarted)
		select {
		case <-r.Context().Done():
		case <-time.After(200 * time.Millisecond):
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(server.Close)

	requestContext, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil).WithContext(requestContext)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelBaseUrl: server.URL,
		ApiKey:         "access-key|secret-key",
		ChannelSetting: dto.ChannelSettings{},
	}}

	result := make(chan error, 1)
	go func() {
		_, err := (&Adaptor{}).DoRequest(ginContext, info, strings.NewReader(`{"prompt":"test"}`))
		result <- err
	}()
	<-upstreamStarted
	cancel()

	require.Error(t, <-result)
}

func TestAdaptorDoRequestSucceedsWithActiveRequestContext(t *testing.T) {
	service.InitHttpClient()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelBaseUrl: server.URL,
		ApiKey:         "access-key|secret-key",
		ChannelSetting: dto.ChannelSettings{},
	}}

	response, err := (&Adaptor{}).DoRequest(ginContext, info, strings.NewReader(`{"prompt":"test"}`))

	require.NoError(t, err)
	resp, ok := response.(*http.Response)
	require.True(t, ok)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}
