package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/constant"
	"github.com/QuantumNous/the-one/model"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/QuantumNous/the-one/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAgentPlanQuotaPoolTestDB(t *testing.T) time.Time {
	t.Helper()
	previousDB := model.DB
	previousMainDatabaseType := common.MainDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.LogDatabaseType())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.Channel{},
		&model.User{},
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
		&model.SubscriptionPreConsumeRecord{},
		&model.AgentPlanQuotaPool{},
		&model.AgentPlanPackagePlan{},
		&model.AgentPlanPackageAllocation{},
	))
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetDatabaseTypes(previousMainDatabaseType, common.LogDatabaseType())
		_ = sqlDB.Close()
	})
	return time.Date(2026, 7, 23, 9, 0, 0, 0, time.UTC)
}

func TestNewBillingSessionUsesWalletFallbackAfterMatchedPackageIsExhausted(t *testing.T) {
	now := setupAgentPlanQuotaPoolTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 901, Username: "wallet-fallback", Quota: 100, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.SubscriptionPlan{Id: 901, Title: "Agent package", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 10}).Error)
	packageSub := &model.UserSubscription{UserId: 901, PlanId: 901, AmountTotal: 10, AmountUsed: 10, StartTime: now.Add(-time.Minute).Unix(), EndTime: now.Add(time.Hour).Unix(), Status: "active"}
	require.NoError(t, model.DB.Create(packageSub).Error)
	require.NoError(t, model.DB.Create(&model.AgentPlanPackageAllocation{
		UserSubscriptionId:      packageSub.Id,
		PoolId:                  1,
		AllocationAFPMicros:     AgentPlanAFPMicros,
		DisplayMultiplierMicros: AgentPlanMultiplierMicros,
		ScopeGroup:              "agent",
		ScopeModels:             `["glm-5.2"]`,
		AllowWalletFallback:     true,
	}).Error)

	ctx, _ := gin.CreateTestContext(nil)
	session, apiErr := NewBillingSession(ctx, &relaycommon.RelayInfo{
		UserId:          901,
		UsingGroup:      "agent",
		OriginModelName: "glm-5.2",
		RequestId:       "agent-plan-exhausted-wallet-fallback",
		IsPlayground:    true,
		UserSetting: dto.UserSetting{
			AgentPlanWalletFallbackEnabled: true,
		},
	}, 10)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
	assert.Equal(t, BillingSourceWallet, session.funding.Source())
	var user model.User
	require.NoError(t, model.DB.First(&user, 901).Error)
	assert.Equal(t, 90, user.Quota)
}

func seedAgentPlanQuotaPool(t *testing.T, now time.Time, monthlyAFPMicros int64, multiplierMicros int64) *model.AgentPlanQuotaPool {
	t.Helper()
	channel := &model.Channel{
		Type:               constant.ChannelTypeVolcEngine,
		Key:                "relay-key",
		Name:               "Agent Plan source",
		AgentPlanAccessKey: "access-key",
		AgentPlanSecretKey: "secret-key",
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{AgentPlanUsageEnabled: true})
	require.NoError(t, model.DB.Create(channel).Error)
	pool := &model.AgentPlanQuotaPool{
		SourceChannelId:                   channel.Id,
		Name:                              "V1 pool",
		DisplayMultiplierMicros:           multiplierMicros,
		OfficialMonthlyRemainingAFPMicros: monthlyAFPMicros,
		SyncedAt:                          now.Unix(),
		SyncStatus:                        model.AgentPlanQuotaPoolSyncStatusAvailable,
	}
	require.NoError(t, model.DB.Create(pool).Error)
	return pool
}

