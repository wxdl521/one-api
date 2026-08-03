package model

import (
	"crypto/subtle"
	"errors"
	"time"

	"gorm.io/gorm"
)

// BeginAgentConnectReauthentication invalidates any previous fresh-login
// binding and returns a one-time nonce to be kept only in the browser's
// HttpOnly cookie. A subsequent login must present that cookie to bind its
// server-side session to this connection request.
func BeginAgentConnectReauthentication(requestID string) (string, *AgentConnectRequest, error) {
	if requestID == "" {
		return "", nil, ErrAgentConnectInvalid
	}
	nonce, err := newAgentConnectOpaqueValue()
	if err != nil {
		return "", nil, err
	}
	nonceHash := agentConnectValueHash("reauthentication-nonce", nonce)
	var request AgentConnectRequest
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).
			Where("request_hash = ?", agentConnectValueHash("request", requestID)).
			First(&request).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAgentConnectInvalid
			}
			return err
		}
		if err := validatePendingAgentConnectRequest(&request, time.Now()); err != nil {
			return err
		}
		if err := tx.Model(&AgentConnectRequest{}).
			Where("id = ?", request.Id).
			Updates(map[string]any{
				"reauthentication_nonce_hash":  nonceHash,
				"reauthenticated_session_hash": nil,
			}).Error; err != nil {
			return err
		}
		request.ReauthenticationNonceHash = &nonceHash
		request.ReauthenticatedSessionHash = nil
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	return nonce, &request, nil
}

// CompleteAgentConnectReauthentication binds the session created by the
// browser login to the pending connection request. The nonce is single-use.
func CompleteAgentConnectReauthentication(requestID string, nonce string, sessionID string) error {
	if requestID == "" || nonce == "" || sessionID == "" {
		return ErrAgentConnectInvalid
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var request AgentConnectRequest
		if err := lockForUpdate(tx).
			Where("request_hash = ?", agentConnectValueHash("request", requestID)).
			First(&request).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAgentConnectInvalid
			}
			return err
		}
		if err := validatePendingAgentConnectRequest(&request, time.Now()); err != nil {
			return err
		}
		if request.ReauthenticationNonceHash == nil || subtle.ConstantTimeCompare(
			[]byte(agentConnectValueHash("reauthentication-nonce", nonce)),
			[]byte(*request.ReauthenticationNonceHash),
		) != 1 {
			return ErrAgentConnectReauthenticationRequired
		}
		sessionHash := agentConnectValueHash("reauthentication-session", sessionID)
		return tx.Model(&AgentConnectRequest{}).
			Where("id = ?", request.Id).
			Updates(map[string]any{
				"reauthentication_nonce_hash":  nil,
				"reauthenticated_session_hash": sessionHash,
			}).Error
	})
}

// IsAgentConnectReauthenticatedSession reports whether the active server-side
// login session is the fresh login bound to this connection request.
func IsAgentConnectReauthenticatedSession(request *AgentConnectRequest, sessionID string) bool {
	if request == nil || request.ReauthenticatedSessionHash == nil || sessionID == "" {
		return false
	}
	return subtle.ConstantTimeCompare(
		[]byte(agentConnectValueHash("reauthentication-session", sessionID)),
		[]byte(*request.ReauthenticatedSessionHash),
	) == 1
}
