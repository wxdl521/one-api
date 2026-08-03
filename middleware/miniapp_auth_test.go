package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMiniAppAuthMiddlewareTest(t *testing.T) {
	t.Helper()
	setupDashboardAuthMiddlewareTest(t)
	previousEnabled, previousTextEnabled := common.GetMiniProgramFeatureFlags()
	previousAppID := common.WeChatMiniAppAppID
	previousAppSecret := common.WeChatMiniAppAppSecret
	previousSubjectHMACKey := common.WeChatMiniAppSubjectHMACKey
	previousBindURL := common.MiniAppBindWebBaseURL
	previousTimeout := common.MiniAppHTTPTimeout

	common.MiniProgramEnabled = true
	common.MiniProgramTextTestEnabled = false
	common.WeChatMiniAppAppID = "wx-miniapp-middleware-test"
	common.WeChatMiniAppAppSecret = "miniapp-middleware-secret"
	common.WeChatMiniAppSubjectHMACKey = "miniapp-middleware-subject-key"
	common.MiniAppBindWebBaseURL = "https://console.example.com/miniapp-bind"
	common.MiniAppHTTPTimeout = time.Second
	t.Cleanup(func() {
		common.MiniProgramEnabled = previousEnabled
		common.MiniProgramTextTestEnabled = previousTextEnabled
		common.WeChatMiniAppAppID = previousAppID
		common.WeChatMiniAppAppSecret = previousAppSecret
		common.WeChatMiniAppSubjectHMACKey = previousSubjectHMACKey
		common.MiniAppBindWebBaseURL = previousBindURL
		common.MiniAppHTTPTimeout = previousTimeout
	})
}

func createMiniAppAuthSession(t *testing.T, username, loginMethod string) (*model.User, *service.AuthBundle) {
	t.Helper()
	user := &model.User{
		Username:    username,
		Password:    "password-placeholder",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
		AffCode:     "miniapp-auth-" + username,
	}
	require.NoError(t, model.DB.Create(user).Error)
	if loginMethod == "wechat-miniapp" {
		bundle, err := service.CreateMiniAppLoginSession(user.Id, "127.0.0.1", "miniapp-test")
		require.NoError(t, err)
		return user, bundle
	}
	bundle, err := service.CreateLoginSession(user.Id, loginMethod, "127.0.0.1", "browser-test")
	require.NoError(t, err)
	return user, bundle
}

func TestMiniAppAuthAcceptsOnlyLiveMiniProgramSessions(t *testing.T) {
	setupMiniAppAuthMiddlewareTest(t)
	gin.SetMode(gin.TestMode)

	miniUser, miniBundle := createMiniAppAuthSession(t, "miniapp-session-user", "wechat-miniapp")
	_, passwordBundle := createMiniAppAuthSession(t, "password-session-user", "password")
	_, oauthBundle := createMiniAppAuthSession(t, "oauth-session-user", "oauth:github")
	createMiddlewarePATUser(t, "miniapp-pat-user", "miniapp.pat.token")

	router := gin.New()
	router.GET("/protected", MiniAppAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"id":         c.GetInt("id"),
			"session_id": c.GetString("session_id"),
		})
	})

	tests := []struct {
		name          string
		token         string
		wantStatus    int
		wantCode      string
		wantHandlerID int
	}{
		{name: "mini program session", token: miniBundle.AccessToken, wantStatus: http.StatusOK, wantHandlerID: miniUser.Id},
		{name: "password browser session", token: passwordBundle.AccessToken, wantStatus: http.StatusUnauthorized, wantCode: "MINIAPP_SESSION_INVALID"},
		{name: "oauth browser session", token: oauthBundle.AccessToken, wantStatus: http.StatusUnauthorized, wantCode: "MINIAPP_SESSION_INVALID"},
		{name: "personal access token", token: "miniapp.pat.token", wantStatus: http.StatusUnauthorized, wantCode: "MINIAPP_SESSION_INVALID"},
		{name: "expired access token", token: issueExpiredDashboardAccessToken(t, service.AuthIdentity{
			UserID: miniUser.Id, SessionID: miniBundle.Session.SID, UserAuthVersion: 1, SessionVersion: 1,
		}), wantStatus: http.StatusUnauthorized, wantCode: "MINIAPP_SESSION_INVALID"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			request.Header.Set("Authorization", "Bearer "+test.token)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			require.Equal(t, test.wantStatus, response.Code)
			if test.wantCode != "" {
				assert.Contains(t, response.Body.String(), test.wantCode)
				return
			}
			var body struct {
				ID        int    `json:"id"`
				SessionID string `json:"session_id"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
			assert.Equal(t, test.wantHandlerID, body.ID)
			assert.Equal(t, miniBundle.Session.SID, body.SessionID)
		})
	}

	revoked, err := model.RevokeUserSession(miniUser.Id, miniBundle.Session.SID, "test_revoked")
	require.NoError(t, err)
	require.True(t, revoked)
	revokedRequest := httptest.NewRequest(http.MethodGet, "/protected", nil)
	revokedRequest.Header.Set("Authorization", "Bearer "+miniBundle.AccessToken)
	revokedResponse := httptest.NewRecorder()
	router.ServeHTTP(revokedResponse, revokedRequest)
	require.Equal(t, http.StatusUnauthorized, revokedResponse.Code)
	assert.Contains(t, revokedResponse.Body.String(), "MINIAPP_SESSION_INVALID")
}

func TestMiniAppAuthRejectsRequestsWhenTheFeatureIsDisabled(t *testing.T) {
	setupMiniAppAuthMiddlewareTest(t)
	common.MiniProgramEnabled = false
	router := gin.New()
	router.GET("/protected", MiniAppAuth(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/protected", nil))

	require.Equal(t, http.StatusNotFound, response.Code)
	assert.Contains(t, response.Body.String(), "MINIAPP_DISABLED")
}

func TestMiniAppBindingRequestBodyLimitRejectsOversizedRequestsBeforeHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlerCalled := false
	router.POST("/confirm", MiniAppBindingRequestBodyLimit(), func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusNoContent)
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/confirm", strings.NewReader(strings.Repeat("x", miniAppBindingRequestBodyMaxBytes+1)))
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	assert.False(t, handlerCalled)
	assert.Contains(t, response.Body.String(), "MINIAPP_REQUEST_TOO_LARGE")
	assert.NotContains(t, response.Body.String(), "x")
}
