package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatVideoTaskTicketBindsTaskAndOwner(t *testing.T) {
	previousSecret := common.SessionSecret
	common.SessionSecret = "chat-video-ticket-test-secret"
	t.Cleanup(func() { common.SessionSecret = previousSecret })

	ticket, err := IssueChatVideoTaskTicket("task_123", 42, time.Hour)
	require.NoError(t, err)

	claims, err := VerifyChatVideoTaskTicket(ticket, "task_123")
	require.NoError(t, err)
	assert.Equal(t, 42, claims.UserID)
	assert.Equal(t, "task_123", claims.TaskID)

	_, err = VerifyChatVideoTaskTicket(ticket, "task_other")
	assert.ErrorIs(t, err, ErrChatVideoTaskTicketInvalid)
}

func TestChatVideoTaskTicketRejectsTamperingAndExpiry(t *testing.T) {
	previousSecret := common.SessionSecret
	common.SessionSecret = "chat-video-ticket-test-secret"
	t.Cleanup(func() { common.SessionSecret = previousSecret })

	ticket, err := IssueChatVideoTaskTicket("task_123", 42, time.Hour)
	require.NoError(t, err)
	_, err = VerifyChatVideoTaskTicket(ticket+"x", "task_123")
	assert.ErrorIs(t, err, ErrChatVideoTaskTicketInvalid)

	expiredTicket, err := IssueChatVideoTaskTicket("task_123", 42, -time.Second)
	require.NoError(t, err)
	_, err = VerifyChatVideoTaskTicket(expiredTicket, "task_123")
	assert.ErrorIs(t, err, ErrChatVideoTaskTicketExpired)
}
