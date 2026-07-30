package router

import (
	"net/http"

	agentintegrations "github.com/QuantumNous/the-one/agent-integrations"
	"github.com/gin-gonic/gin"
)

func SetSkillRouter(router *gin.Engine) {
	router.GET("/skills/myagents/SKILL.md", serveSkill(agentintegrations.MyAgentsPairingSkill))
	router.GET("/skills/myagents/the-one-gateway/SKILL.md", serveSkill(agentintegrations.MyAgentsGatewaySkill))
}

func serveSkill(content string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=300")
		c.Header("X-The-One-Skill-Version", agentintegrations.MyAgentsSkillVersion)
		c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(content))
	}
}
