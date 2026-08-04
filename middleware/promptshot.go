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
