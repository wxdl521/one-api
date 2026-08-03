package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMiniAppAccountServiceTest(t *testing.T) {
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

func useMiniAppAccountRedis(t *testing.T) {
	t.Helper()
	previousRedisEnabled := common.RedisEnabled
	previousRedisClient := common.RDB
	previousSyncFrequency := common.SyncFrequency
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	require.NoError(t, redisClient.Ping(t.Context()).Err())
	common.RedisEnabled = true
	common.RDB = redisClient
	common.SyncFrequency = 60
	t.Cleanup(func() {
		_ = redisClient.Close()
		common.RedisEnabled = previousRedisEnabled
		common.RDB = previousRedisClient
		common.SyncFrequency = previousSyncFrequency
	})
}

func TestGetMiniAppAccountOverviewUsesTheCurrentAccountAndActiveSubscriptions(t *testing.T) {
	setupMiniAppAccountServiceTest(t)
	user := &model.User{
		Username: "current-mini-user", DisplayName: "Current User", Email: "current@example.com",
		Password: "password-placeholder", Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		Group: "default", Quota: 67890, AuthVersion: 1, AffCode: "current-mini-aff",
	}
	require.NoError(t, model.DB.Create(user).Error)
	activePlan := &model.SubscriptionPlan{Title: "Active Plan", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1}
	expiredPlan := &model.SubscriptionPlan{Title: "Expired Plan", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1}
	require.NoError(t, model.DB.Create(activePlan).Error)
	require.NoError(t, model.DB.Create(expiredPlan).Error)
	require.NoError(t, model.DB.Create(&model.UserSubscription{
		UserId: user.Id, PlanId: activePlan.Id, AmountTotal: 4000, AmountUsed: 1500,
		StartTime: time.Now().Add(-time.Hour).Unix(), EndTime: time.Now().Add(time.Hour).Unix(), Status: "active", Source: "order",
	}).Error)
	require.NoError(t, model.DB.Create(&model.UserSubscription{
		UserId: user.Id, PlanId: expiredPlan.Id, AmountTotal: 1000,
		StartTime: time.Now().Add(-2 * time.Hour).Unix(), EndTime: time.Now().Add(-time.Hour).Unix(), Status: "expired", Source: "admin",
	}).Error)

	overview, err := GetMiniAppAccountOverview(user.Id)

	require.NoError(t, err)
	assert.Equal(t, "current-mini-user", overview.Username)
	assert.Equal(t, "Current User", overview.DisplayName)
	assert.Equal(t, "c***@example.com", overview.Email)
	assert.Equal(t, 67890, overview.Quota.Balance)
	assert.Equal(t, "quota", overview.Quota.Unit)
	require.Len(t, overview.Subscriptions, 1)
	assert.Equal(t, "Active Plan", overview.Subscriptions[0].PlanTitle)
	assert.Equal(t, int64(2500), overview.Subscriptions[0].Quota.Remaining)
}

func TestGetMiniAppAccountOverviewUsesTheCachedQuotaDuringBatchUpdates(t *testing.T) {
	setupMiniAppAccountServiceTest(t)
	useMiniAppAccountRedis(t)
	previousBatchUpdateEnabled := common.BatchUpdateEnabled
	common.BatchUpdateEnabled = true
	t.Cleanup(func() {
		common.BatchUpdateEnabled = previousBatchUpdateEnabled
	})

	user := &model.User{
		Username: "cached-quota-mini-user", Password: "password-placeholder", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", Quota: 67890, AuthVersion: 1, AffCode: "cached-quota-aff",
	}
	require.NoError(t, model.DB.Create(user).Error)
	cachedUser := user.ToBaseUser()
	cachedUser.Quota = 54321
	require.NoError(t, common.RedisHSetObj(fmt.Sprintf("user:%d", user.Id), cachedUser, time.Minute))

	overview, err := GetMiniAppAccountOverview(user.Id)

	require.NoError(t, err)
	assert.Equal(t, 54321, overview.Quota.Balance)
	assert.Equal(t, 67890, user.Quota, "the database intentionally lags while the batch update is pending")
}
