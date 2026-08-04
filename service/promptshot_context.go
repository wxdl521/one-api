package service

import "context"

type promptShotContextKey struct{}

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
