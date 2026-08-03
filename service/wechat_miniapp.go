package service

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"gorm.io/gorm"
)

const (
	wechatMiniAppCodeExchangeDefaultEndpoint = "https://api.weixin.qq.com/sns/jscode2session"
	miniAppAuthFlowProvider                  = "wechat-miniapp"
	miniAppPendingTicketTTL                  = 5 * time.Minute
	MiniAppLoginStatePending                 = "pending"
	MiniAppLoginStateAuthenticated           = "authenticated"
)

var (
	ErrWechatMiniCodeInvalid         = errors.New("mini app login code is invalid")
	ErrWechatMiniProviderRejected    = errors.New("mini app login code was rejected")
	ErrWechatMiniProviderUnavailable = errors.New("mini app identity provider is unavailable")
	ErrMiniAppRegistrationDisabled   = errors.New("mini app registration is disabled")
	ErrMiniAppPasswordRegistration   = errors.New("mini app password registration is disabled")
	ErrMiniAppRegistrationInvalid    = errors.New("mini app registration is invalid")
	ErrMiniAppEmailVerification      = errors.New("mini app email verification is required")
	ErrMiniAppIdentityUnbound        = errors.New("mini app identity is not bound")
	ErrMiniAppSessionOwnership       = errors.New("mini app session does not belong to the identity")

	wechatMiniAppCodeExchangeEndpoint = wechatMiniAppCodeExchangeDefaultEndpoint
	wechatMiniAppHTTPClientFactory    = func(timeout time.Duration) *http.Client {
		return &http.Client{Timeout: timeout}
	}
)

// WechatMiniSubject is the privacy-preserving representation of a WeChat Mini
// Program subject. It deliberately contains no plaintext OpenID or session key.
type WechatMiniSubject struct {
	AppID      string `json:"-"`
	OpenIDHash string `json:"-"`
}

// MiniAppLoginResult contains either a short-lived Mini Program session for
// an already bound subject, or the opaque pending ticket needed to bind or
// register an unbound subject.
type MiniAppLoginResult struct {
	Session       *AuthBundle `json:"session,omitempty"`
	PendingTicket string      `json:"pending_ticket,omitempty"`
	State         string      `json:"state"`
}

type miniAppPendingIdentityPayload struct {
	AppID      string `json:"app_id"`
	OpenIDHash string `json:"open_id_hash"`
}

type miniAppBindingFlowPayload struct {
	PendingFlowID int64  `json:"pending_flow_id"`
	BindingID     string `json:"binding_id"`
	AppID         string `json:"app_id"`
	OpenIDHash    string `json:"open_id_hash"`
}

// MiniAppBindingStart is the browser handoff for an unbound Mini Program
// subject. Both query parameters are opaque one-time credentials; no raw
// WeChat identity material appears in the URL.
type MiniAppBindingStart struct {
	BindURL string `json:"bind_url"`
}

// MiniAppRegistration is the password-registration subset that a Mini Program
// BFF may submit after it has a pending identity ticket. It intentionally
// mirrors the durable dashboard registration fields instead of granting a
// weaker Mini Program-only account path.
type MiniAppRegistration struct {
	Username         string
	Password         string
	Email            string
	VerificationCode string
	AffCode          string
}

