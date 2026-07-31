package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAgentConnectReauthenticationRouteRecognizesFreshBrowserSession(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedis, previousSecret := common.RedisEnabled, common.SessionSecret
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled = false
	common.SessionSecret = "agent-connect-route-test-secret"
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled, common.SessionSecret = previousRedis, previousSecret
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.AgentConnectRequest{}))
	user := &model.User{Username: "agent-connect-route", Password: "unused", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1}
	require.NoError(t, db.Create(user).Error)
	session, err := service.CreateLoginSession(user.Id, "password", "127.0.0.1", "test-browser")
	require.NoError(t, err)
	requestID, _, err := model.CreateAgentConnectRequest(model.AgentConnectRequestCreate{ClientKind: "hermes-skill", CodeChallenge: strings.Repeat("a", 43), CodeChallengeMethod: "S256"})
	require.NoError(t, err)
	nonce, _, err := model.BeginAgentConnectReauthentication(requestID)
	require.NoError(t, err)
	require.NoError(t, model.CompleteAgentConnectReauthentication(requestID, nonce, session.Session.SID))

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/agent-connect/requests/"+requestID+"/reauthenticate", nil)
	request.Header.Set("Authorization", "Bearer "+session.AccessToken)
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "reauthentication_required\":false")
}