func seedAgentPlanPackageSubscription(t *testing.T, now time.Time, poolID int, planID int, allocationAFPMicros int64, amountTotal int64) (*model.AgentPlanPackagePlan, *model.UserSubscription) {
	t.Helper()
	packagePlan := &model.AgentPlanPackagePlan{
		SubscriptionPlanId:  planID,
		PoolId:              poolID,
		AllocationAFPMicros: allocationAFPMicros,
		ScopeGroup:          "pro",
		ScopeModels:         `["gpt-4.1"]`,
		AllowWalletFallback: true,
	}
	require.NoError(t, model.DB.Create(packagePlan).Error)
	subscription := &model.UserSubscription{
		UserId:      planID,
		PlanId:      planID,
		AmountTotal: amountTotal,
		AmountUsed:  0,
		StartTime:   now.Add(-time.Minute).Unix(),
		EndTime:     now.Add(time.Hour).Unix(),
		Status:      "active",
	}
	require.NoError(t, model.DB.Create(subscription).Error)
	return packagePlan, subscription
}

func TestCreateAgentPlanPackageAllocationBooksRawStockAndReturnsDisplayAFP(t *testing.T) {
	now := setupAgentPlanQuotaPoolTestDB(t)
	pool := seedAgentPlanQuotaPool(t, now, 1_000*AgentPlanAFPMicros, 2*AgentPlanMultiplierMicros)
	packagePlan, subscription := seedAgentPlanPackageSubscription(t, now, pool.Id, 11, 1_000*AgentPlanAFPMicros, 2_000)

	booking, err := CreateAgentPlanPackageAllocation(context.Background(), AgentPlanPackageAllocationRequest{
		PackagePlanId:      packagePlan.Id,
		UserSubscriptionId: subscription.Id,
		Now:                now,
	})

	require.NoError(t, err)
	require.NotNil(t, booking.Allocation)
	assert.Equal(t, 1_000*AgentPlanAFPMicros, booking.Allocation.AllocationAFPMicros)
	assert.Equal(t, 2*AgentPlanMultiplierMicros, booking.Allocation.DisplayMultiplierMicros)
	assert.Equal(t, 2_000*AgentPlanAFPMicros, booking.DisplayAllocationAFPMicros)
	assert.Equal(t, 1_000*AgentPlanAFPMicros, booking.Stock.ActiveReservationAFPMicros)
	assert.Zero(t, booking.Stock.SellableAFPMicros)
}

func TestCreateAgentPlanPackageAllocationRejectsStockOverbooking(t *testing.T) {
	now := setupAgentPlanQuotaPoolTestDB(t)
	pool := seedAgentPlanQuotaPool(t, now, 1_500*AgentPlanAFPMicros, AgentPlanMultiplierMicros)
	firstPlan, firstSubscription := seedAgentPlanPackageSubscription(t, now, pool.Id, 21, 1_000*AgentPlanAFPMicros, 1_000)
	secondPlan, secondSubscription := seedAgentPlanPackageSubscription(t, now, pool.Id, 22, 1_000*AgentPlanAFPMicros, 1_000)

	_, err := CreateAgentPlanPackageAllocation(context.Background(), AgentPlanPackageAllocationRequest{
		PackagePlanId:      firstPlan.Id,
		UserSubscriptionId: firstSubscription.Id,
		Now:                now,
	})
	require.NoError(t, err)
	_, err = CreateAgentPlanPackageAllocation(context.Background(), AgentPlanPackageAllocationRequest{
		PackagePlanId:      secondPlan.Id,
		UserSubscriptionId: secondSubscription.Id,
		Now:                now,
	})

	assert.ErrorIs(t, err, ErrAgentPlanQuotaPoolStockInsufficient)
	var allocations []model.AgentPlanPackageAllocation
	require.NoError(t, model.DB.Find(&allocations).Error)
	assert.Len(t, allocations, 1)
}

func TestCreateAgentPlanPackageAllocationRejectsStalePoolSnapshot(t *testing.T) {
	now := setupAgentPlanQuotaPoolTestDB(t)
	pool := seedAgentPlanQuotaPool(t, now.Add(-61*time.Second), 1_000*AgentPlanAFPMicros, AgentPlanMultiplierMicros)
	packagePlan, subscription := seedAgentPlanPackageSubscription(t, now, pool.Id, 31, 1_000*AgentPlanAFPMicros, 1_000)

	previousFetch := fetchAgentPlanUsageForQuotaPool
	fetchAgentPlanUsageForQuotaPool = func(context.Context, *model.Channel) (AgentPlanUsage, int64, bool, error) {
		return AgentPlanUsage{}, 0, true, errors.New("upstream unavailable")
	}
	t.Cleanup(func() { fetchAgentPlanUsageForQuotaPool = previousFetch })

	_, err := CreateAgentPlanPackageAllocation(context.Background(), AgentPlanPackageAllocationRequest{
		PackagePlanId:      packagePlan.Id,
		UserSubscriptionId: subscription.Id,
		Now:                now,
	})

	assert.ErrorIs(t, err, ErrAgentPlanQuotaPoolSnapshotStale)
}

