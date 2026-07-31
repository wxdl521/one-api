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

func TestForceAgentConnectReauthenticationStartsOpaqueFreshLoginFlow(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AgentConnectRequest{}))
	requestID, _, err := model.CreateAgentConnectRequest(model.AgentConnectRequestCreate{
		ClientKind:          "hermes-skill",
		CodeChallenge:       strings.Repeat("a", 43),
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)

	context, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/agent-connect/requests/"+requestID+"/reauthenticate", nil, 0)
	context.Params = gin.Params{{Key: "request_id", Value: requestID}}
	ForceAgentConnectReauthentication(context)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success)
	assert.Contains(t, recorder.Body.String(), "reauthentication_required\":true")
	request, err := model.GetAgentConnectRequest(requestID)
	require.NoError(t, err)
	assert.NotNil(t, request.ReauthenticationNonceHash)
	assert.Nil(t, request.ReauthenticatedSessionHash)
	assert.Contains(t, recorder.Header().Get("Set-Cookie"), "HttpOnly")
	assert.NotContains(t, recorder.Body.String(), "agent_connect_reauthentication")
}
