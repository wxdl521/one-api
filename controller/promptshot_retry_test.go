package controller

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/the-one/model"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/QuantumNous/the-one/relaykit/types"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptShotChannelErrorStopsBeforeFallbackChannelSelection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set(promptShotContextKey, true)
	context.Set("specific_channel_id", "17")

	channelError := types.NewError(errors.New("selected channel failed"), types.ErrorCodeChannelInvalidKey)
	require.True(t, types.IsChannelError(channelError))

	fallbackSelections := 0
	originalSelector := cacheGetRandomSatisfiedChannel
	cacheGetRandomSatisfiedChannel = func(*service.RetryParam) (*model.Channel, string, error) {
		fallbackSelections++
		return &model.Channel{Id: 23}, "default", nil
	}
	t.Cleanup(func() { cacheGetRandomSatisfiedChannel = originalSelector })

	// This is the exact transition Relay takes after its first selected-channel
	// error: a retry would invoke getChannel with the already initialized meta.
	if shouldRetry(context, channelError, 2) {
		_, err := getChannel(context, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelId: 17}}, &service.RetryParam{Ctx: context})
		require.NoError(t, err)
	}

	assert.Zero(t, fallbackSelections, "PromptShot must not call CacheGetRandomSatisfiedChannel after its selected channel fails")
}

func TestShouldRetryKeepsOrdinaryChannelErrorRetries(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	channelError := types.NewError(errors.New("ordinary channel failed"), types.ErrorCodeChannelInvalidKey)

	assert.True(t, shouldRetry(context, channelError, 2))
}
