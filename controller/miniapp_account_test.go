package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMiniAppAccountControllerTest(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
		&model.AgentPlanPackageAllocation{},
	))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
	})
}

func TestMiniAppAccountOverviewReturnsOnlyTheSafeAccountProjection(t *testing.T) {
	setupMiniAppAccountControllerTest(t)
	user := &model.User{
		Username:    "mini-account-user",
		DisplayName: "Mini Account",
		Email:       "member@example.com",
		Password:    "password-placeholder",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		Group:       "vip",
		Quota:       125000,
		AuthVersion: 1,
		AffCode:     "mini-account-aff",
	}
	require.NoError(t, model.DB.Create(user).Error)
	plan := &model.SubscriptionPlan{Title: "Mini Plan", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 9000}
	require.NoError(t, model.DB.Create(plan).Error)
	require.NoError(t, model.DB.Create(&model.UserSubscription{
		UserId: user.Id, PlanId: plan.Id, AmountTotal: 9000, AmountUsed: 1000,
		StartTime: time.Now().Add(-time.Hour).Unix(), EndTime: time.Now().Add(time.Hour).Unix(), Status: "active", Source: "admin",
	}).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/me/overview", nil)
	context.Set("id", user.Id)

	MiniAppAccountOverview(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Username    string `json:"username"`
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
			Quota       struct {
				Balance int    `json:"balance"`
				Unit    string `json:"unit"`
			} `json:"quota"`
			EnabledGroups []string `json:"enabled_groups"`
			Subscriptions []struct {
				PlanTitle string `json:"plan_title"`
				Status    string `json:"status"`
				EndsAt    int64  `json:"ends_at"`
				Quota     struct {
					Remaining int64  `json:"remaining"`
					Unit      string `json:"unit"`
				} `json:"quota"`
			} `json:"subscriptions"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	assert.Equal(t, "mini-account-user", response.Data.Username)
	assert.Equal(t, "Mini Account", response.Data.DisplayName)
	assert.Equal(t, "m***@example.com", response.Data.Email)
	assert.Equal(t, 125000, response.Data.Quota.Balance)
	assert.Equal(t, "quota", response.Data.Quota.Unit)
	assert.Contains(t, response.Data.EnabledGroups, "vip")
	require.Len(t, response.Data.Subscriptions, 1)
	assert.Equal(t, "Mini Plan", response.Data.Subscriptions[0].PlanTitle)
	assert.Equal(t, "active", response.Data.Subscriptions[0].Status)
	assert.Equal(t, int64(8000), response.Data.Subscriptions[0].Quota.Remaining)
	assert.Equal(t, "quota", response.Data.Subscriptions[0].Quota.Unit)

	body := recorder.Body.String()
	assert.NotContains(t, body, "member@example.com")
	assert.NotContains(t, body, "password-placeholder")
	assert.NotContains(t, body, `"role"`)
	assert.NotContains(t, body, `"source"`)
	assert.NotContains(t, body, `"user_id"`)
	assert.NotContains(t, body, `"plan_id"`)
}
