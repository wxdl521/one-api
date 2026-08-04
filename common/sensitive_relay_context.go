package common

import (
	"context"
	"errors"
	"io"
)

type sensitiveRelayPayloadLoggingContextKey struct{}

const SensitiveRelayResponseBodyMaxBytes = 64 << 10

var ErrSensitiveRelayResponseBodyTooLarge = errors.New("sensitive relay response body too large")

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

// ReadRelayResponseBody preserves the ordinary relay behavior for public
// traffic, but bounds confidential internal relay responses before they can
// consume unbounded memory or reach diagnostics.
func ReadRelayResponseBody(ctx context.Context, reader io.Reader) ([]byte, error) {
	if !SensitiveRelayPayloadLoggingSuppressed(ctx) {
		return io.ReadAll(reader)
	}
	body, err := io.ReadAll(io.LimitReader(reader, SensitiveRelayResponseBodyMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > SensitiveRelayResponseBodyMaxBytes {
		return nil, ErrSensitiveRelayResponseBodyTooLarge
	}
	return body, nil
}
