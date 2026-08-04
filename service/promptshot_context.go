package service

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

type promptShotContextKey struct{}
type promptShotResponseLimitContextKey struct{}

const PromptShotCompatContextKey = "promptshot_compat"

const PromptShotMaxUpstreamResponseBytes int64 = 30 << 20

var ErrPromptShotUpstreamResponseTooLarge = errors.New("promptshot upstream response exceeds maximum size")

// WithPromptShotRequestContext marks a request whose original images and
// provider responses must never be copied into diagnostic logs.
func WithPromptShotRequestContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, promptShotContextKey{}, true)
}

func IsPromptShotRequestContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	marked, _ := ctx.Value(promptShotContextKey{}).(bool)
	return marked
}

func promptShotResponseLimit(ctx context.Context) int64 {
	if ctx != nil {
		if limit, ok := ctx.Value(promptShotResponseLimitContextKey{}).(int64); ok && limit > 0 {
			return limit
		}
	}
	return PromptShotMaxUpstreamResponseBytes
}

// IsPromptShotCompatibleRequest accepts both the Gin marker used by relay
// middleware and the request-context marker used by transport/adaptor code.
// The latter is set before the request leaves PromptShot preparation, so
// adaptors can reliably suppress payload logging without importing controller.
func IsPromptShotCompatibleRequest(c *gin.Context) bool {
	if c == nil {
		return false
	}
	if c.GetBool(PromptShotCompatContextKey) {
		return true
	}
	return c.Request != nil && IsPromptShotRequestContext(c.Request.Context())
}

// LimitPromptShotHTTPResponse keeps provider responses bounded before an
// adaptor can call io.ReadAll. Callers must invoke it only for PromptShot
// requests; regular relay traffic retains its existing behavior.
func LimitPromptShotHTTPResponse(response *http.Response) error {
	return limitPromptShotHTTPResponse(response, PromptShotMaxUpstreamResponseBytes)
}

func limitPromptShotHTTPResponse(response *http.Response, maxBytes int64) error {
	if response == nil || response.Body == nil {
		return nil
	}
	if response.ContentLength > maxBytes {
		_ = response.Body.Close()
		return ErrPromptShotUpstreamResponseTooLarge
	}
	response.Body = &promptShotLimitedResponseBody{
		ReadCloser: response.Body,
		remaining:  maxBytes,
	}
	return nil
}

type promptShotLimitedResponseBody struct {
	io.ReadCloser
	remaining int64
}

func (body *promptShotLimitedResponseBody) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return body.ReadCloser.Read(buffer)
	}
	if body.remaining == 0 {
		var probe [1]byte
		count, err := body.ReadCloser.Read(probe[:])
		if count > 0 {
			return 0, ErrPromptShotUpstreamResponseTooLarge
		}
		return 0, err
	}
	if int64(len(buffer)) > body.remaining {
		buffer = buffer[:body.remaining]
	}
	count, err := body.ReadCloser.Read(buffer)
	body.remaining -= int64(count)
	return count, err
}
