package model

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/setting/operation_setting"
	"gorm.io/gorm"
)

const (
	agentConnectClientMyAgents      = "myagents"
	agentConnectClientMyAgentsSkill = "myagents-skill"
	agentConnectPKCES256            = "S256"

	AgentConnectRequestLifetime = 10 * time.Minute
	AgentConnectTokenLifetime   = 90 * 24 * time.Hour

	agentConnectStatusPending    = "pending"
	agentConnectStatusAuthorized = "authorized"
	agentConnectStatusConsumed   = "consumed"
	agentConnectStatusCanceled   = "canceled"
)

var (
	ErrAgentConnectInvalid         = errors.New("agent connect request is invalid")
	ErrAgentConnectExpired         = errors.New("agent connect request has expired")
	ErrAgentConnectCanceled        = errors.New("agent connect request was canceled")
	ErrAgentConnectConsumed        = errors.New("agent connect request was already consumed")
	ErrAgentConnectNotAuthorized   = errors.New("agent connect request is not authorized")
	ErrAgentConnectInvalidVerifier = errors.New("agent connect PKCE verifier is invalid")
	ErrAgentConnectTokenLimit      = errors.New("agent connect token limit reached")
)

// AgentConnectRequest stores the server-side state for a short-lived native
// agent authorization. The externally visible request ID and authorization
// code are HMACed before storage, so neither credential can be replayed from a
// database dump.
type AgentConnectRequest struct {
	Id                    int64      `json:"id" gorm:"primaryKey"`
	RequestHash           string     `json:"-" gorm:"type:char(64);not null;uniqueIndex"`
	ClientKind            string     `json:"client_kind" gorm:"type:varchar(32);not null"`
	PairingMode           bool       `json:"pairing_mode"`
	RedirectURI           string     `json:"redirect_uri" gorm:"type:varchar(512);not null"`
	State                 string     `json:"state" gorm:"type:varchar(128);not null"`
	CodeChallenge         string     `json:"-" gorm:"type:varchar(128);not null"`
	UserId                int        `json:"user_id,omitempty" gorm:"index"`
	Group                 string     `json:"group,omitempty" gorm:"type:varchar(128)"`
	Model                 string     `json:"model,omitempty" gorm:"type:varchar(255)"`
	AuthorizationCodeHash *string    `json:"-" gorm:"type:char(64);uniqueIndex"`
	Status                string     `json:"status" gorm:"type:varchar(16);not null;index"`
	TokenId               *int       `json:"token_id,omitempty" gorm:"uniqueIndex"`
	CreatedAt             time.Time  `json:"created_at"`
	ExpiresAt             time.Time  `json:"expires_at" gorm:"not null;index"`
	AuthorizedAt          *time.Time `json:"authorized_at,omitempty"`
	ConsumedAt            *time.Time `json:"consumed_at,omitempty" gorm:"index"`
	CanceledAt            *time.Time `json:"canceled_at,omitempty" gorm:"index"`
}

func (AgentConnectRequest) TableName() string {
	return "agent_connect_requests"
}

type AgentConnectRequestCreate struct {
	ClientKind          string
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string
	State               string
}

func validateAgentConnectBootstrap(clientKind string, redirectURI string, codeChallenge string, codeChallengeMethod string, state string) error {
	if err := validateAgentConnectPKCE(codeChallenge, codeChallengeMethod); err != nil {
		return err
	}
	if clientKind == agentConnectClientMyAgentsSkill {
		if redirectURI != "" || state != "" {
			return errors.New("pairing requests must not include redirect URI or state")
		}
		return nil
	}
	if clientKind != agentConnectClientMyAgents {
		return errors.New("unsupported agent connect client")
	}
	if !isPKCEValue(state) {
		return errors.New("invalid agent connect state")
	}

	parsedRedirectURI, err := url.Parse(redirectURI)
	if err != nil {
		return errors.New("invalid loopback redirect URI")
	}
	if !strings.EqualFold(parsedRedirectURI.Scheme, "http") || parsedRedirectURI.User != nil ||
		parsedRedirectURI.RawQuery != "" || parsedRedirectURI.Fragment != "" ||
		parsedRedirectURI.Path != "/callback" {
		return errors.New("invalid loopback redirect URI")
	}
	if parsedRedirectURI.Port() == "" {
		return errors.New("loopback redirect URI requires a port")
	}
	port, err := strconv.Atoi(parsedRedirectURI.Port())
	if err != nil || port < 1 || port > 65535 {
		return errors.New("invalid loopback redirect port")
	}
	ip := net.ParseIP(parsedRedirectURI.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return errors.New("redirect URI must use a loopback address")
	}
	return nil
}

func validateAgentConnectPKCE(codeChallenge string, codeChallengeMethod string) error {
	if codeChallengeMethod != agentConnectPKCES256 {
		return errors.New("agent connect requires S256 PKCE")
	}
	if !isPKCEValue(codeChallenge) {
		return errors.New("invalid PKCE code challenge")
	}
	return nil
}

func isPKCEValue(value string) bool {
	if len(value) < 32 || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '.' || char == '_' || char == '~' {
			continue
		}
		return false
	}
	return true
}

