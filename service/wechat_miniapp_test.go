package service

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useMiniAppExchangeTestConfig(t *testing.T) {
	t.Helper()
	previousAppID := common.WeChatMiniAppAppID
	previousAppSecret := common.WeChatMiniAppAppSecret
	previousSubjectHMACKey := common.WeChatMiniAppSubjectHMACKey
	previousBindURL := common.MiniAppBindWebBaseURL
	previousTimeout := common.MiniAppHTTPTimeout
	previousEnabled, previousTextEnabled := common.GetMiniProgramFeatureFlags()
	previousEndpoint := wechatMiniAppCodeExchangeEndpoint
	previousClientFactory := wechatMiniAppHTTPClientFactory

	common.WeChatMiniAppAppID = "wx-test-app"
	common.WeChatMiniAppAppSecret = "miniapp-test-secret"
	common.WeChatMiniAppSubjectHMACKey = "miniapp-test-subject-hmac-key-v1"
	common.MiniAppBindWebBaseURL = "https://console.example.com/miniapp-bind"
	common.MiniAppHTTPTimeout = time.Second
	common.OptionMapRWMutex.Lock()
	common.MiniProgramEnabled = true
	common.MiniProgramTextTestEnabled = false
	common.OptionMapRWMutex.Unlock()
	miniAppConfigOnce = sync.Once{}
	cachedMiniAppConfig = MiniAppConfig{}
	cachedMiniAppConfigErr = nil

	t.Cleanup(func() {
		common.WeChatMiniAppAppID = previousAppID
		common.WeChatMiniAppAppSecret = previousAppSecret
		common.WeChatMiniAppSubjectHMACKey = previousSubjectHMACKey
		common.MiniAppBindWebBaseURL = previousBindURL
		common.MiniAppHTTPTimeout = previousTimeout
		common.OptionMapRWMutex.Lock()
		common.MiniProgramEnabled = previousEnabled
		common.MiniProgramTextTestEnabled = previousTextEnabled
		common.OptionMapRWMutex.Unlock()
		wechatMiniAppCodeExchangeEndpoint = previousEndpoint
		wechatMiniAppHTTPClientFactory = previousClientFactory
		miniAppConfigOnce = sync.Once{}
		cachedMiniAppConfig = MiniAppConfig{}
		cachedMiniAppConfigErr = nil
	})
}

type blockingWechatMiniAppRoundTripper struct {
	started chan struct{}
}

func (transport *blockingWechatMiniAppRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	close(transport.started)
	<-request.Context().Done()
	return nil, request.Context().Err()
}

func TestExchangeWechatMiniCodeRejectsMissingSubjectHMACKey(t *testing.T) {
	useMiniAppExchangeTestConfig(t)
	common.WeChatMiniAppSubjectHMACKey = ""
	miniAppConfigOnce = sync.Once{}

	_, err := ExchangeWechatMiniCode(context.Background(), "one-use-code")

	require.ErrorIs(t, err, ErrMiniAppConfiguration)
}

