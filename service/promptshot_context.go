package service

import (
	"context"

	"github.com/gin-gonic/gin"
)

type promptShotContextKey struct{}

const PromptShotCompatContextKey = "promptshot_compat"

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
