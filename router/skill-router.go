package router

import (
	"net/http"

	agentintegrations "github.com/QuantumNous/the-one/agent-integrations"
	"github.com/gin-gonic/gin"
)

func SetSkillRouter(router *gin.Engine) {
	router.GET("/skills/myagents/SKILL.md", serveSkill(agentintegrations.MyAgentsPairingSkill))
	router.GET("/skills/myagents/the-one-gateway/SKILL.md", serveSkill(agentintegrations.MyAgentsGatewaySkill))
	router.GET("/skills/myagents/the-one-gateway.zip", serveGatewaySkillArchive)
	router.GET("/skills/hermes/SKILL.md", serveSkill(agentintegrations.HermesPairingSkill))
	router.GET("/skills/hermes/the-one-gateway/SKILL.md", serveSkill(agentintegrations.HermesGatewaySkill))
}

func serveGatewaySkillArchive(c *gin.Context) {
	content, err := agentintegrations.MyAgentsGatewaySkillArchive()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Header("Cache-Control", "public, max-age=300")
	c.Header("Content-Disposition", "attachment; filename=the-one-gateway.zip")
	c.Header("X-The-One-Skill-Version", agentintegrations.MyAgentsSkillVersion)
	c.Data(http.StatusOK, "application/zip", content)
}

func serveSkill(content string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=300")
		c.Header("X-The-One-Skill-Version", agentintegrations.MyAgentsSkillVersion)
		c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(content))
	}
}