func TestExchangeWechatMiniCodeRejectsProviderAndTransportFailuresWithoutSecrets(t *testing.T) {
	t.Run("WeChat error code", func(t *testing.T) {
		useMiniAppExchangeTestConfig(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			payload, err := common.Marshal(map[string]any{
				"errcode":     40029,
				"errmsg":      "invalid code session-key-must-not-escape",
				"session_key": "session-key-must-not-escape",
			})
			require.NoError(t, err)
			_, _ = w.Write(payload)
		}))
		t.Cleanup(server.Close)
		wechatMiniAppCodeExchangeEndpoint = server.URL

		_, err := ExchangeWechatMiniCode(context.Background(), "bad-code")

		require.ErrorIs(t, err, ErrWechatMiniProviderRejected)
		assert.NotContains(t, err.Error(), "session-key-must-not-escape")
	})

	t.Run("malformed JSON", func(t *testing.T) {
		useMiniAppExchangeTestConfig(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("not-json-session-key-must-not-escape"))
		}))
		t.Cleanup(server.Close)
		wechatMiniAppCodeExchangeEndpoint = server.URL

		_, err := ExchangeWechatMiniCode(context.Background(), "bad-json")

		require.ErrorIs(t, err, ErrWechatMiniProviderUnavailable)
		assert.NotContains(t, err.Error(), "session-key-must-not-escape")
	})

	t.Run("timeout", func(t *testing.T) {
		useMiniAppExchangeTestConfig(t)
		common.MiniAppHTTPTimeout = 10 * time.Millisecond
		miniAppConfigOnce = sync.Once{}
		transport := &blockingWechatMiniAppRoundTripper{started: make(chan struct{})}
		wechatMiniAppHTTPClientFactory = func(timeout time.Duration) *http.Client {
			return &http.Client{Timeout: timeout, Transport: transport}
		}

		_, err := ExchangeWechatMiniCode(context.Background(), "slow-code")

		require.ErrorIs(t, err, ErrWechatMiniProviderUnavailable)
		select {
		case <-transport.started:
		default:
			require.Fail(t, "exchange request never reached the transport")
		}
	})
}

func TestExchangeWechatMiniCodePropagatesCallerCancellation(t *testing.T) {
	useMiniAppExchangeTestConfig(t)
	transport := &blockingWechatMiniAppRoundTripper{started: make(chan struct{})}
	wechatMiniAppHTTPClientFactory = func(timeout time.Duration) *http.Client {
		return &http.Client{Timeout: timeout, Transport: transport}
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	result := make(chan error, 1)
	go func() {
		_, err := ExchangeWechatMiniCode(ctx, "cancelled-code")
		result <- err
	}()

	<-transport.started
	cancel()
	err := <-result

	require.ErrorIs(t, err, ErrWechatMiniProviderUnavailable)
}

func TestExchangeWechatMiniCodeRejectsRedirectsBeforeSecretQueryCanLeaveProvider(t *testing.T) {
	useMiniAppExchangeTestConfig(t)
	redirectTargetHits := 0
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirectTargetHits++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(redirectTarget.Close)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "miniapp-test-secret", request.URL.Query().Get("secret"))
		assert.Equal(t, "redirect-code", request.URL.Query().Get("js_code"))
		http.Redirect(w, request, redirectTarget.URL, http.StatusFound)
	}))
	t.Cleanup(provider.Close)
	wechatMiniAppCodeExchangeEndpoint = provider.URL

	_, err := ExchangeWechatMiniCode(context.Background(), "redirect-code")

	require.ErrorIs(t, err, ErrWechatMiniProviderUnavailable)
	assert.Zero(t, redirectTargetHits)
}

func TestMiniAppAuthErrorCodeMapsProviderFailuresToSafeBFFResponses(t *testing.T) {
	status, code := MiniAppAuthErrorCode(ErrWechatMiniProviderRejected)
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "MINIAPP_CODE_REJECTED", code)
	assert.NotContains(t, code, "session")

	status, code = MiniAppAuthErrorCode(ErrWechatMiniProviderUnavailable)
	assert.Equal(t, http.StatusBadGateway, status)
	assert.Equal(t, "MINIAPP_PROVIDER_UNAVAILABLE", code)
}

