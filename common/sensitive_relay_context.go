package common

import "context"

type sensitiveRelayPayloadLoggingContextKey struct{}

// WithSensitiveRelayPayloadLogging marks an internal relay call whose request
// and response bodies must not reach debug or error logs.
func WithSensitiveRelayPayloadLogging(ctx context.Context) context.Context {
	return context.WithValue(ctx, sensitiveRelayPayloadLoggingContextKey{}, true)
}

// SensitiveRelayPayloadLoggingSuppressed reports whether relay payload logging
// must be suppressed for this request context.
func SensitiveRelayPayloadLoggingSuppressed(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	suppressed, _ := ctx.Value(sensitiveRelayPayloadLoggingContextKey{}).(bool)
	return suppressed
}
