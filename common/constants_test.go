package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultCriticalRateLimitIsTwentyRequestsPerMinute(t *testing.T) {
	assert.Equal(t, 20, DefaultCriticalRateLimitNum)
	assert.Equal(t, 60, DefaultCriticalRateLimitDuration)
}
