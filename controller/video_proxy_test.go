package controller

import (
	"testing"

	"github.com/QuantumNous/the-one/constant"
	"github.com/stretchr/testify/assert"
)

func TestTrustedAICCVideoURL(t *testing.T) {
	validURL := "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/doubao-seedance-2-0/video.mp4?X-Tos-Algorithm=TOS4-HMAC-SHA256&X-Tos-Signature=signature"

	assert.True(t, isTrustedAICCVideoURL(constant.ChannelTypeAICCSeedance, validURL))
	assert.False(t, isTrustedAICCVideoURL(constant.ChannelTypeDoubaoVideo, validURL))
	assert.False(t, isTrustedAICCVideoURL(constant.ChannelTypeAICCSeedance, "http://ark-acg-cn-beijing.tos-cn-beijing.volces.com/doubao-seedance-2-0/video.mp4"))
	assert.False(t, isTrustedAICCVideoURL(constant.ChannelTypeAICCSeedance, "https://example.com/doubao-seedance-2-0/video.mp4"))
	assert.False(t, isTrustedAICCVideoURL(constant.ChannelTypeAICCSeedance, "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/other-model/video.mp4"))
}
