package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/the-one/common"
)

const chatVideoTaskTicketPurpose = "chat-video-task-page"

var (
	ErrChatVideoTaskTicketInvalid = errors.New("chat video task ticket is invalid")
	ErrChatVideoTaskTicketExpired = errors.New("chat video task ticket has expired")
)

type ChatVideoTaskTicketClaims struct {
	TaskID    string `json:"task_id"`
	UserID    int    `json:"user_id"`
	Purpose   string `json:"purpose"`
	IssuedAt  int64  `json:"issued_at"`
	ExpiresAt int64  `json:"expires_at"`
}

func chatVideoTaskTicketKey() []byte {
	mac := hmac.New(sha256.New, []byte(common.SessionSecret))
	_, _ = mac.Write([]byte("the-one/chat-video-task-ticket/v1"))
	return mac.Sum(nil)
}

func IssueChatVideoTaskTicket(taskID string, userID int, ttl time.Duration) (string, error) {
	if strings.TrimSpace(taskID) == "" || userID <= 0 {
		return "", ErrChatVideoTaskTicketInvalid
	}
	now := time.Now()
	claims := ChatVideoTaskTicketClaims{
		TaskID:    taskID,
		UserID:    userID,
		Purpose:   chatVideoTaskTicketPurpose,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	}
	payload, err := common.Marshal(claims)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, chatVideoTaskTicketKey())
	_, _ = mac.Write([]byte(encodedPayload))
	return encodedPayload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func VerifyChatVideoTaskTicket(raw string, taskID string) (ChatVideoTaskTicketClaims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 || strings.TrimSpace(taskID) == "" {
		return ChatVideoTaskTicketClaims{}, ErrChatVideoTaskTicketInvalid
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ChatVideoTaskTicketClaims{}, ErrChatVideoTaskTicketInvalid
	}
	mac := hmac.New(sha256.New, chatVideoTaskTicketKey())
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return ChatVideoTaskTicketClaims{}, ErrChatVideoTaskTicketInvalid
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ChatVideoTaskTicketClaims{}, ErrChatVideoTaskTicketInvalid
	}
	var claims ChatVideoTaskTicketClaims
	if err := common.Unmarshal(payload, &claims); err != nil {
		return ChatVideoTaskTicketClaims{}, ErrChatVideoTaskTicketInvalid
	}
	if claims.ExpiresAt <= time.Now().Unix() {
		return ChatVideoTaskTicketClaims{}, ErrChatVideoTaskTicketExpired
	}
	if claims.UserID <= 0 || claims.Purpose != chatVideoTaskTicketPurpose || !hmac.Equal([]byte(claims.TaskID), []byte(taskID)) {
		return ChatVideoTaskTicketClaims{}, ErrChatVideoTaskTicketInvalid
	}
	return claims, nil
}
