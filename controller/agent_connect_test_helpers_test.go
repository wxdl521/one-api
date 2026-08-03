package controller

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/the-one/model"
	"github.com/stretchr/testify/require"
)

func agentConnectTestSessionID(userID int) string {
	return fmt.Sprintf("agent-connect-test-session-%d", userID)
}

func bindAgentConnectTestFreshLogin(t *testing.T, requestID string, userID int) {
	t.Helper()
	nonce, _, err := model.BeginAgentConnectReauthentication(requestID)
	require.NoError(t, err)
	require.NoError(t, model.CompleteAgentConnectReauthentication(requestID, nonce, agentConnectTestSessionID(userID)))
}
