package controller

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/the-one/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentConnectOptionsRejectsPreexistingBrowserSession(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AgentConnectRequest{}))
	requestID, _, err := model.CreateAgentConnectRequest(model.AgentConnectRequestCreate{
		ClientKind:          "hermes-skill",
		CodeChallenge:       strings.Repeat("a", 43),
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)

	context, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/agent-connect/requests/"+requestID, nil, 42)
	context.Params = gin.Params{{Key: "request_id", Value: requestID}}
	context.Set("group", "default")
	context.Set("session_id", "old-browser-session")
	context.Set("auth_version", int64(1))
	context.Set("session_version", int64(1))
	GetAgentConnectRequestOptions(context)

	response := decodeAPIResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Contains(t, recorder.Body.String(), "Sign in again")
}

func TestForceAgentConnectReauthenticationKeepsOnlyBoundFreshSession(t *testing.T) {
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
	require.NoError(t, model.CompleteAgentConnectReauthentication(requestID, nonce, "fresh-login-session"))

	context, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/agent-connect/requests/"+requestID+"/reauthenticate", nil, 42)
	context.Params = gin.Params{{Key: "request_id", Value: requestID}}
	context.Set("session_id", "fresh-login-session")
	context.Set("auth_version", int64(1))
	context.Set("session_version", int64(1))
	ForceAgentConnectReauthentication(context)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success)
	assert.Contains(t, recorder.Body.String(), "reauthentication_required\":false")
}