func TestExchangeWechatMiniCodeReturnsOnlyDurableSubjectDigest(t *testing.T) {
	useMiniAppExchangeTestConfig(t)
	var logBuffer bytes.Buffer
	common.LogWriterMu.Lock()
	previousErrorWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = previousErrorWriter
		common.LogWriterMu.Unlock()
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "wx-test-app", r.URL.Query().Get("appid"))
		assert.Equal(t, "miniapp-test-secret", r.URL.Query().Get("secret"))
		assert.Equal(t, "one-use-code", r.URL.Query().Get("js_code"))
		assert.Equal(t, "authorization_code", r.URL.Query().Get("grant_type"))
		payload, err := common.Marshal(map[string]any{
			"openid":      "raw-openid-must-not-escape",
			"session_key": "session-key-must-not-escape",
		})
		require.NoError(t, err)
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)
	wechatMiniAppCodeExchangeEndpoint = server.URL

	subject, err := ExchangeWechatMiniCode(context.Background(), "one-use-code")

	require.NoError(t, err)
	assert.Equal(t, "wx-test-app", subject.AppID)
	assert.Len(t, subject.OpenIDHash, 64)
	serialized, err := common.Marshal(subject)
	require.NoError(t, err)
	assert.NotContains(t, string(serialized), "raw-openid-must-not-escape")
	assert.NotContains(t, string(serialized), "session-key-must-not-escape")
	assert.NotContains(t, logBuffer.String(), "raw-openid-must-not-escape")
	assert.NotContains(t, logBuffer.String(), "session-key-must-not-escape")
}

func TestStartMiniAppLoginCreatesFiveMinutePendingTicketForUnboundSubject(t *testing.T) {
	useTestSessionSecret(t)
	setupAuthSessionTestDB(t)
	useMiniAppExchangeTestConfig(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload, err := common.Marshal(map[string]any{"openid": "unbound-openid", "session_key": "never-store-this"})
		require.NoError(t, err)
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)
	wechatMiniAppCodeExchangeEndpoint = server.URL

	result, err := StartMiniAppLogin(context.Background(), "one-use-code", "127.0.0.1", "miniapp-test")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.Session)
	assert.NotEmpty(t, result.PendingTicket)
	assert.Equal(t, MiniAppLoginStatePending, result.State)
	flow, err := model.GetAuthFlow(result.PendingTicket, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeMiniAppPendingIdentity, Provider: miniAppAuthFlowProvider, Intent: model.AuthFlowIntentLogin,
	})
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(5*time.Minute), flow.ExpiresAt, time.Second)
	assert.NotContains(t, flow.Payload, "unbound-openid")
	assert.NotContains(t, flow.Payload, "never-store-this")
	secondResult, err := StartMiniAppLogin(context.Background(), "second-one-use-code", "127.0.0.1", "miniapp-test")
	require.NoError(t, err)
	assert.NotEqual(t, result.PendingTicket, secondResult.PendingTicket)
	var identityCount int64
	require.NoError(t, model.DB.Model(&model.WechatMiniIdentity{}).Count(&identityCount).Error)
	assert.Zero(t, identityCount)
}

func TestStartMiniAppLoginIssuesMiniSessionForBoundSubject(t *testing.T) {
	useTestSessionSecret(t)
	user := setupAuthSessionTestDB(t)
	useMiniAppExchangeTestConfig(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload, err := common.Marshal(map[string]any{"openid": "bound-openid", "session_key": "never-store-this"})
		require.NoError(t, err)
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)
	wechatMiniAppCodeExchangeEndpoint = server.URL
	subject, err := ExchangeWechatMiniCode(context.Background(), "first-code")
	require.NoError(t, err)
	require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
		_, err := model.ClaimWechatMiniIdentityWithTx(tx, subject.AppID, subject.OpenIDHash, user.Id)
		return err
	}))

	result, err := StartMiniAppLogin(context.Background(), "fresh-code", "127.0.0.1", "miniapp-test")

	require.NoError(t, err)
	assert.Equal(t, MiniAppLoginStateAuthenticated, result.State)
	require.NotNil(t, result.Session)
	assert.Empty(t, result.Session.RefreshToken)
	assert.Empty(t, result.PendingTicket)
	assert.Equal(t, "wechat-miniapp", result.Session.Session.LoginMethod)
	var identityCount int64
	require.NoError(t, model.DB.Model(&model.WechatMiniIdentity{}).Count(&identityCount).Error)
	assert.Equal(t, int64(1), identityCount)
}

