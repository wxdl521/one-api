package controller

import "github.com/gin-gonic/gin"

func setAgentConnectTestBrowserSession(context *gin.Context, userID int) {
	context.Set("session_id", agentConnectTestSessionID(userID))
	context.Set("auth_version", int64(1))
	context.Set("session_version", int64(1))
}