func CreateAgentConnectRequest(input AgentConnectRequestCreate) (string, *AgentConnectRequest, error) {
	if err := validateAgentConnectBootstrap(
		input.ClientKind,
		input.RedirectURI,
		input.CodeChallenge,
		input.CodeChallengeMethod,
		input.State,
	); err != nil {
		return "", nil, err
	}
	requestID, err := newAgentConnectOpaqueValue()
	if err != nil {
		return "", nil, err
	}
	request := &AgentConnectRequest{
		RequestHash:   agentConnectValueHash("request", requestID),
		ClientKind:    input.ClientKind,
		PairingMode:   input.ClientKind == agentConnectClientMyAgentsSkill,
		RedirectURI:   input.RedirectURI,
		State:         input.State,
		CodeChallenge: input.CodeChallenge,
		Status:        agentConnectStatusPending,
		ExpiresAt:     time.Now().Add(AgentConnectRequestLifetime),
	}
	if err := DB.Create(request).Error; err != nil {
		return "", nil, err
	}
	return requestID, request, nil
}

func GetAgentConnectRequest(requestID string) (*AgentConnectRequest, error) {
	if requestID == "" {
		return nil, ErrAgentConnectInvalid
	}
	var request AgentConnectRequest
	if err := DB.Where("request_hash = ?", agentConnectValueHash("request", requestID)).First(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentConnectInvalid
		}
		return nil, err
	}
	if !request.ExpiresAt.After(time.Now()) {
		return nil, ErrAgentConnectExpired
	}
	return &request, nil
}

// IsAgentConnectToken reports whether a token was issued by the agent-connect
// exchange. It prevents ordinary user API keys from being used as MCP keys.
func IsAgentConnectToken(tokenID int) (bool, error) {
	if tokenID <= 0 {
		return false, nil
	}
	var count int64
	if err := DB.Model(&AgentConnectRequest{}).Where("token_id = ?", tokenID).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 1, nil
}

func AuthorizeAgentConnectRequest(requestID string, userID int, group string, modelName string) (string, error) {
	if requestID == "" || userID <= 0 || !validAgentConnectSelection(group, modelName) {
		return "", ErrAgentConnectInvalid
	}
	authorizationCode, err := newAgentConnectOpaqueValue()
	if err != nil {
		return "", err
	}
	now := time.Now()
	err = DB.Transaction(func(tx *gorm.DB) error {
		var request AgentConnectRequest
		if err := lockForUpdate(tx).
			Where("request_hash = ?", agentConnectValueHash("request", requestID)).
			First(&request).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAgentConnectInvalid
			}
			return err
		}
		if err := validatePendingAgentConnectRequest(&request, now); err != nil {
			return err
		}
		result := tx.Model(&AgentConnectRequest{}).
			Where("id = ? AND status = ? AND expires_at > ?", request.Id, agentConnectStatusPending, now).
			Updates(map[string]any{
				"user_id":                 userID,
				"group":                   group,
				"model":                   modelName,
				"authorization_code_hash": agentConnectValueHash("authorization-code", authorizationCode),
				"status":                  agentConnectStatusAuthorized,
				"authorized_at":           now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrAgentConnectConsumed
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return authorizationCode, nil
}

func ExchangeAgentConnectRequest(requestID string, authorizationCode string, verifier string) (*Token, error) {
	if requestID == "" || authorizationCode == "" || verifier == "" {
		return nil, ErrAgentConnectInvalid
	}
	var issuedToken *Token
	err := DB.Transaction(func(tx *gorm.DB) error {
		var request AgentConnectRequest
		if err := lockForUpdate(tx).
			Where("request_hash = ?", agentConnectValueHash("request", requestID)).
			First(&request).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAgentConnectInvalid
			}
			return err
		}
		now := time.Now()
		if err := validateAuthorizedAgentConnectRequest(&request, now); err != nil {
			return err
		}
		if request.AuthorizationCodeHash == nil || subtle.ConstantTimeCompare(
			[]byte(agentConnectValueHash("authorization-code", authorizationCode)),
			[]byte(*request.AuthorizationCodeHash),
		) != 1 {
			return ErrAgentConnectInvalid
		}
		verifierHash := sha256.Sum256([]byte(verifier))
		if subtle.ConstantTimeCompare(
			[]byte(base64.RawURLEncoding.EncodeToString(verifierHash[:])),
			[]byte(request.CodeChallenge),
		) != 1 {
			return ErrAgentConnectInvalidVerifier
		}

		var issueErr error
		issuedToken, issueErr = issueAgentConnectToken(tx, &request, now)
		return issueErr
	})
	if err != nil {
		return nil, err
	}
	return issuedToken, nil
}