func TestMiniAppBindingRequiresBrowserSessionAndConsumesFlowsOnce(t *testing.T) {
	useTestSessionSecret(t)
	user := setupAuthSessionTestDB(t)
	useMiniAppExchangeTestConfig(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload, err := common.Marshal(map[string]any{"openid": "binding-openid", "session_key": "never-store-this"})
		require.NoError(t, err)
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)
	wechatMiniAppCodeExchangeEndpoint = server.URL
	pending, err := StartMiniAppLogin(context.Background(), "one-use-code", "127.0.0.1", "miniapp-test")
	require.NoError(t, err)
	login, err := CreateLoginSession(user.Id, "password", "127.0.0.1", "browser-test")
	require.NoError(t, err)
	browserIdentity, err := ParseAccessToken(login.AccessToken)
	require.NoError(t, err)

	binding, err := CreateMiniAppBinding(pending.PendingTicket)

	require.NoError(t, err)
	require.NotEmpty(t, binding.BindingID)
	bindURL, err := url.Parse(binding.BindURL)
	require.NoError(t, err)
	assert.Empty(t, bindURL.RawQuery)
	bindTicket := miniAppBindingTicketFromURL(t, binding.BindURL)
	require.NotEmpty(t, bindTicket)
	assert.NotContains(t, bindURL.RequestURI(), bindTicket)
	_, err = model.GetAuthFlow(pending.PendingTicket, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeMiniAppPendingIdentity, Provider: miniAppAuthFlowProvider, Intent: model.AuthFlowIntentLogin})
	require.NoError(t, err, "creating a bind flow must not consume the pending ticket")
	status, err := GetMiniAppBindingStatusForBinding(pending.PendingTicket, binding.BindingID)
	require.NoError(t, err)
	assert.Equal(t, model.MiniAppBindingStatusPending, status)
	_, err = GetMiniAppBindingStatusForBinding(pending.PendingTicket, "other-binding")
	assert.ErrorIs(t, err, model.ErrAuthFlowInvalid)
	miniAppSession, err := CreateLoginSession(user.Id, "wechat-miniapp", "127.0.0.1", "miniapp-test")
	require.NoError(t, err)
	miniAppIdentity, err := ParseAccessToken(miniAppSession.AccessToken)
	require.NoError(t, err)
	assert.ErrorIs(t, ConfirmMiniAppBinding(bindTicket, miniAppIdentity), ErrMiniAppBrowserSessionRequired)

	err = ConfirmMiniAppBinding(bindTicket, browserIdentity)

	require.NoError(t, err)
	status, err = GetMiniAppBindingStatus(pending.PendingTicket)
	require.NoError(t, err)
	assert.Equal(t, model.MiniAppBindingStatusBound, status)
	assert.ErrorIs(t, ConfirmMiniAppBinding(bindTicket, browserIdentity), model.ErrAuthFlowConsumed)
	var identity model.WechatMiniIdentity
	require.NoError(t, model.DB.First(&identity).Error)
	assert.Equal(t, user.Id, identity.UserID)
}

