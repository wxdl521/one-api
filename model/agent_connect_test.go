package model

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestValidateAgentConnectBootstrapAcceptsStrictLoopbackPKCE(t *testing.T) {
	challenge := strings.Repeat("a", 43)

	err := validateAgentConnectBootstrap(
		"myagents",
		"http://127.0.0.1:43127/callback",
		challenge,
		"S256",
		strings.Repeat("s", 32),
	)

	require.NoError(t, err)
}

func TestAgentConnectPairingBootstrapRequiresS256WithoutLoopbackFields(t *testing.T) {
	challenge := strings.Repeat("a", 43)

	err := validateAgentConnectBootstrap(
		"myagents-skill",
		"",
		challenge,
		"S256",
		"",
	)
	require.NoError(t, err)

	for _, input := range []AgentConnectRequestCreate{
		{
			ClientKind:          "myagents-skill",
			RedirectURI:         "http://127.0.0.1:43127/callback",
			CodeChallenge:       challenge,
			CodeChallengeMethod: "S256",
		},
		{
			ClientKind:          "myagents-skill",
			CodeChallenge:       challenge,
			CodeChallengeMethod: "plain",
		},
		{
			ClientKind:          "myagents-skill",
			CodeChallenge:       "short",
			CodeChallengeMethod: "S256",
		},
		{
			ClientKind:          "myagents-skill",
			CodeChallenge:       challenge,
			CodeChallengeMethod: "S256",
			State:               strings.Repeat("s", 32),
		},
	} {
		_, _, createErr := CreateAgentConnectRequest(input)
		assert.Error(t, createErr)
	}
}