func TestCreateAgentPlanPackageAllocationRejectsUnlimitedSubscription(t *testing.T) {
	now := setupAgentPlanQuotaPoolTestDB(t)
	pool := seedAgentPlanQuotaPool(t, now, 1_000*AgentPlanAFPMicros, AgentPlanMultiplierMicros)
	packagePlan, subscription := seedAgentPlanPackageSubscription(t, now, pool.Id, 41, 1_000*AgentPlanAFPMicros, 0)

	_, err := CreateAgentPlanPackageAllocation(context.Background(), AgentPlanPackageAllocationRequest{
		PackagePlanId:      packagePlan.Id,
		UserSubscriptionId: subscription.Id,
		Now:                now,
	})

	assert.ErrorIs(t, err, ErrAgentPlanPackageAllocationInvalid)
	var count int64
	require.NoError(t, model.DB.Model(&model.AgentPlanPackageAllocation{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestAdminAssignSubscriptionCreatesPackageAllocationWithDisplayQuota(t *testing.T) {
	now := setupAgentPlanQuotaPoolTestDB(t)
	pool := seedAgentPlanQuotaPool(t, now, 2_000*AgentPlanAFPMicros, 2*AgentPlanMultiplierMicros)
	plan := &model.SubscriptionPlan{Title: "Assigned package", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 1, QuotaResetPeriod: model.SubscriptionResetNever}
	require.NoError(t, model.DB.Create(plan).Error)
	packagePlan := &model.AgentPlanPackagePlan{SubscriptionPlanId: plan.Id, PoolId: pool.Id, AllocationAFPMicros: 1_000 * AgentPlanAFPMicros, ScopeGroup: "pro", ScopeModels: `["gpt-4.1"]`}
	require.NoError(t, model.DB.Create(packagePlan).Error)
	previousNow := agentPlanQuotaPoolNow
	agentPlanQuotaPoolNow = func() time.Time { return now }
	t.Cleanup(func() { agentPlanQuotaPoolNow = previousNow })

	_, err := AdminAssignSubscription(context.Background(), 901, plan.Id, "")

	require.NoError(t, err)
	var subscription model.UserSubscription
	require.NoError(t, model.DB.Where("user_id = ?", 901).First(&subscription).Error)
	assert.EqualValues(t, 2*common.QuotaPerUnit*1_000, subscription.AmountTotal)
	var allocation model.AgentPlanPackageAllocation
	require.NoError(t, model.DB.Where("user_subscription_id = ?", subscription.Id).First(&allocation).Error)
	assert.Equal(t, pool.Id, allocation.PoolId)
	assert.Equal(t, packagePlan.AllowWalletFallback, allocation.AllowWalletFallback)
}

func TestAdminAssignSubscriptionRollsBackWhenPoolSourceBecomesIneligibleDuringBooking(t *testing.T) {
	now := setupAgentPlanQuotaPoolTestDB(t)
	pool := seedAgentPlanQuotaPool(t, now, 2_000*AgentPlanAFPMicros, AgentPlanMultiplierMicros)
	plan := &model.SubscriptionPlan{Title: "Assigned package", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 1, QuotaResetPeriod: model.SubscriptionResetNever}
	require.NoError(t, model.DB.Create(plan).Error)
	require.NoError(t, model.DB.Create(&model.AgentPlanPackagePlan{
		SubscriptionPlanId:  plan.Id,
		PoolId:              pool.Id,
		AllocationAFPMicros: 1_000 * AgentPlanAFPMicros,
		ScopeGroup:          "pro",
		ScopeModels:         `["gpt-4.1"]`,
	}).Error)
	previousNow := agentPlanQuotaPoolNow
	agentPlanQuotaPoolNow = func() time.Time { return now }
	t.Cleanup(func() { agentPlanQuotaPoolNow = previousNow })

	const callbackName = "agent-plan-test-disable-source-before-allocation"
	sourceDisabled := false
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*model.UserSubscription); !ok {
			return
		}
		err := tx.Session(&gorm.Session{NewDB: true}).Model(&model.Channel{}).
			Where("id = ?", pool.SourceChannelId).Update("settings", `{}`).Error
		if err != nil {
			tx.AddError(err)
			return
		}
		sourceDisabled = true
	}))
	t.Cleanup(func() {
		require.NoError(t, model.DB.Callback().Create().Remove(callbackName))
	})

	_, err := AdminAssignSubscription(context.Background(), 902, plan.Id, "")

	require.True(t, sourceDisabled)
	require.Error(t, err)
	var subscriptions int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("user_id = ?", 902).Count(&subscriptions).Error)
	require.Zero(t, subscriptions)
	var allocations int64
	require.NoError(t, model.DB.Model(&model.AgentPlanPackageAllocation{}).Count(&allocations).Error)
	require.Zero(t, allocations)
}