func TestMiniAppBindingReplayCreatesOnlyOneBindingAndBindFlow(t *testing.T) {
	useTestSessionSecret(t)
	setupAuthSessionTestDB(t)
	useMiniAppExchangeTestConfig(t)
	payload, err := common.Marshal(miniAppPendingIdentityPayload{
		AppID:      "wx-test-app",
		OpenIDHash: strings.Repeat("e", 64),
	})
	require.NoError(t, err)
	pendingTicket, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeMiniAppPendingIdentity,
		Provider:  miniAppAuthFlowProvider,
		Intent:    model.AuthFlowIntentLogin,
		Payload:   string(payload),
		ExpiresAt: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)

	type result struct {
		binding *MiniAppBindingStart
		err     error
	}
	const callers = 4
	start := make(chan struct{})
	results := make(chan result, callers)
	var waitGroup sync.WaitGroup
	for range callers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			binding, err := CreateMiniAppBinding(pendingTicket)
			results <- result{binding: binding, err: err}
		}()
	}
	close(start)
	waitGroup.Wait()
	close(results)

	var successful *MiniAppBindingStart
	for result := range results {
		if result.err == nil {
			require.Nil(t, successful)
			successful = result.binding
			continue
		}
		assert.ErrorIs(t, result.err, model.ErrMiniAppBindingAlreadyExists)
	}
	require.NotNil(t, successful)
	miniAppBindingTicketFromURL(t, successful.BindURL)

	var bindingCount, bindFlowCount int64
	require.NoError(t, model.DB.Model(&model.MiniAppBinding{}).Count(&bindingCount).Error)
	require.NoError(t, model.DB.Model(&model.AuthFlow{}).
		Where("purpose = ?", model.AuthFlowPurposeMiniAppBind).Count(&bindFlowCount).Error)
	assert.Equal(t, int64(1), bindingCount)
	assert.Equal(t, int64(1), bindFlowCount)
	status, err := GetMiniAppBindingStatusForBinding(pendingTicket, successful.BindingID)
	require.NoError(t, err)
	assert.Equal(t, model.MiniAppBindingStatusPending, status)
}

func TestMiniAppBindingStatusDoesNotUseIdentityFallbackWhileBindingIsPending(t *testing.T) {
	useTestSessionSecret(t)
	setupAuthSessionTestDB(t)
	useMiniAppExchangeTestConfig(t)
	owner := &model.User{
		Username: "existing-miniapp-owner", Password: "unused-password-hash", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AffCode: "existing-owner", AuthVersion: 1,
	}
	require.NoError(t, model.DB.Create(owner).Error)
	openIDHash := strings.Repeat("f", 64)
	payload, err := common.Marshal(miniAppPendingIdentityPayload{AppID: "wx-test-app", OpenIDHash: openIDHash})
	require.NoError(t, err)
	pendingTicket, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose: model.AuthFlowPurposeMiniAppPendingIdentity, Provider: miniAppAuthFlowProvider,
		Intent: model.AuthFlowIntentLogin, Payload: string(payload), ExpiresAt: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	binding, err := CreateMiniAppBinding(pendingTicket)
	require.NoError(t, err)
	require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
		_, err := model.ClaimWechatMiniIdentityWithTx(tx, "wx-test-app", openIDHash, owner.Id)
		return err
	}))

	status, err := GetMiniAppBindingStatus(pendingTicket)
	require.NoError(t, err)
	assert.Equal(t, model.MiniAppBindingStatusPending, status)
	status, err = GetMiniAppBindingStatusForBinding(pendingTicket, binding.BindingID)
	require.NoError(t, err)
	assert.Equal(t, model.MiniAppBindingStatusPending, status)
}

