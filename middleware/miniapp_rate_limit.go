package middleware

import "github.com/gin-gonic/gin"

const (
	MiniAppAnonymousRateLimitMark       = "MA"
	MiniAppAnonymousRateLimitMax        = 10
	MiniAppAnonymousRateLimitDuration   = 60
	MiniAppAuthenticatedRateLimitMark   = "MU"
	MiniAppAuthenticatedRateLimitMax    = 30
	MiniAppAuthenticatedRateLimitWindow = 60
)

// MiniAppAnonymousIPRateLimit limits code exchange and other unauthenticated
// Mini Program BFF calls by client IP before request parsing begins.
func MiniAppAnonymousIPRateLimit() func(c *gin.Context) {
	return rateLimitFactory(
		MiniAppAnonymousRateLimitMax,
		MiniAppAnonymousRateLimitDuration,
		MiniAppAnonymousRateLimitMark,
	)
}

// MiniAppAuthenticatedUserRateLimit limits Mini Program and browser binding
// operations by authenticated user. It must run after the auth middleware.
func MiniAppAuthenticatedUserRateLimit() func(c *gin.Context) {
	return userRateLimitFactory(
		MiniAppAuthenticatedRateLimitMax,
		MiniAppAuthenticatedRateLimitWindow,
		MiniAppAuthenticatedRateLimitMark,
	)
}
