package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSensitiveRelayPayloadLoggingContext(t *testing.T) {
	assert.False(t, SensitiveRelayPayloadLoggingSuppressed(context.Background()))
	assert.True(t, SensitiveRelayPayloadLoggingSuppressed(WithSensitiveRelayPayloadLogging(context.Background())))
}