func TestMiniAppBindingRejectsCrossBoundTicketWithoutConsumption(t *testing.T) {
	useTestSessionSecret(t)
	user := setupAuthSessionTestDB(t)
	useMiniAppExchangeTestConfig(t)
	makePending := func(openIDHash string) string {
		payload, err := common.Marshal(miniAppPendingIdentityPayload{AppID: "wx-test-app", OpenIDHash: openIDHash})
		require.NoError(t, err)
		ticket, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
			Purpose: model.AuthFlowPurposeMiniAppPendingIdentity, Provider: miniAppAuthFlowProvider,
			Intent: model.AuthFlowIntentLogin, Payload: string(payload), ExpiresAt: time.Now().Add(time.Minute),
		})
		require.NoError(t, err)
		return ticket
	}
	firstTicket := makePending(strings.Repeat("a", 64))
	secondTicket := makePending(strings.Repeat("b", 64))
	binding, err := CreateMiniAppBinding(firstTicket)
	require.NoError(t, err)
	bindingURL, err := url.Parse(binding.BindURL)
	require.NoError(t, err)
	bindFlow, err := model.GetAuthFlow(miniAppBindingTicketFromURL(t, bindingURL.String()), model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeMiniAppBind, Provider: miniAppAuthFlowProvider, Intent: model.AuthFlowIntentBind,
	})
	require.NoError(t, err)
	var crossBoundPayload miniAppBindingFlowPayload
	require.NoError(t, common.UnmarshalJsonStr(bindFlow.Payload, &crossBoundPayload))
	crossBoundPayload.OpenIDHash = strings.Repeat("b", 64)
	crossBoundRaw, err := common.Marshal(crossBoundPayload)
	require.NoError(t, err)
	crossBoundTicket, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose: model.AuthFlowPurposeMiniAppBind, Provider: miniAppAuthFlowProvider, Intent: model.AuthFlowIntentBind,
		Payload: string(crossBoundRaw), ExpiresAt: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	browser, err := CreateLoginSession(user.Id, "password", "127.0.0.1", "browser-test")
	require.NoError(t, err)
	browserIdentity, err := ParseAccessToken(browser.AccessToken)
	require.NoError(t, err)

	err = ConfirmMiniAppBinding(crossBoundTicket, browserIdentity)

	assert.ErrorIs(t, err, model.ErrAuthFlowInvalid)
	firstStatus, err := GetMiniAppBindingStatus(firstTicket)
	require.NoError(t, err)
	assert.Equal(t, model.MiniAppBindingStatusPending, firstStatus)
	secondStatus, err := GetMiniAppBindingStatus(secondTicket)
	require.NoError(t, err)
	assert.Equal(t, model.MiniAppBindingStatusPending, secondStatus)
}

func TestMiniAppBindingRejectsSubjectAlreadyHeldByAnotherUser(t *testing.T) {
	useTestSessionSecret(t)
	user := setupAuthSessionTestDB(t)
	otherUser := &model.User{
		Username: "binding-owner", Password: "unused-password-hash", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AffCode: "owner", AuthVersion: 1,
	}
	require.NoError(t, model.DB.Create(otherUser).Error)
	useMiniAppExchangeTestConfig(t)
	openIDHash := strings.Repeat("d", 64)
	payload, err := common.Marshal(miniAppPendingIdentityPayload{AppID: "wx-test-app", OpenIDHash: openIDHash})
	require.NoError(t, err)
	ticket, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose: model.AuthFlowPurposeMiniAppPendingIdentity, Provider: miniAppAuthFlowProvider,
		Intent: model.AuthFlowIntentLogin, Payload: string(payload), ExpiresAt: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
		_, err := model.ClaimWechatMiniIdentityWithTx(tx, "wx-test-app", openIDHash, otherUser.Id)
		return err
	}))
	binding, err := CreateMiniAppBinding(ticket)
	require.NoError(t, err)
	bindingURL, err := url.Parse(binding.BindURL)
	require.NoError(t, err)
	browser, err := CreateLoginSession(user.Id, "password", "127.0.0.1", "browser-test")
	require.NoError(t, err)
	browserIdentity, err := ParseAccessToken(browser.AccessToken)
	require.NoError(t, err)

	err = ConfirmMiniAppBinding(miniAppBindingTicketFromURL(t, bindingURL.String()), browserIdentity)

	assert.ErrorIs(t, err, model.ErrWechatMiniIdentityAlreadyBound)
	status, err := GetMiniAppBindingStatus(ticket)
	require.NoError(t, err)
	assert.Equal(t, model.MiniAppBindingStatusPending, status)
}

func miniAppBindingTicketFromURL(t *testing.T, rawURL string) string {
	t.Helper()
	bindingURL, err := url.Parse(rawURL)
	require.NoError(t, err)
	require.Empty(t, bindingURL.RawQuery)
	fragment, err := url.ParseQuery(bindingURL.Fragment)
	require.NoError(t, err)
	require.Len(t, fragment, 1)
	ticket := fragment.Get("binding_ticket")
	require.NotEmpty(t, ticket)
	return ticket
}

