package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/middleware"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMiniAppControllerTest(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	previousSecret := common.SessionSecret
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.SessionSecret = "miniapp-controller-test-secret"
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
		common.SessionSecret = previousSecret
	})
}

func TestMiniAppWechatLoginRejectsUnknownAndEmptyRequestValues(t *testing.T) {
	setupMiniAppControllerTest(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/auth/wechat", MiniAppWechatLogin)

	for _, body := range []string{
		`{}`,
		`{"code":"   "}`,
		`{"code":"valid","unexpected":true}`,
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/auth/wechat", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "MINIAPP_INVALID_REQUEST")
	}
}

func TestMiniAppLogoutRevokesTheAuthenticatedMiniProgramSession(t *testing.T) {
	setupMiniAppControllerTest(t)
	user := &model.User{
		Username: "miniapp-controller-user", Password: "password-placeholder", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "miniapp-controller-aff",
	}
	require.NoError(t, model.DB.Create(user).Error)
	bundle, err := service.CreateMiniAppLoginSession(user.Id, "127.0.0.1", "miniapp-controller-test")
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	c.Set("id", user.Id)
	c.Set("session_id", bundle.Session.SID)

	MiniAppLogout(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	var session model.UserSession
	require.NoError(t, model.DB.First(&session, "sid = ?", bundle.Session.SID).Error)
	assert.Equal(t, model.UserSessionStatusRevoked, session.Status)
}

func TestMiniAppBrowserBindingRejectsAMiniProgramSession(t *testing.T) {
	setupMiniAppControllerTest(t)
	gin.SetMode(gin.TestMode)
	user := &model.User{
		Username: "miniapp-browser-rejection", Password: "password-placeholder", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: "miniapp-browser-aff",
	}
	require.NoError(t, model.DB.Create(user).Error)
	bundle, err := service.CreateMiniAppLoginSession(user.Id, "127.0.0.1", "miniapp-controller-test")
	require.NoError(t, err)
	router := gin.New()
	router.POST("/confirm", middleware.UserAuth(), ConfirmMiniAppBrowserBinding)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/confirm", strings.NewReader(`{"binding_ticket":"bind-ticket"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+bundle.AccessToken)
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusForbidden, response.Code)
	assert.Contains(t, response.Body.String(), "MINIAPP_BROWSER_SESSION_REQUIRED")
}
