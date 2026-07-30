package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// MCPBearerAuth narrows the generic token authenticator to the Bearer header
// format required by remote Streamable HTTP MCP clients.
func MCPBearerAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Bearer authentication is required",
			})
			return
		}
		c.Next()
	}
}
