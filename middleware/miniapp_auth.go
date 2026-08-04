package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/constant"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
)

const miniAppSessionLoginMethod = "wechat-miniapp"

const miniAppBindingRequestBodyMaxBytes = 8 << 10

const miniAppTokenRequestBodyMaxBytes = 8 << 10

// A 4,000-rune JSON string can use 48 KiB when non-BMP runes are represented
// as surrogate-pair escapes, so leave room for the fixed envelope as well.
const miniAppTextTestRequestBodyMaxBytes = 64 << 10

// MiniAppFeatureGate rejects disabled or incomplete Mini Program deployments
// before rate limits, body parsing, bot checks, or authentication run.
func MiniAppFeatureGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := service.RequireMiniProgramConfig(); err != nil {
			writeMiniAppAuthError(c, err)
			return
		}
		c.Next()
	}
}

// MiniAppTextTestFeatureGate keeps the unreviewed text-test capability off by
// default, independently of the rest of the Mini Program BFF.
func MiniAppTextTestFeatureGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := service.RequireMiniProgramTextTestConfig(); err != nil {
			writeMiniAppAuthError(c, err)
			return
		}
		c.Next()
	}
}

// MiniAppAuth accepts only a live dashboard JWT backed by a Mini Program
// session. It deliberately does not fall back to personal access tokens or
// ordinary browser login sessions.
func MiniAppAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := service.RequireMiniProgramConfig(); err != nil {
			writeMiniAppAuthError(c, err)
			return
		}

		raw, ok := authorizationToken(c.GetHeader("Authorization"))
		if !ok {
			writeMiniAppSessionInvalid(c)
			return
		}
		identity, internal, err := service.ParseDashboardAccessToken(raw)
		if !internal || err != nil {
			writeMiniAppSessionInvalid(c)
			return
		}
		session, user, err := service.ValidateLoginSession(identity)
		if err != nil || session.LoginMethod != miniAppSessionLoginMethod || user.Status != common.UserStatusEnabled {
			writeMiniAppSessionInvalid(c)
			return
		}

		setDashboardAuthContext(c, user, identity, false)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, user.Group)
		c.Next()
	}
}

func writeMiniAppAuthError(c *gin.Context, err error) {
	status, code := service.MiniAppAuthErrorCode(err)
	if code == "MINIAPP_INTERNAL_ERROR" {
		common.SysError("mini program authentication failed: " + err.Error())
	}
	c.AbortWithStatusJSON(status, gin.H{
		"success": false,
		"code":    code,
		"message": http.StatusText(status),
	})
}

func writeMiniAppSessionInvalid(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"success": false,
		"code":    "MINIAPP_SESSION_INVALID",
		"message": http.StatusText(http.StatusUnauthorized),
	})
}

// MiniAppBindingRequestBodyLimit bounds the browser confirmation payload
// before the controller decodes it. The endpoint accepts only one short
// opaque ticket, so an 8 KiB cap is intentionally narrow.
func MiniAppBindingRequestBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body == nil {
			c.Next()
			return
		}
		if c.Request.ContentLength > miniAppBindingRequestBodyMaxBytes {
			writeMiniAppRequestTooLarge(c)
			return
		}

		originalBody := c.Request.Body
		body, err := io.ReadAll(io.LimitReader(originalBody, miniAppBindingRequestBodyMaxBytes+1))
		_ = originalBody.Close()
		if err != nil {
			writeMiniAppRequestTooLarge(c)
			return
		}
		if len(body) > miniAppBindingRequestBodyMaxBytes {
			writeMiniAppRequestTooLarge(c)
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Request.ContentLength = int64(len(body))
		c.Next()
	}
}

// MiniAppTokenRequestBodyLimit bounds the small token lifecycle payloads
// before controllers parse JSON. It accepts unknown-length (chunked) bodies
// only up to the same fixed limit.
func MiniAppTokenRequestBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body == nil {
			c.Next()
			return
		}
		if c.Request.ContentLength > miniAppTokenRequestBodyMaxBytes {
			writeMiniAppRequestTooLarge(c)
			return
		}

		originalBody := c.Request.Body
		body, err := io.ReadAll(io.LimitReader(originalBody, miniAppTokenRequestBodyMaxBytes+1))
		_ = originalBody.Close()
		if err != nil || len(body) > miniAppTokenRequestBodyMaxBytes {
			writeMiniAppRequestTooLarge(c)
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Request.ContentLength = int64(len(body))
		c.Next()
	}
}

// MiniAppTextTestRequestBodyLimit accepts the largest valid encoded 4,000-rune
// prompt while rejecting files or oversized payloads before a handler can
// decode them.
func MiniAppTextTestRequestBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body == nil {
			c.Next()
			return
		}
		if c.Request.ContentLength > miniAppTextTestRequestBodyMaxBytes {
			writeMiniAppRequestTooLarge(c)
			return
		}
		originalBody := c.Request.Body
		body, err := io.ReadAll(io.LimitReader(originalBody, miniAppTextTestRequestBodyMaxBytes+1))
		_ = originalBody.Close()
		if err != nil || len(body) > miniAppTextTestRequestBodyMaxBytes {
			writeMiniAppRequestTooLarge(c)
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Request.ContentLength = int64(len(body))
		c.Next()
	}
}

func writeMiniAppRequestTooLarge(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
		"success": false,
		"code":    "MINIAPP_REQUEST_TOO_LARGE",
		"message": http.StatusText(http.StatusRequestEntityTooLarge),
	})
}
