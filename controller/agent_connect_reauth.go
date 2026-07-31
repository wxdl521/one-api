package controller

import (
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/middleware"
	"github.com/QuantumNous/the-one/model"
	"github.com/gin-gonic/gin"
)

const agentConnectReauthenticationCookieName = "agent_connect_reauthentication"

// ForceAgentConnectReauthentication starts a fresh-login flow for a pending
// connection request. The nonce is HttpOnly and can only be completed by the
// next browser login on this origin.
func ForceAgentConnectReauthentication(c *gin.Context) {
	requestID := c.Param("request_id")
	request, err := model.GetAgentConnectRequest(requestID)
	if err != nil {
		writeAgentConnectError(c, err)
		return
	}
	if identity, ok := middleware.GetSessionAuthIdentity(c); ok &&
		model.IsAgentConnectReauthenticatedSession(request, identity.SessionID) {
		common.ApiSuccess(c, gin.H{"reauthentication_required": false})
		return
	}
	nonce, request, err := model.BeginAgentConnectReauthentication(requestID)
	if err != nil {
		writeAgentConnectError(c, err)
		return
	}
	writeAgentConnectReauthenticationCookie(c, requestID, nonce, request.ExpiresAt)
	common.ApiSuccess(c, gin.H{"reauthentication_required": true})
}

func requireAgentConnectFreshLogin(c *gin.Context, request *model.AgentConnectRequest) bool {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok || !model.IsAgentConnectReauthenticatedSession(request, identity.SessionID) {
		writeAgentConnectError(c, model.ErrAgentConnectReauthenticationRequired)
		return false
	}
	return true
}

func completeAgentConnectReauthentication(c *gin.Context, sessionID string) {
	value, err := c.Cookie(agentConnectReauthenticationCookieName)
	if err != nil || value == "" || sessionID == "" {
		return
	}
	clearAgentConnectReauthenticationCookie(c)
	requestID, nonce, ok := strings.Cut(value, ":")
	if !ok || requestID == "" || nonce == "" || strings.Contains(nonce, ":") {
		return
	}
	_ = model.CompleteAgentConnectReauthentication(requestID, nonce, sessionID)
}

func writeAgentConnectReauthenticationCookie(c *gin.Context, requestID string, nonce string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt) / time.Second)
	if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     agentConnectReauthenticationCookieName,
		Value:    requestID + ":" + nonce,
		Path:     "/",
		MaxAge:   maxAge,
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   common.SessionCookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearAgentConnectReauthenticationCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     agentConnectReauthenticationCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   common.SessionCookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}