type wechatMiniCodeExchangeResponse struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// ExchangeWechatMiniCode exchanges the one-use wx.login code directly with
// WeChat. Provider secrets and raw identity material are intentionally never
// returned, logged, or persisted.
func ExchangeWechatMiniCode(code string) (*WechatMiniSubject, error) {
	config, err := RequireMiniProgramConfig()
	if err != nil {
		return nil, err
	}
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 2048 {
		return nil, ErrWechatMiniCodeInvalid
	}

	endpoint, err := url.Parse(wechatMiniAppCodeExchangeEndpoint)
	if err != nil || endpoint.Scheme != "https" && endpoint.Scheme != "http" || endpoint.Host == "" {
		return nil, ErrWechatMiniProviderUnavailable
	}
	query := endpoint.Query()
	query.Set("appid", config.AppID)
	query.Set("secret", common.WeChatMiniAppAppSecret)
	query.Set("js_code", code)
	query.Set("grant_type", "authorization_code")
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, ErrWechatMiniProviderUnavailable
	}
	response, err := wechatMiniAppHTTPClientFactory(config.HTTPTimeout).Do(request)
	if err != nil {
		return nil, ErrWechatMiniProviderUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, ErrWechatMiniProviderUnavailable
	}

	var exchange wechatMiniCodeExchangeResponse
	if err := common.DecodeJson(response.Body, &exchange); err != nil {
		return nil, ErrWechatMiniProviderUnavailable
	}
	openID := strings.TrimSpace(exchange.OpenID)
	exchange.OpenID = ""
	exchange.SessionKey = ""
	exchange.ErrMsg = ""
	if exchange.ErrCode != 0 {
		return nil, ErrWechatMiniProviderRejected
	}
	if openID == "" || len(openID) > 128 {
		return nil, ErrWechatMiniProviderRejected
	}
	return &WechatMiniSubject{
		AppID:      config.AppID,
		OpenIDHash: common.GenerateHMACWithKey([]byte("wechat-miniapp-openid-v1:"+common.SessionSecret), config.AppID+":"+openID),
	}, nil
}

// StartMiniAppLogin exchanges a fresh Mini Program code and either issues a
// session for the already-owned subject or creates an opaque five-minute
// pending identity ticket. It never creates a durable identity for an
// unbound subject.
func StartMiniAppLogin(code, ip, userAgent string) (*MiniAppLoginResult, error) {
	subject, err := ExchangeWechatMiniCode(code)
	if err != nil {
		return nil, err
	}
	var identity model.WechatMiniIdentity
	err = model.DB.Where("app_id = ? AND open_id_hash = ?", subject.AppID, subject.OpenIDHash).First(&identity).Error
	if err == nil {
		bundle, err := CreateMiniAppLoginSession(identity.UserID, ip, userAgent)
		if err != nil {
			return nil, err
		}
		return &MiniAppLoginResult{Session: bundle, State: MiniAppLoginStateAuthenticated}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	payload, err := common.Marshal(miniAppPendingIdentityPayload{AppID: subject.AppID, OpenIDHash: subject.OpenIDHash})
	if err != nil {
		return nil, err
	}
	ticket, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeMiniAppPendingIdentity,
		Provider:  miniAppAuthFlowProvider,
		Intent:    model.AuthFlowIntentLogin,
		Payload:   string(payload),
		ExpiresAt: time.Now().Add(miniAppPendingTicketTTL),
	})
	if err != nil {
		return nil, err
	}
	return &MiniAppLoginResult{PendingTicket: ticket, State: MiniAppLoginStatePending}, nil
}

// CreateMiniAppBinding validates, but does not consume, a pending ticket and
// prepares the separate browser confirmation flow.
func CreateMiniAppBinding(pendingTicket string) (*MiniAppBindingStart, error) {
	config, err := RequireMiniProgramConfig()
	if err != nil {
		return nil, err
	}
	pending, err := model.GetAuthFlow(pendingTicket, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeMiniAppPendingIdentity, Provider: miniAppAuthFlowProvider, Intent: model.AuthFlowIntentLogin,
	})
	if err != nil {
		return nil, err
	}
	var pendingPayload miniAppPendingIdentityPayload
	if err := common.Unmarshal([]byte(pending.Payload), &pendingPayload); err != nil ||
		pendingPayload.AppID == "" || pendingPayload.OpenIDHash == "" {
		return nil, model.ErrAuthFlowInvalid
	}
	binding, err := model.CreateMiniAppBinding(model.MiniAppBindingCreate{
		PendingFlowID: pending.Id,
		AppID:         pendingPayload.AppID,
		OpenIDHash:    pendingPayload.OpenIDHash,
		ExpiresAt:     pending.ExpiresAt,
	})
	if err != nil {
		return nil, err
	}
	payload, err := common.Marshal(miniAppBindingFlowPayload{
		PendingFlowID: pending.Id,
		BindingID:     binding.ID,
		AppID:         pendingPayload.AppID,
		OpenIDHash:    pendingPayload.OpenIDHash,
	})
	if err != nil {
		return nil, err
	}
	bindingTicket, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeMiniAppBind,
		Provider:  miniAppAuthFlowProvider,
		Intent:    model.AuthFlowIntentBind,
		Payload:   string(payload),
		ExpiresAt: pending.ExpiresAt,
	})
	if err != nil {
		return nil, err
	}
	bindingURL, err := url.Parse(config.BindWebBaseURL)
	if err != nil {
		return nil, ErrMiniAppConfiguration
	}
	query := bindingURL.Query()
	query.Set("ticket", pendingTicket)
	query.Set("binding_ticket", bindingTicket)
	bindingURL.RawQuery = query.Encode()
	return &MiniAppBindingStart{BindURL: bindingURL.String()}, nil
}