func TestRegisterMiniAppUserUsesPendingTicketAndNormalPasswordRules(t *testing.T) {
	useTestSessionSecret(t)
	setupAuthSessionTestDB(t)
	useMiniAppExchangeTestConfig(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload, err := common.Marshal(map[string]any{"openid": "registration-openid", "session_key": "never-store-this"})
		require.NoError(t, err)
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)
	wechatMiniAppCodeExchangeEndpoint = server.URL
	pending, err := StartMiniAppLogin(context.Background(), "one-use-code", "127.0.0.1", "miniapp-test")
	require.NoError(t, err)

	bundle, err := RegisterMiniAppUser(pending.PendingTicket, MiniAppRegistration{
		Username: "miniapp-user",
		Password: "valid-password",
	}, "127.0.0.1", "miniapp-test")

	require.NoError(t, err)
	assert.NotEmpty(t, bundle.AccessToken)
	assert.Empty(t, bundle.RefreshToken)
	var user model.User
	require.NoError(t, model.DB.Where("username = ?", "miniapp-user").First(&user).Error)
	assert.True(t, common.ValidatePasswordAndHash("valid-password", user.Password))
	var identity model.WechatMiniIdentity
	require.NoError(t, model.DB.Where("user_id = ?", user.Id).First(&identity).Error)
	status, err := GetMiniAppBindingStatus(pending.PendingTicket)
	require.NoError(t, err)
	assert.Equal(t, model.MiniAppBindingStatusBound, status)
	_, err = RegisterMiniAppUser(pending.PendingTicket, MiniAppRegistration{
		Username: "another-user", Password: "valid-password",
	}, "127.0.0.1", "miniapp-test")
	assert.ErrorIs(t, err, model.ErrAuthFlowConsumed)
}

func TestRegisterMiniAppUserPreservesRegistrationFeatureAndVerificationChecks(t *testing.T) {
	setupAuthSessionTestDB(t)
	useMiniAppExchangeTestConfig(t)
	payload, err := common.Marshal(miniAppPendingIdentityPayload{AppID: "wx-test-app", OpenIDHash: strings.Repeat("c", 64)})
	require.NoError(t, err)
	ticket, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose: model.AuthFlowPurposeMiniAppPendingIdentity, Provider: miniAppAuthFlowProvider,
		Intent: model.AuthFlowIntentLogin, Payload: string(payload), ExpiresAt: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	previousRegisterEnabled := common.RegisterEnabled
	previousPasswordRegisterEnabled := common.PasswordRegisterEnabled
	previousEmailVerificationEnabled := common.EmailVerificationEnabled
	t.Cleanup(func() {
		common.RegisterEnabled = previousRegisterEnabled
		common.PasswordRegisterEnabled = previousPasswordRegisterEnabled
		common.EmailVerificationEnabled = previousEmailVerificationEnabled
	})
	registration := MiniAppRegistration{Username: "feature-check", Password: "valid-password"}

	common.RegisterEnabled = false
	_, err = RegisterMiniAppUser(ticket, registration, "127.0.0.1", "miniapp-test")
	assert.ErrorIs(t, err, ErrMiniAppRegistrationDisabled)
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = false
	_, err = RegisterMiniAppUser(ticket, registration, "127.0.0.1", "miniapp-test")
	assert.ErrorIs(t, err, ErrMiniAppPasswordRegistration)
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = true
	_, err = RegisterMiniAppUser(ticket, registration, "127.0.0.1", "miniapp-test")
	assert.ErrorIs(t, err, ErrMiniAppEmailVerification)
	_, err = model.GetAuthFlow(ticket, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeMiniAppPendingIdentity, Provider: miniAppAuthFlowProvider, Intent: model.AuthFlowIntentLogin,
	})
	assert.NoError(t, err, "registration validation failures must not consume the pending ticket")
}

