package controller

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/the-one/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompleteAgentConnectReauthenticationConsumesCookieNonce(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AgentConnectRequest{}))
	requestID, _, err := model.CreateAgentConnectRequest(model.AgentConnectRequestCreate{
		ClientKind:          "hermes-skill",
		CodeChallenge:       strings.Repeat("a", 43),
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	nonce, _, err := model.BeginAgentConnectReauthentication(requestID)
	require.NoError(t, err)

	context, _ := newAuthenticatedContext(t, http.MethodPost, "/api/user/login", nil, 0)
	context.Request.AddCookie(&http.Cookie{
		Name:  agentConnectReauthenticationCookieName,
		Value: requestID + ":" + nonce,
	})
	completeAgentConnectReauthentication(context, "new-browser-session")

	request, err := model.GetAgentConnectRequest(requestID)
	require.NoError(t, err)
	assert.True(t, model.IsAgentConnectReauthenticatedSession(request, "new-browser-session"))
	assert.NotEmpty(t, context.Writer.Header().Values("Set-Cookie"))
}