// ConfirmMiniAppBinding requires an authenticated browser session, then
// consumes both one-time flows and claims the durable Mini Program identity in
// one transaction. The payload comparisons prevent tickets being mixed across
// unrelated Mini Program subjects.
func ConfirmMiniAppBinding(pendingTicket, bindingTicket string, browserIdentity AuthIdentity) error {
	if _, err := RequireMiniProgramConfig(); err != nil {
		return err
	}
	if _, _, err := ValidateLoginSession(browserIdentity); err != nil {
		return err
	}
	_, err := model.ConsumeAuthFlowWithAction(bindingTicket, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeMiniAppBind, Provider: miniAppAuthFlowProvider, Intent: model.AuthFlowIntentBind,
	}, func(tx *gorm.DB, bindingFlow *model.AuthFlow) error {
		var bindingPayload miniAppBindingFlowPayload
		if err := common.Unmarshal([]byte(bindingFlow.Payload), &bindingPayload); err != nil ||
			bindingPayload.PendingFlowID <= 0 || bindingPayload.BindingID == "" ||
			bindingPayload.AppID == "" || bindingPayload.OpenIDHash == "" {
			return model.ErrAuthFlowInvalid
		}
		_, err := model.ConsumeAuthFlowWithTx(tx, pendingTicket, model.AuthFlowMatch{
			Purpose: model.AuthFlowPurposeMiniAppPendingIdentity, Provider: miniAppAuthFlowProvider, Intent: model.AuthFlowIntentLogin,
		}, func(tx *gorm.DB, pendingFlow *model.AuthFlow) error {
			if pendingFlow.Id != bindingPayload.PendingFlowID {
				return model.ErrAuthFlowInvalid
			}
			var pendingPayload miniAppPendingIdentityPayload
			if err := common.Unmarshal([]byte(pendingFlow.Payload), &pendingPayload); err != nil ||
				pendingPayload.AppID != bindingPayload.AppID || pendingPayload.OpenIDHash != bindingPayload.OpenIDHash {
				return model.ErrAuthFlowInvalid
			}
			_, err := model.ConfirmMiniAppBindingWithTx(tx, model.MiniAppBindingConfirmation{
				BindingID:     bindingPayload.BindingID,
				PendingFlowID: pendingFlow.Id,
				AppID:         pendingPayload.AppID,
				OpenIDHash:    pendingPayload.OpenIDHash,
				UserID:        browserIdentity.UserID,
			})
			return err
		})
		return err
	})
	return err
}