func TestAgentConnectPairingExchangeWaitsForApprovalAndIssuesOneToken(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&AgentConnectRequest{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AgentConnectRequest{}).Error)

	verifier := strings.Repeat("v", 43)
	verifierHash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(verifierHash[:])
	requestID, request, err := CreateAgentConnectRequest(AgentConnectRequestCreate{
		ClientKind:          "myagents-skill",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	assert.True(t, request.PairingMode)

	_, err = ExchangeAgentConnectPairingRequest(requestID, verifier)
	require.ErrorIs(t, err, ErrAgentConnectNotAuthorized)

	_, err = AuthorizeAgentConnectRequest(requestID, 42, "default", "gpt-4.1-mini")
	require.NoError(t, err)
	_, err = ExchangeAgentConnectPairingRequest(requestID, strings.Repeat("x", 43))
	require.ErrorIs(t, err, ErrAgentConnectInvalidVerifier)

	token, err := ExchangeAgentConnectPairingRequest(requestID, verifier)
	require.NoError(t, err)
	assert.Equal(t, "default", token.Group)
	assert.Equal(t, "gpt-4.1-mini", token.ModelLimits)
	assert.True(t, token.UnlimitedQuota)
	assert.True(t, token.ModelLimitsEnabled)
	assert.InDelta(t, time.Now().Add(AgentConnectTokenLifetime).Unix(), token.ExpiredTime, 1)

	_, err = ExchangeAgentConnectPairingRequest(requestID, verifier)
	require.ErrorIs(t, err, ErrAgentConnectConsumed)
	var count int64
	require.NoError(t, DB.Model(&Token{}).Where("user_id = ?", 42).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestAgentConnectPairingCanceledOrExpiredRequestsCannotIssueTokens(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&AgentConnectRequest{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AgentConnectRequest{}).Error)

	verifier := strings.Repeat("v", 43)
	verifierHash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(verifierHash[:])
	canceledRequestID, _, err := CreateAgentConnectRequest(AgentConnectRequestCreate{
		ClientKind:          "myagents-skill",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	require.NoError(t, CancelAgentConnectRequest(canceledRequestID, 42))
	_, err = ExchangeAgentConnectPairingRequest(canceledRequestID, verifier)
	require.ErrorIs(t, err, ErrAgentConnectCanceled)

	expiredRequestID, request, err := CreateAgentConnectRequest(AgentConnectRequestCreate{
		ClientKind:          "myagents-skill",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	_, err = AuthorizeAgentConnectRequest(expiredRequestID, 42, "default", "gpt-4.1-mini")
	require.NoError(t, err)
	require.NoError(t, DB.Model(&AgentConnectRequest{}).Where("id = ?", request.Id).Update("expires_at", time.Now().Add(-time.Second)).Error)
	_, err = ExchangeAgentConnectPairingRequest(expiredRequestID, verifier)
	require.ErrorIs(t, err, ErrAgentConnectExpired)

	var count int64
	require.NoError(t, DB.Model(&Token{}).Where("user_id = ?", 42).Count(&count).Error)
	assert.Zero(t, count)
}

func TestAgentConnectExchangeIssuesOnlyOneLimitedNinetyDayToken(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&AgentConnectRequest{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AgentConnectRequest{}).Error)

	verifier := strings.Repeat("v", 43)
	verifierHash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(verifierHash[:])
	requestID, _, err := CreateAgentConnectRequest(AgentConnectRequestCreate{
		ClientKind:          "myagents",
		RedirectURI:         "http://127.0.0.1:43127/callback",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
		State:               strings.Repeat("s", 32),
	})
	require.NoError(t, err)

	authorizationCode, err := AuthorizeAgentConnectRequest(requestID, 42, "default", "gpt-4.1-mini")
	require.NoError(t, err)
	issuedAt := time.Now().Unix()

	token, err := ExchangeAgentConnectRequest(requestID, authorizationCode, verifier)
	require.NoError(t, err)
	assert.NotEmpty(t, token.Key)
	assert.Equal(t, 42, token.UserId)
	assert.Equal(t, common.TokenStatusEnabled, token.Status)
	assert.True(t, token.UnlimitedQuota)
	assert.True(t, token.ModelLimitsEnabled)
	assert.Equal(t, "gpt-4.1-mini", token.ModelLimits)
	assert.Equal(t, "default", token.Group)
	assert.GreaterOrEqual(t, token.ExpiredTime, issuedAt+int64(90*24*time.Hour/time.Second)-1)
	assert.LessOrEqual(t, token.ExpiredTime, issuedAt+int64(90*24*time.Hour/time.Second)+1)

	_, err = ExchangeAgentConnectRequest(requestID, authorizationCode, verifier)
	assert.ErrorIs(t, err, ErrAgentConnectConsumed)
	var count int64
	require.NoError(t, DB.Model(&Token{}).Where("user_id = ?", 42).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestAgentConnectExchangeRejectsWrongVerifierWithoutCreatingToken(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&AgentConnectRequest{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AgentConnectRequest{}).Error)

	verifier := strings.Repeat("v", 43)
	verifierHash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(verifierHash[:])
	requestID, _, err := CreateAgentConnectRequest(AgentConnectRequestCreate{
		ClientKind:          "myagents",
		RedirectURI:         "http://127.0.0.1:43127/callback",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
		State:               strings.Repeat("s", 32),
	})
	require.NoError(t, err)
	authorizationCode, err := AuthorizeAgentConnectRequest(requestID, 42, "default", "gpt-4.1-mini")
	require.NoError(t, err)

	_, err = ExchangeAgentConnectRequest(requestID, authorizationCode, strings.Repeat("x", 43))
	assert.ErrorIs(t, err, ErrAgentConnectInvalidVerifier)
	var count int64
	require.NoError(t, DB.Model(&Token{}).Where("user_id = ?", 42).Count(&count).Error)
	assert.Zero(t, count)

	_, err = ExchangeAgentConnectRequest(requestID, authorizationCode, verifier)
	require.NoError(t, err)
}

func TestAgentConnectCanceledOrExpiredRequestsCannotIssueTokens(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&AgentConnectRequest{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AgentConnectRequest{}).Error)

	verifier := strings.Repeat("v", 43)
	verifierHash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(verifierHash[:])
	canceledRequestID, _, err := CreateAgentConnectRequest(AgentConnectRequestCreate{
		ClientKind:          "myagents",
		RedirectURI:         "http://127.0.0.1:43127/callback",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
		State:               strings.Repeat("s", 32),
	})
	require.NoError(t, err)
	require.NoError(t, CancelAgentConnectRequest(canceledRequestID, 42))
	_, err = AuthorizeAgentConnectRequest(canceledRequestID, 42, "default", "gpt-4.1-mini")
	assert.ErrorIs(t, err, ErrAgentConnectCanceled)

	expiredRequestID, request, err := CreateAgentConnectRequest(AgentConnectRequestCreate{
		ClientKind:          "myagents",
		RedirectURI:         "http://127.0.0.1:43127/callback",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
		State:               strings.Repeat("s", 32),
	})
	require.NoError(t, err)
	authorizationCode, err := AuthorizeAgentConnectRequest(expiredRequestID, 42, "default", "gpt-4.1-mini")
	require.NoError(t, err)
	require.NoError(t, DB.Model(&AgentConnectRequest{}).Where("id = ?", request.Id).Update("expires_at", time.Now().Add(-time.Second)).Error)

	_, err = ExchangeAgentConnectRequest(expiredRequestID, authorizationCode, verifier)
	assert.ErrorIs(t, err, ErrAgentConnectExpired)
	var count int64
	require.NoError(t, DB.Model(&Token{}).Where("user_id = ?", 42).Count(&count).Error)
	assert.Zero(t, count)
}

func TestConcurrentAgentConnectExchangeIssuesOneToken(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&AgentConnectRequest{}))
	require.NoError(t, DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&AgentConnectRequest{}).Error)

	verifier := strings.Repeat("v", 43)
	verifierHash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(verifierHash[:])
	requestID, _, err := CreateAgentConnectRequest(AgentConnectRequestCreate{
		ClientKind:          "myagents",
		RedirectURI:         "http://127.0.0.1:43127/callback",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
		State:               strings.Repeat("s", 32),
	})
	require.NoError(t, err)
	authorizationCode, err := AuthorizeAgentConnectRequest(requestID, 42, "default", "gpt-4.1-mini")
	require.NoError(t, err)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var waitGroup sync.WaitGroup
	for range 2 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			_, exchangeErr := ExchangeAgentConnectRequest(requestID, authorizationCode, verifier)
			errs <- exchangeErr
		}()
	}
	close(start)
	waitGroup.Wait()
	close(errs)

	successes := 0
	for exchangeErr := range errs {
		if exchangeErr == nil {
			successes++
			continue
		}
		assert.ErrorIs(t, exchangeErr, ErrAgentConnectConsumed)
	}
	assert.Equal(t, 1, successes)
	var count int64
	require.NoError(t, DB.Model(&Token{}).Where("user_id = ?", 42).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestValidateAgentConnectBootstrapRejectsUnsafeCallbacksAndPKCE(t *testing.T) {
	challenge := strings.Repeat("a", 43)
	testCases := []struct {
		name        string
		redirectURI string
		challenge   string
		method      string
	}{
		{
			name:        "remote callback",
			redirectURI: "http://example.com/callback",
			challenge:   challenge,
			method:      "S256",
		},
		{
			name:        "loopback callback with query",
			redirectURI: "http://127.0.0.1:43127/callback?next=1",
			challenge:   challenge,
			method:      "S256",
		},
		{
			name:        "plain PKCE",
			redirectURI: "http://127.0.0.1:43127/callback",
			challenge:   challenge,
			method:      "plain",
		},
		{
			name:        "short PKCE challenge",
			redirectURI: "http://127.0.0.1:43127/callback",
			challenge:   "short",
			method:      "S256",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := validateAgentConnectBootstrap(
				"myagents",
				testCase.redirectURI,
				testCase.challenge,
				testCase.method,
				strings.Repeat("s", 32),
			)

			assert.Error(t, err)
		})
	}
}
