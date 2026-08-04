package service

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	wechatMiniAppCodeExchangeDefaultEndpoint = "https://api.weixin.qq.com/sns/jscode2session"
	miniAppAuthFlowProvider                  = "wechat-miniapp"
	miniAppPendingTicketTTL                  = 5 * time.Minute
	wechatMiniSubjectHashDomain              = "wechat-miniapp-openid-v1"
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
	ErrMiniAppBrowserSessionRequired = errors.New("a regular browser session is required for mini app binding")

	wechatMiniAppCodeExchangeEndpoint = wechatMiniAppCodeExchangeDefaultEndpoint
	wechatMiniAppHTTPClientFactory    = func(timeout time.Duration) *http.Client {
		return &http.Client{
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
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
// subject. Its URL carries exactly one opaque one-time bind credential; no
// raw WeChat identity material appears in the URL.
type MiniAppBindingStart struct {
	BindingID string `json:"binding_id"`
	BindURL   string `json:"bind_url"`
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
func ExchangeWechatMiniCode(ctx context.Context, code string) (*WechatMiniSubject, error) {
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
	query.Set("secret", config.appSecret)
	query.Set("js_code", code)
	query.Set("grant_type", "authorization_code")
	endpoint.RawQuery = query.Encode()
	if ctx == nil {
		ctx = context.Background()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
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
		OpenIDHash: deriveWechatMiniSubjectHash(config, openID),
	}, nil
}

func deriveWechatMiniSubjectHash(config MiniAppConfig, openID string) string {
	return common.GenerateHMACWithKey([]byte(config.subjectHMACKey), wechatMiniSubjectHashDomain+":"+config.AppID+":"+openID)
}

// StartMiniAppLogin exchanges a fresh Mini Program code and either issues a
// session for the already-owned subject or creates an opaque five-minute
// pending identity ticket. It never creates a durable identity for an
// unbound subject.
func StartMiniAppLogin(ctx context.Context, code, ip, userAgent string) (*MiniAppLoginResult, error) {
	subject, err := ExchangeWechatMiniCode(ctx, code)
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
	bindingURL, err := url.Parse(config.BindWebBaseURL)
	if err != nil {
		return nil, ErrMiniAppConfiguration
	}

	var bindingID, bindingTicket string
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		pending, err := model.GetAuthFlowWithTx(tx, pendingTicket, model.AuthFlowMatch{
			Purpose: model.AuthFlowPurposeMiniAppPendingIdentity, Provider: miniAppAuthFlowProvider, Intent: model.AuthFlowIntentLogin,
		})
		if err != nil {
			return err
		}
		var pendingPayload miniAppPendingIdentityPayload
		if err := common.Unmarshal([]byte(pending.Payload), &pendingPayload); err != nil ||
			pendingPayload.AppID == "" || pendingPayload.OpenIDHash == "" {
			return model.ErrAuthFlowInvalid
		}
		binding, err := model.CreateMiniAppBindingWithTx(tx, model.MiniAppBindingCreate{
			PendingFlowID: pending.Id,
			AppID:         pendingPayload.AppID,
			OpenIDHash:    pendingPayload.OpenIDHash,
			ExpiresAt:     pending.ExpiresAt,
		})
		if err != nil {
			return err
		}
		payload, err := common.Marshal(miniAppBindingFlowPayload{
			PendingFlowID: pending.Id,
			BindingID:     binding.ID,
			AppID:         pendingPayload.AppID,
			OpenIDHash:    pendingPayload.OpenIDHash,
		})
		if err != nil {
			return err
		}
		bindingTicket, _, err = model.CreateAuthFlowWithTx(tx, model.AuthFlowCreate{
			Purpose:   model.AuthFlowPurposeMiniAppBind,
			Provider:  miniAppAuthFlowProvider,
			Intent:    model.AuthFlowIntentBind,
			Payload:   string(payload),
			ExpiresAt: pending.ExpiresAt,
		})
		if err != nil {
			return err
		}
		bindingID = binding.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	bindingURL.Fragment = "binding_ticket=" + bindingTicket
	return &MiniAppBindingStart{BindingID: bindingID, BindURL: bindingURL.String()}, nil
}

// ConfirmMiniAppBinding requires an authenticated browser session, then
// consumes the opaque browser bind flow and its server-linked pending flow in
// one transaction. The browser never receives the pending identity ticket.
func ConfirmMiniAppBinding(bindingTicket string, browserIdentity AuthIdentity) error {
	if _, err := RequireMiniProgramConfig(); err != nil {
		return err
	}
	session, _, err := ValidateLoginSession(browserIdentity)
	if err != nil {
		return err
	}
	if session.LoginMethod == "wechat-miniapp" {
		return ErrMiniAppBrowserSessionRequired
	}
	_, err = model.ConsumeAuthFlowWithAction(bindingTicket, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeMiniAppBind, Provider: miniAppAuthFlowProvider, Intent: model.AuthFlowIntentBind,
	}, func(tx *gorm.DB, bindingFlow *model.AuthFlow) error {
		var bindingPayload miniAppBindingFlowPayload
		if err := common.Unmarshal([]byte(bindingFlow.Payload), &bindingPayload); err != nil ||
			bindingPayload.PendingFlowID <= 0 || bindingPayload.BindingID == "" ||
			bindingPayload.AppID == "" || bindingPayload.OpenIDHash == "" {
			return model.ErrAuthFlowInvalid
		}
		_, err := model.ConsumeAuthFlowByIDWithTx(tx, bindingPayload.PendingFlowID, model.AuthFlowMatch{
			Purpose: model.AuthFlowPurposeMiniAppPendingIdentity, Provider: miniAppAuthFlowProvider, Intent: model.AuthFlowIntentLogin,
		}, func(tx *gorm.DB, pendingFlow *model.AuthFlow) error {
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
	return getMiniAppBindingStatus(pendingTicket, "")
}

// GetMiniAppBindingStatusForBinding returns the status of exactly one binding
// after the Mini Program proves possession of the pending identity ticket.
func GetMiniAppBindingStatusForBinding(pendingTicket, bindingID string) (string, error) {
	bindingID = strings.TrimSpace(bindingID)
	if bindingID == "" {
		return "", model.ErrAuthFlowInvalid
	}
	return getMiniAppBindingStatus(pendingTicket, bindingID)
}

func getMiniAppBindingStatus(pendingTicket, bindingID string) (string, error) {
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
	bindingQuery := model.DB.Where("pending_flow_id = ? AND app_id = ? AND open_id_hash = ?", flow.Id, payload.AppID, payload.OpenIDHash)
	if bindingID != "" {
		bindingQuery = bindingQuery.Where("id = ?", bindingID)
	}
	bindingErr := bindingQuery.Order("created_at DESC").First(&binding).Error
	if bindingErr != nil && !errors.Is(bindingErr, gorm.ErrRecordNotFound) {
		return "", bindingErr
	}
	if bindingID != "" && errors.Is(bindingErr, gorm.ErrRecordNotFound) {
		return "", model.ErrAuthFlowInvalid
	}
	if bindingErr == nil {
		if binding.Status == model.MiniAppBindingStatusBound {
			return model.MiniAppBindingStatusBound, nil
		}
		if !flow.ExpiresAt.After(time.Now()) || binding.Status == model.MiniAppBindingStatusExpired {
			return model.MiniAppBindingStatusExpired, nil
		}
		if flow.ConsumedAt != nil {
			return model.MiniAppBindingStatusExpired, nil
		}
		return model.MiniAppBindingStatusPending, nil
	}
	var identity model.WechatMiniIdentity
	identityErr := model.DB.Where("app_id = ? AND open_id_hash = ?", payload.AppID, payload.OpenIDHash).First(&identity).Error
	if identityErr == nil {
		return model.MiniAppBindingStatusBound, nil
	}
	if !errors.Is(identityErr, gorm.ErrRecordNotFound) {
		return "", identityErr
	}
	if !flow.ExpiresAt.After(time.Now()) {
		return model.MiniAppBindingStatusExpired, nil
	}
	if flow.ConsumedAt != nil {
		return model.MiniAppBindingStatusExpired, nil
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
func RenewMiniAppLogin(ctx context.Context, code, priorSID, ip, userAgent string) (*AuthBundle, error) {
	subject, err := ExchangeWechatMiniCode(ctx, code)
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
	user, err := model.GetUserCache(identity.UserID)
	if err != nil {
		return nil, err
	}
	if user.Status != common.UserStatusEnabled || user.AuthVersion <= 0 {
		return nil, ErrMiniAppSessionOwnership
	}
	refreshSecret, err := common.GenerateRandomCharsKey(64)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	session := &model.UserSession{
		SID:             uuid.NewString(),
		UserID:          identity.UserID,
		Version:         1,
		UserAuthVersion: user.AuthVersion,
		RefreshHash:     hashRefreshSecret(refreshSecret),
		LoginMethod:     "wechat-miniapp",
		IP:              truncateAuthMetadata(ip, 64),
		UserAgent:       truncateAuthMetadata(userAgent, 512),
		CreatedAt:       now,
		LastActiveAt:    now,
		ExpiresAt:       time.Unix(now, 0).Add(LoginSessionTTL).Unix(),
	}
	session, err = model.RenewMiniAppUserSession(identity.UserID, strings.TrimSpace(priorSID), session)
	if err != nil {
		if errors.Is(err, model.ErrUserSessionInactive) {
			return nil, ErrMiniAppSessionOwnership
		}
		return nil, err
	}
	bundle, err := issueAuthBundle(session, "", true)
	if err != nil {
		_, _ = model.RevokeUserSession(identity.UserID, session.SID, "token_issue_failed")
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
	case errors.Is(err, ErrMiniProgramTextTestDisabled):
		return http.StatusNotFound, "MINIAPP_TEXT_TEST_DISABLED"
	case errors.Is(err, ErrMiniAppConfiguration):
		return http.StatusServiceUnavailable, "MINIAPP_CONFIGURATION_ERROR"
	case errors.Is(err, ErrMiniTextTestInvalid):
		return http.StatusBadRequest, "MINIAPP_TEXT_TEST_INVALID"
	case errors.Is(err, ErrMiniTextTestRequestConflict):
		return http.StatusConflict, "MINIAPP_TEXT_TEST_REQUEST_CONFLICT"
	case errors.Is(err, ErrMiniTextTestModelUnavailable):
		return http.StatusForbidden, "MINIAPP_TEXT_TEST_MODEL_UNAVAILABLE"
	case errors.Is(err, ErrMiniTextTestNotFound):
		return http.StatusNotFound, "MINIAPP_TEXT_TEST_NOT_FOUND"
	case errors.Is(err, ErrWechatMiniCodeInvalid):
		return http.StatusBadRequest, "MINIAPP_CODE_INVALID"
	case errors.Is(err, ErrWechatMiniProviderRejected):
		return http.StatusUnauthorized, "MINIAPP_CODE_REJECTED"
	case errors.Is(err, ErrWechatMiniProviderUnavailable):
		return http.StatusBadGateway, "MINIAPP_PROVIDER_UNAVAILABLE"
	case errors.Is(err, ErrMiniAppRegistrationDisabled), errors.Is(err, ErrMiniAppPasswordRegistration):
		return http.StatusForbidden, "MINIAPP_REGISTRATION_DISABLED"
	case errors.Is(err, ErrMiniAppBrowserSessionRequired):
		return http.StatusForbidden, "MINIAPP_BROWSER_SESSION_REQUIRED"
	case errors.Is(err, ErrMiniAppRegistrationInvalid), errors.Is(err, ErrMiniAppEmailVerification):
		return http.StatusBadRequest, "MINIAPP_REGISTRATION_INVALID"
	case errors.Is(err, ErrMiniAppTokenInvalid), errors.Is(err, ErrMiniAppTokenExpired), errors.Is(err, ErrMiniAppTokenLimit):
		return http.StatusBadRequest, "MINIAPP_TOKEN_INVALID"
	case errors.Is(err, ErrMiniAppTokenNotFound):
		return http.StatusNotFound, "MINIAPP_TOKEN_NOT_FOUND"
	case errors.Is(err, ErrMiniAppCheckoutInvalid):
		return http.StatusBadRequest, "MINIAPP_CHECKOUT_INVALID"
	case errors.Is(err, ErrMiniAppCheckoutUnavailable):
		return http.StatusNotFound, "MINIAPP_CHECKOUT_UNAVAILABLE"
	case errors.Is(err, model.ErrAuthFlowExpired), errors.Is(err, model.ErrMiniAppBindingExpired):
		return http.StatusGone, "MINIAPP_TICKET_EXPIRED"
	case errors.Is(err, model.ErrAuthFlowConsumed), errors.Is(err, model.ErrMiniAppBindingAlreadyExists), errors.Is(err, model.ErrMiniAppBindingAlreadyBound), errors.Is(err, model.ErrWechatMiniIdentityAlreadyBound):
		return http.StatusConflict, "MINIAPP_TICKET_CONSUMED"
	case errors.Is(err, model.ErrAuthFlowInvalid), errors.Is(err, model.ErrMiniAppBindingInvalid),
		errors.Is(err, ErrMiniAppIdentityUnbound), errors.Is(err, ErrMiniAppSessionOwnership):
		return http.StatusUnauthorized, "MINIAPP_UNAUTHORIZED"
	default:
		return http.StatusInternalServerError, "MINIAPP_INTERNAL_ERROR"
	}
}