// ExchangeAgentConnectPairingRequest completes a Skill pairing without ever
// returning the browser authorization code to the local agent.
func ExchangeAgentConnectPairingRequest(requestID string, verifier string) (*Token, error) {
	if requestID == "" || verifier == "" {
		return nil, ErrAgentConnectInvalid
	}
	var issuedToken *Token
	err := DB.Transaction(func(tx *gorm.DB) error {
		var request AgentConnectRequest
		if err := lockForUpdate(tx).
			Where("request_hash = ?", agentConnectValueHash("request", requestID)).
			First(&request).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAgentConnectInvalid
			}
			return err
		}
		if !request.PairingMode {
			return ErrAgentConnectInvalid
		}
		now := time.Now()
		if err := validateAuthorizedAgentConnectRequest(&request, now); err != nil {
			return err
		}
		verifierHash := sha256.Sum256([]byte(verifier))
		if subtle.ConstantTimeCompare(
			[]byte(base64.RawURLEncoding.EncodeToString(verifierHash[:])),
			[]byte(request.CodeChallenge),
		) != 1 {
			return ErrAgentConnectInvalidVerifier
		}
		var err error
		issuedToken, err = issueAgentConnectToken(tx, &request, now)
		return err
	})
	if err != nil {
		return nil, err
	}
	return issuedToken, nil
}

func issueAgentConnectToken(tx *gorm.DB, request *AgentConnectRequest, now time.Time) (*Token, error) {
	var tokenCount int64
	if err := tx.Model(&Token{}).Where("user_id = ?", request.UserId).Count(&tokenCount).Error; err != nil {
		return nil, err
	}
	if int(tokenCount) >= operation_setting.GetMaxUserTokens() {
		return nil, ErrAgentConnectTokenLimit
	}
	key, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	result := tx.Model(&AgentConnectRequest{}).
		Where("id = ? AND status = ? AND expires_at > ?", request.Id, agentConnectStatusAuthorized, now).
		Updates(map[string]any{
			"status":      agentConnectStatusConsumed,
			"consumed_at": now,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, ErrAgentConnectConsumed
	}
	issuedToken := &Token{
		UserId:             request.UserId,
		Name:               "MyAgents connection",
		Key:                key,
		CreatedTime:        now.Unix(),
		AccessedTime:       now.Unix(),
		ExpiredTime:        now.Add(AgentConnectTokenLifetime).Unix(),
		UnlimitedQuota:     true,
		ModelLimitsEnabled: true,
		ModelLimits:        request.Model,
		Group:              request.Group,
	}
	if err := tx.Create(issuedToken).Error; err != nil {
		return nil, err
	}
	if err := tx.Model(&AgentConnectRequest{}).Where("id = ?", request.Id).Update("token_id", issuedToken.Id).Error; err != nil {
		return nil, err
	}
	return issuedToken, nil
}

func CancelAgentConnectRequest(requestID string, userID int) error {
	if requestID == "" || userID <= 0 {
		return ErrAgentConnectInvalid
	}
	now := time.Now()
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
		if !request.ExpiresAt.After(now) {
			return ErrAgentConnectExpired
		}
		if request.UserId != 0 && request.UserId != userID {
			return ErrAgentConnectInvalid
		}
		switch request.Status {
		case agentConnectStatusCanceled:
			return nil
		case agentConnectStatusConsumed:
			return ErrAgentConnectConsumed
		case agentConnectStatusPending, agentConnectStatusAuthorized:
			result := tx.Model(&AgentConnectRequest{}).
				Where("id = ? AND status IN ?", request.Id, []string{agentConnectStatusPending, agentConnectStatusAuthorized}).
				Updates(map[string]any{
					"user_id":     userID,
					"status":      agentConnectStatusCanceled,
					"canceled_at": now,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrAgentConnectConsumed
			}
			return nil
		default:
			return ErrAgentConnectInvalid
		}
	})
}

func validatePendingAgentConnectRequest(request *AgentConnectRequest, now time.Time) error {
	if !request.ExpiresAt.After(now) {
		return ErrAgentConnectExpired
	}
	switch request.Status {
	case agentConnectStatusPending:
		return nil
	case agentConnectStatusCanceled:
		return ErrAgentConnectCanceled
	case agentConnectStatusConsumed, agentConnectStatusAuthorized:
		return ErrAgentConnectConsumed
	default:
		return ErrAgentConnectInvalid
	}
}

func validateAuthorizedAgentConnectRequest(request *AgentConnectRequest, now time.Time) error {
	if !request.ExpiresAt.After(now) {
		return ErrAgentConnectExpired
	}
	switch request.Status {
	case agentConnectStatusAuthorized:
		return nil
	case agentConnectStatusCanceled:
		return ErrAgentConnectCanceled
	case agentConnectStatusConsumed:
		return ErrAgentConnectConsumed
	case agentConnectStatusPending:
		return ErrAgentConnectNotAuthorized
	default:
		return ErrAgentConnectInvalid
	}
}

func validAgentConnectSelection(group string, modelName string) bool {
	if group == "" || group == "auto" || modelName == "" || len(group) > 128 || len(modelName) > 255 {
		return false
	}
	return !strings.ContainsAny(group, ",\r\n") && !strings.ContainsAny(modelName, ",\r\n")
}

func newAgentConnectOpaqueValue() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func agentConnectValueHash(purpose string, value string) string {
	return common.GenerateHMACWithKey([]byte("agent-connect-v1:"+common.SessionSecret), purpose+":"+value)
}