func TestMiniAppBindingStatusMapsConsumedTicketWithoutIdentityToExpired(t *testing.T) {
	setupAuthSessionTestDB(t)
	useMiniAppExchangeTestConfig(t)
	payload, err := common.Marshal(miniAppPendingIdentityPayload{AppID: "wx-test-app", OpenIDHash: strings.Repeat("e", 64)})
	require.NoError(t, err)
	ticket, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose: model.AuthFlowPurposeMiniAppPendingIdentity, Provider: miniAppAuthFlowProvider,
		Intent: model.AuthFlowIntentLogin, Payload: string(payload), ExpiresAt: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	_, err = model.ConsumeAuthFlow(ticket, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeMiniAppPendingIdentity, Provider: miniAppAuthFlowProvider, Intent: model.AuthFlowIntentLogin,
	})
	require.NoError(t, err)

	status, err := GetMiniAppBindingStatus(ticket)

	require.NoError(t, err)
	assert.Equal(t, model.MiniAppBindingStatusExpired, status)
}

func TestRenewMiniAppLoginRechecksCodeAndOnlyRevokesOwnedMiniSession(t *testing.T) {
	useTestSessionSecret(t)
	user := setupAuthSessionTestDB(t)
	otherUser := &model.User{
		Username: "other-miniapp-user", Password: "unused-password-hash", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AffCode: "other", AuthVersion: 1,
	}
	require.NoError(t, model.DB.Create(otherUser).Error)
	useMiniAppExchangeTestConfig(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload, err := common.Marshal(map[string]any{"openid": "renewal-openid", "session_key": "never-store-this"})
		require.NoError(t, err)
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)
	wechatMiniAppCodeExchangeEndpoint = server.URL
	subject, err := ExchangeWechatMiniCode(context.Background(), "first-code")
	require.NoError(t, err)
	require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
		_, err := model.ClaimWechatMiniIdentityWithTx(tx, subject.AppID, subject.OpenIDHash, user.Id)
		return err
	}))
	prior, err := CreateMiniAppLoginSession(user.Id, "127.0.0.1", "miniapp-test")
	require.NoError(t, err)
	otherPrior, err := CreateMiniAppLoginSession(otherUser.Id, "127.0.0.1", "miniapp-test")
	require.NoError(t, err)

	renewed, err := RenewMiniAppLogin(context.Background(), "fresh-code", prior.Session.SID, "127.0.0.2", "miniapp-test")

	require.NoError(t, err)
	assert.Empty(t, renewed.RefreshToken)
	assert.NotEqual(t, prior.Session.SID, renewed.Session.SID)
	var revoked model.UserSession
	require.NoError(t, model.DB.First(&revoked, "sid = ?", prior.Session.SID).Error)
	assert.Equal(t, model.UserSessionStatusRevoked, revoked.Status)
	_, err = RenewMiniAppLogin(context.Background(), "another-code", otherPrior.Session.SID, "127.0.0.2", "miniapp-test")
	assert.ErrorIs(t, err, ErrMiniAppSessionOwnership)
	var stillActive model.UserSession
	require.NoError(t, model.DB.First(&stillActive, "sid = ?", otherPrior.Session.SID).Error)
	assert.Equal(t, model.UserSessionStatusActive, stillActive.Status)
	browserSession, err := CreateLoginSession(user.Id, "password", "127.0.0.1", "browser-test")
	require.NoError(t, err)
	_, err = RenewMiniAppLogin(context.Background(), "browser-sid-code", browserSession.Session.SID, "127.0.0.2", "miniapp-test")
	assert.ErrorIs(t, err, ErrMiniAppSessionOwnership)
	var browserStillActive model.UserSession
	require.NoError(t, model.DB.First(&browserStillActive, "sid = ?", browserSession.Session.SID).Error)
	assert.Equal(t, model.UserSessionStatusActive, browserStillActive.Status)
}