func TestCreateAgentPlanPackageAllocationValidatesFreshPoolSource(t *testing.T) {
	now := setupAgentPlanQuotaPoolTestDB(t)
	pool := seedAgentPlanQuotaPool(t, now, 1_000*AgentPlanAFPMicros, AgentPlanMultiplierMicros)
	packagePlan, subscription := seedAgentPlanPackageSubscription(t, now, pool.Id, 51, 1_000*AgentPlanAFPMicros, 1_000)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", pool.SourceChannelId).Update("settings", `{}`).Error)

	_, err := CreateAgentPlanPackageAllocation(context.Background(), AgentPlanPackageAllocationRequest{
		PackagePlanId:      packagePlan.Id,
		UserSubscriptionId: subscription.Id,
		Now:                now,
	})

	assert.Error(t, err)
	var count int64
	require.NoError(t, model.DB.Model(&model.AgentPlanPackageAllocation{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestCreateAgentPlanPackageAllocationUsesCurrentTimeAfterSync(t *testing.T) {
	now := setupAgentPlanQuotaPoolTestDB(t)
	pool := seedAgentPlanQuotaPool(t, now.Add(-61*time.Second), 1_000*AgentPlanAFPMicros, AgentPlanMultiplierMicros)
	packagePlan, subscription := seedAgentPlanPackageSubscription(t, now, pool.Id, 61, 1_000*AgentPlanAFPMicros, 1_000)

	previousNow := agentPlanQuotaPoolNow
	agentPlanQuotaPoolNow = func() time.Time { return now.Add(2 * time.Second) }
	t.Cleanup(func() { agentPlanQuotaPoolNow = previousNow })
	previousFetch := fetchAgentPlanUsageForQuotaPool
	fetchAgentPlanUsageForQuotaPool = func(context.Context, *model.Channel) (AgentPlanUsage, int64, bool, error) {
		return AgentPlanUsage{
			FiveHour: AgentPlanUsageWindow{Remaining: 1_000},
			Weekly:   AgentPlanUsageWindow{Remaining: 1_000},
			Monthly:  AgentPlanUsageWindow{Remaining: 1_000},
		}, now.Add(time.Second).Unix(), false, nil
	}
	t.Cleanup(func() { fetchAgentPlanUsageForQuotaPool = previousFetch })

	booking, err := CreateAgentPlanPackageAllocation(context.Background(), AgentPlanPackageAllocationRequest{
		PackagePlanId:      packagePlan.Id,
		UserSubscriptionId: subscription.Id,
		Now:                now,
	})

	require.NoError(t, err)
	assert.NotNil(t, booking.Allocation)
}

func TestAgentPlanUsageAFPMicrosRejectsRoundedInt64Overflow(t *testing.T) {
	_, err := agentPlanUsageAFPMicros(float64(math.MaxInt64) / float64(AgentPlanAFPMicros))

	assert.Error(t, err)
}