// GetMiniAppBindingStatus returns only the pending, bound, or expired state
// for a caller that proves possession of the pending ticket.
func GetMiniAppBindingStatus(pendingTicket string) (string, error) {
	if _, err := RequireMiniProgramConfig(); err != nil {
		return "", err
	}
	flow, err := model.GetAuthFlowState(pendingTicket, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeMiniAppPendingIdentity, Provider: miniAppAuthFlowProvider, Intent: model.AuthFlowIntentLogin,
	})
	if err != nil {
		return "", err
	}
	var payload miniAppPendingIdentityPayload
	if err := common.Unmarshal([]byte(flow.Payload), &payload); err != nil || payload.AppID == "" || payload.OpenIDHash == "" {
		return "", model.ErrAuthFlowInvalid
	}
	var binding model.MiniAppBinding
	bindingErr := model.DB.Where("pending_flow_id = ? AND app_id = ? AND open_id_hash = ?", flow.Id, payload.AppID, payload.OpenIDHash).
		Order("created_at DESC").First(&binding).Error
	if bindingErr != nil && !errors.Is(bindingErr, gorm.ErrRecordNotFound) {
		return "", bindingErr
	}
	if bindingErr == nil && binding.Status == model.MiniAppBindingStatusBound {
		return model.MiniAppBindingStatusBound, nil
	}
	if !flow.ExpiresAt.After(time.Now()) || bindingErr == nil && binding.Status == model.MiniAppBindingStatusExpired {
		return model.MiniAppBindingStatusExpired, nil
	}
	if flow.ConsumedAt != nil {
		return "", model.ErrAuthFlowConsumed
	}
	return model.MiniAppBindingStatusPending, nil
}

// RegisterMiniAppUser consumes a pending subject ticket only after the normal
// password-registration validation has passed. The user row and durable
// subject claim commit together; session issuance happens only afterwards.
// HTTP rate limits remain a boundary concern for the Mini Program BFF, just as
// the existing dashboard Register controller applies its critical limiter.
func RegisterMiniAppUser(pendingTicket string, registration MiniAppRegistration, ip, userAgent string) (*AuthBundle, error) {
	if _, err := RequireMiniProgramConfig(); err != nil {
		return nil, err
	}
	if !common.RegisterEnabled {
		return nil, ErrMiniAppRegistrationDisabled
	}
	if !common.PasswordRegisterEnabled {
		return nil, ErrMiniAppPasswordRegistration
	}
	registration.Username = strings.TrimSpace(registration.Username)
	registration.Email = model.NormalizeEmail(registration.Email)
	if registration.Username == "" {
		return nil, ErrMiniAppRegistrationInvalid
	}
	candidate := model.User{
		Username:         registration.Username,
		Password:         registration.Password,
		Email:            registration.Email,
		VerificationCode: registration.VerificationCode,
	}
	if err := common.Validate.Struct(&candidate); err != nil {
		return nil, ErrMiniAppRegistrationInvalid
	}
	if common.EmailVerificationEnabled {
		if candidate.Email == "" || candidate.VerificationCode == "" ||
			!common.VerifyCodeWithKey(candidate.Email, candidate.VerificationCode, common.EmailVerificationPurpose) {
			return nil, ErrMiniAppEmailVerification
		}
		if err := model.EnsureEmailAvailable(candidate.Email, 0); err != nil {
			return nil, err
		}
	}
	emailForExistCheck := ""
	if common.EmailVerificationEnabled {
		emailForExistCheck = candidate.Email
	}
	exists, err := model.CheckUserExistOrDeleted(candidate.Username, emailForExistCheck)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrMiniAppRegistrationInvalid
	}
	inviterID, _ := model.GetUserIdByAffCode(registration.AffCode)
	newUser := model.User{
		Username:    candidate.Username,
		Password:    candidate.Password,
		DisplayName: candidate.Username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		InviterId:   inviterID,
	}
	if common.EmailVerificationEnabled {
		newUser.Email = candidate.Email
	}
	_, err = model.ConsumeAuthFlowWithAction(pendingTicket, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeMiniAppPendingIdentity, Provider: miniAppAuthFlowProvider, Intent: model.AuthFlowIntentLogin,
	}, func(tx *gorm.DB, flow *model.AuthFlow) error {
		var pendingPayload miniAppPendingIdentityPayload
		if err := common.Unmarshal([]byte(flow.Payload), &pendingPayload); err != nil ||
			pendingPayload.AppID == "" || pendingPayload.OpenIDHash == "" {
			return model.ErrAuthFlowInvalid
		}
		if err := newUser.InsertWithTx(tx, inviterID); err != nil {
			return err
		}
		_, err := model.ClaimWechatMiniIdentityWithTx(tx, pendingPayload.AppID, pendingPayload.OpenIDHash, newUser.Id)
		return err
	})
	if err != nil {
		return nil, err
	}
	newUser.FinishInsert(inviterID)
	return CreateMiniAppLoginSession(newUser.Id, ip, userAgent)
}

