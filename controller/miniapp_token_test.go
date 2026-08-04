package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMiniAppTokenControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	previousGroups := setting.UserUsableGroups2JSONString()
	previousAllowedModels := common.MiniAppAllowedModels
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Ability{}))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.MiniAppAllowedModels = "gpt-mini"
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
		common.MiniAppAllowedModels = previousAllowedModels
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousGroups))
	})
	return db
}

func createMiniAppTokenContext(t *testing.T, method, path string, body string, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("id", userID)
	return context, recorder
}

func seedMiniAppTokenUser(t *testing.T, db *gorm.DB, username string, quota int) *model.User {
	t.Helper()
	user := &model.User{
		Username: username, Password: "password-placeholder", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", Quota: quota, AuthVersion: 1,
		AffCode: username + "-aff",
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func seedMiniAppToken(t *testing.T, db *gorm.DB, userID int, name, key, source string) *model.Token {
	t.Helper()
	token := &model.Token{
		UserId: userID, Name: name, Key: key, Source: source,
		Status: common.TokenStatusEnabled, CreatedTime: 100, AccessedTime: 101,
		ExpiredTime: 1_800_000_000, UnlimitedQuota: true, ModelLimitsEnabled: true,
		ModelLimits: "gpt-mini", Group: "default", CrossGroupRetry: false,
	}
	require.NoError(t, db.Create(token).Error)
	return token
}

func TestMiniAppCreateTokenReturnsTheRawKeyOnlyInTheCreationResponse(t *testing.T) {
	db := setupMiniAppTokenControllerTest(t)
	user := seedMiniAppTokenUser(t, db, "mini-token-owner", 54321)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "gpt-mini", ChannelId: 1, Enabled: true}).Error)

	context, recorder := createMiniAppTokenContext(
		t,
		http.MethodPost,
		"/api/miniapp/v1/tokens",
		`{"name":" Mobile key ","group":"default","models":["gpt-mini"],"expires_in_days":30}`,
		user.Id,
	)
	MiniAppCreateToken(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			TokenKey string `json:"token_key"`
			Token    struct {
				ID int `json:"id"`
			} `json:"token"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.NotEmpty(t, response.Data.TokenKey)
	require.Positive(t, response.Data.Token.ID)
	assert.Contains(t, recorder.Header().Get("Cache-Control"), "no-store")
	assert.Equal(t, 1, strings.Count(recorder.Body.String(), response.Data.TokenKey))
	assert.NotContains(t, recorder.Body.String(), `"key":`)

	var stored model.Token
	require.NoError(t, db.First(&stored, response.Data.Token.ID).Error)
	assert.Equal(t, response.Data.TokenKey, stored.Key)
	assert.Equal(t, "Mobile key", stored.Name)
	assert.Equal(t, "miniapp", stored.Source)
	assert.True(t, stored.ModelLimitsEnabled)
	assert.False(t, stored.CrossGroupRetry)
	assert.True(t, stored.UnlimitedQuota)
	assert.Equal(t, "gpt-mini", stored.ModelLimits)
	assert.Equal(t, "default", stored.Group)

	var quota int
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", user.Id).Select("quota").Scan(&quota).Error)
	assert.Equal(t, 54321, quota)
}

func TestMiniAppCreateTokenRejectsModelsMissingServerAllowlist(t *testing.T) {
	db := setupMiniAppTokenControllerTest(t)
	configuredModels := common.MiniAppAllowedModels
	common.MiniAppAllowedModels = ""
	t.Cleanup(func() {
		common.MiniAppAllowedModels = configuredModels
	})
	user := seedMiniAppTokenUser(t, db, "mini-token-allowlist-owner", 100)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "gpt-mini", ChannelId: 1, Enabled: true}).Error)

	context, recorder := createMiniAppTokenContext(
		t,
		http.MethodPost,
		"/api/miniapp/v1/tokens",
		`{"name":"Mobile key","group":"default","models":["gpt-mini"],"expires_in_days":30}`,
		user.Id,
	)
	MiniAppCreateToken(context)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "token_key")
	var count int64
	require.NoError(t, db.Model(&model.Token{}).Where("user_id = ?", user.Id).Count(&count).Error)
	assert.Zero(t, count)
}

func TestMiniAppCreateTokenRequiresEnabledModelForTheSelectedGroup(t *testing.T) {
	db := setupMiniAppTokenControllerTest(t)
	configuredModels := common.MiniAppAllowedModels
	common.MiniAppAllowedModels = "gpt-not-available"
	t.Cleanup(func() {
		common.MiniAppAllowedModels = configuredModels
	})
	user := seedMiniAppTokenUser(t, db, "mini-token-group-owner", 100)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "gpt-mini", ChannelId: 1, Enabled: true}).Error)

	context, recorder := createMiniAppTokenContext(
		t,
		http.MethodPost,
		"/api/miniapp/v1/tokens",
		`{"name":"Mobile key","group":"default","models":["gpt-not-available"],"expires_in_days":30}`,
		user.Id,
	)
	MiniAppCreateToken(context)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "token_key")
}

func TestMiniAppCreateTokenRejectsUnconstrainedInputs(t *testing.T) {
	db := setupMiniAppTokenControllerTest(t)
	user := seedMiniAppTokenUser(t, db, "mini-token-boundary-owner", 100)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "gpt-mini", ChannelId: 1, Enabled: true}).Error)

	for _, body := range []string{
		`{"name":"daily","group":"default","models":["gpt-mini"],"expires_in_days":14}`,
		`{"name":"daily","group":"default","models":["gpt-mini","gpt-mini"],"expires_in_days":30}`,
		`{"name":"daily","group":"default","models":["gpt-unknown"],"expires_in_days":30}`,
		`{"name":"` + strings.Repeat("a", 51) + `","group":"default","models":["gpt-mini"],"expires_in_days":30}`,
		`{"name":"daily","group":"default","models":["gpt-mini"],"expires_in_days":30,"unlimited_quota":false}`,
	} {
		context, recorder := createMiniAppTokenContext(t, http.MethodPost, "/api/miniapp/v1/tokens", body, user.Id)
		MiniAppCreateToken(context)
		require.Equal(t, http.StatusBadRequest, recorder.Code, body)
		assert.NotContains(t, recorder.Body.String(), "token_key", body)
	}

	var count int64
	require.NoError(t, db.Model(&model.Token{}).Where("user_id = ?", user.Id).Count(&count).Error)
	assert.Zero(t, count)
}

func TestMiniAppTokenHandlersKeepRawKeysAndOtherTokenSourcesOutOfReach(t *testing.T) {
	db := setupMiniAppTokenControllerTest(t)
	owner := seedMiniAppTokenUser(t, db, "mini-token-owner", 100)
	other := seedMiniAppTokenUser(t, db, "other-mini-token-owner", 100)
	ownedMini := seedMiniAppToken(t, db, owner.Id, "owned mini", "sk-owned-mini-token", "miniapp")
	ownedDashboard := seedMiniAppToken(t, db, owner.Id, "owned dashboard", "sk-dashboard-token", "")
	otherMini := seedMiniAppToken(t, db, other.Id, "other mini", "sk-other-mini-token", "miniapp")

	listContext, listRecorder := createMiniAppTokenContext(t, http.MethodGet, "/api/miniapp/v1/tokens", "", owner.Id)
	MiniAppListTokens(listContext)
	require.Equal(t, http.StatusOK, listRecorder.Code)
	assert.Contains(t, listRecorder.Body.String(), "owned mini")
	assert.NotContains(t, listRecorder.Body.String(), "owned dashboard")
	assert.NotContains(t, listRecorder.Body.String(), "other mini")
	assert.NotContains(t, listRecorder.Body.String(), ownedMini.Key)
	assert.NotContains(t, listRecorder.Body.String(), ownedDashboard.Key)
	assert.NotContains(t, listRecorder.Body.String(), otherMini.Key)
	assert.NotContains(t, listRecorder.Body.String(), `"key"`)

	ownedStatusContext, ownedStatusRecorder := createMiniAppTokenContext(
		t,
		http.MethodPatch,
		"/api/miniapp/v1/tokens/"+strconv.Itoa(ownedMini.Id)+"/status",
		`{"status":2}`,
		owner.Id,
	)
	ownedStatusContext.Params = gin.Params{{Key: "id", Value: strconv.Itoa(ownedMini.Id)}}
	MiniAppUpdateTokenStatus(ownedStatusContext)
	require.Equal(t, http.StatusOK, ownedStatusRecorder.Code)
	assert.NotContains(t, ownedStatusRecorder.Body.String(), ownedMini.Key)

	var ownedAfterStatus model.Token
	require.NoError(t, db.First(&ownedAfterStatus, ownedMini.Id).Error)
	assert.Equal(t, common.TokenStatusDisabled, ownedAfterStatus.Status)

	statusContext, statusRecorder := createMiniAppTokenContext(
		t,
		http.MethodPatch,
		"/api/miniapp/v1/tokens/"+strconv.Itoa(ownedDashboard.Id)+"/status",
		`{"status":2}`,
		owner.Id,
	)
	statusContext.Params = gin.Params{{Key: "id", Value: strconv.Itoa(ownedDashboard.Id)}}
	MiniAppUpdateTokenStatus(statusContext)
	require.NotEqual(t, http.StatusOK, statusRecorder.Code)

	revokeContext, revokeRecorder := createMiniAppTokenContext(
		t,
		http.MethodDelete,
		"/api/miniapp/v1/tokens/"+strconv.Itoa(otherMini.Id),
		"",
		owner.Id,
	)
	revokeContext.Params = gin.Params{{Key: "id", Value: strconv.Itoa(otherMini.Id)}}
	MiniAppRevokeToken(revokeContext)
	require.Equal(t, http.StatusOK, revokeRecorder.Code)

	var dashboardAfter model.Token
	require.NoError(t, db.First(&dashboardAfter, ownedDashboard.Id).Error)
	assert.Equal(t, common.TokenStatusEnabled, dashboardAfter.Status)
	var otherAfter model.Token
	require.NoError(t, db.First(&otherAfter, otherMini.Id).Error)
	assert.Equal(t, otherMini.Key, otherAfter.Key)
	assert.NotContains(t, strings.ToLower(revokeRecorder.Body.String()), strings.ToLower(otherMini.Key))
}
