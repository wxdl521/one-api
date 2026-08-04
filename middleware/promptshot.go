package middleware

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/the-one/common"

	"github.com/gin-gonic/gin"
)

type promptShotBodyTokenRequest struct {
	Token string `json:"token"`
}

// PromptShotMaxRequestBytes leaves room for the JSON envelope around a 20 MiB
// decoded image while putting a hard cap on work before model selection.
const PromptShotMaxRequestBytes int64 = 30 << 20

const (
	promptShotPreflightRateMaxRequests   = 60
	promptShotPreflightRateWindowSeconds = 60
)

// PromptShotPreflight rejects bodies that cannot possibly be processed before
// authentication or JSON parsing. The inexpensive IP limiter deliberately runs
// before model selection because PromptShot must decode its reference image to
// discover the selected model.
func PromptShotPreflight() gin.HandlerFunc {
	return promptShotPreflightWithRateLimit(promptShotPreflightRateMaxRequests, promptShotPreflightRateWindowSeconds, "PSPF")
}

func promptShotPreflightWithRateLimit(maxRequests int, windowSeconds int64, mark string) gin.HandlerFunc {
	// The application may register routes before Redis has connected. Keep an
	// in-memory fallback initialized, while choosing Redis at request time once
	// it is available instead of permanently freezing the startup state.
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
	return func(c *gin.Context) {
		if c.Request.ContentLength > PromptShotMaxRequestBytes {
			promptShotAbort(c, http.StatusRequestEntityTooLarge, "请求体过大")
			return
		}
		if common.RedisEnabled && common.RDB != nil {
			redisRateLimiter(c, maxRequests, windowSeconds, mark)
		} else {
			memoryRateLimiter(c, maxRequests, windowSeconds, mark)
		}
		if c.IsAborted() {
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, PromptShotMaxRequestBytes)
		c.Next()
	}
}

// PromptShotBodyTokenAuth accepts the historical PromptShot validation
// credential shape and converts it to the normal relay Authorization header
// before TokenAuth executes. The credential is never put in a response or log.
func PromptShotBodyTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.GetHeader("Content-Type"), gin.MIMEJSON) {
			promptShotAbort(c, http.StatusBadRequest, "请求参数无效")
			return
		}

		var request promptShotBodyTokenRequest
		if err := common.UnmarshalBodyReusable(c, &request); err != nil || strings.TrimSpace(request.Token) == "" {
			if common.IsRequestBodyTooLargeError(err) {
				promptShotAbort(c, http.StatusRequestEntityTooLarge, "请求体过大")
				return
			}
			promptShotAbort(c, http.StatusBadRequest, "请求参数无效")
			return
		}

		c.Request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(request.Token))
		c.Next()
	}
}

func promptShotAbort(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error": gin.H{
			"message": message,
		},
	})
}