// RenewMiniAppLogin is the only Mini Program session-renewal path. It accepts
// a fresh wx.login code and the prior Mini Program SID; it never accepts a
// browser refresh cookie, personal access token, or client-supplied user ID.
func RenewMiniAppLogin(code, priorSID, ip, userAgent string) (*AuthBundle, error) {
	subject, err := ExchangeWechatMiniCode(code)
	if err != nil {
		return nil, err
	}
	var identity model.WechatMiniIdentity
	if err := model.DB.Where("app_id = ? AND open_id_hash = ?", subject.AppID, subject.OpenIDHash).First(&identity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMiniAppIdentityUnbound
		}
		return nil, err
	}
	priorSession, err := model.GetUserSessionCached(strings.TrimSpace(priorSID))
	if err != nil {
		return nil, ErrMiniAppSessionOwnership
	}
	if priorSession.UserID != identity.UserID || priorSession.LoginMethod != "wechat-miniapp" ||
		priorSession.Status != model.UserSessionStatusActive || priorSession.RevokedAt != 0 || priorSession.ExpiresAt <= time.Now().Unix() {
		return nil, ErrMiniAppSessionOwnership
	}
	bundle, err := CreateMiniAppLoginSession(identity.UserID, ip, userAgent)
	if err != nil {
		return nil, err
	}
	if _, err := model.RevokeUserSession(identity.UserID, priorSession.SID, "miniapp_renewed"); err != nil {
		return nil, err
	}
	return bundle, nil
}

// MiniAppAuthErrorCode gives the Mini Program BFF a stable, non-sensitive
// response mapping. In particular, WeChat's errcode/errmsg and session data
// are never reflected to the client.
func MiniAppAuthErrorCode(err error) (int, string) {
	switch {
	case errors.Is(err, ErrMiniProgramDisabled):
		return http.StatusNotFound, "MINIAPP_DISABLED"
	case errors.Is(err, ErrMiniAppConfiguration):
		return http.StatusServiceUnavailable, "MINIAPP_CONFIGURATION_ERROR"
	case errors.Is(err, ErrWechatMiniCodeInvalid):
		return http.StatusBadRequest, "MINIAPP_CODE_INVALID"
	case errors.Is(err, ErrWechatMiniProviderRejected):
		return http.StatusUnauthorized, "MINIAPP_CODE_REJECTED"
	case errors.Is(err, ErrWechatMiniProviderUnavailable):
		return http.StatusBadGateway, "MINIAPP_PROVIDER_UNAVAILABLE"
	case errors.Is(err, ErrMiniAppRegistrationDisabled), errors.Is(err, ErrMiniAppPasswordRegistration):
		return http.StatusForbidden, "MINIAPP_REGISTRATION_DISABLED"
	case errors.Is(err, ErrMiniAppRegistrationInvalid), errors.Is(err, ErrMiniAppEmailVerification):
		return http.StatusBadRequest, "MINIAPP_REGISTRATION_INVALID"
	case errors.Is(err, model.ErrAuthFlowExpired), errors.Is(err, model.ErrMiniAppBindingExpired):
		return http.StatusGone, "MINIAPP_TICKET_EXPIRED"
	case errors.Is(err, model.ErrAuthFlowConsumed), errors.Is(err, model.ErrMiniAppBindingAlreadyBound), errors.Is(err, model.ErrWechatMiniIdentityAlreadyBound):
		return http.StatusConflict, "MINIAPP_TICKET_CONSUMED"
	case errors.Is(err, model.ErrAuthFlowInvalid), errors.Is(err, model.ErrMiniAppBindingInvalid),
		errors.Is(err, ErrMiniAppIdentityUnbound), errors.Is(err, ErrMiniAppSessionOwnership):
		return http.StatusUnauthorized, "MINIAPP_UNAUTHORIZED"
	default:
		return http.StatusInternalServerError, "MINIAPP_INTERNAL_ERROR"
	}
}
