package middleware

import (
	"net/http"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
)

const miniAppSessionLoginMethod = "wechat-miniapp"

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
