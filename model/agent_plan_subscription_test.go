package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreConsumeUserSubscriptionPrioritizesMatchingAgentPlanPackage(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&AgentPlanPackageAllocation{}, &SubscriptionPreConsumeRecord{}))
	t.Cleanup(func() {
		DB.Exec("DELETE FROM agent_plan_package_allocations")
		DB.Exec("DELETE FROM subscription_pre_consume_records")
	})

	now := GetDBTimestamp()
	require.NoError(t, DB.Create(&SubscriptionPlan{Id: 701, Title: "Agent package", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 1_000}).Error)
	require.NoError(t, DB.Create(&SubscriptionPlan{Id: 702, Title: "Legacy", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 1_000}).Error)
	packageSub := &UserSubscription{UserId: 701, PlanId: 701, AmountTotal: 1_000, StartTime: now - 10, EndTime: now + 3_600, Status: "active"}
	legacySub := &UserSubscription{UserId: 701, PlanId: 702, AmountTotal: 1_000, StartTime: now - 10, EndTime: now + 600, Status: "active"}
	require.NoError(t, DB.Create(packageSub).Error)
	require.NoError(t, DB.Create(legacySub).Error)
	require.NoError(t, DB.Create(&AgentPlanPackageAllocation{
		UserSubscriptionId:      packageSub.Id,
		PoolId:                  1,
		AllocationAFPMicros:     1_000_000,
		DisplayMultiplierMicros: 2_000_000,
		ScopeGroup:              "agent",
		ScopeModels:             `["gpt-4.1"]`,
	}).Error)

	result, err := PreConsumeUserSubscription("agent-plan-matching", 701, "gpt-4.1", "agent", 0, 100)

	require.NoError(t, err)
	assert.Equal(t, packageSub.Id, result.UserSubscriptionId)
	var refreshedPackage, refreshedLegacy UserSubscription
	require.NoError(t, DB.First(&refreshedPackage, packageSub.Id).Error)
	require.NoError(t, DB.First(&refreshedLegacy, legacySub.Id).Error)
	assert.EqualValues(t, 100, refreshedPackage.AmountUsed)
	assert.Zero(t, refreshedLegacy.AmountUsed)
}

func TestPreConsumeUserSubscriptionSkipsAgentPlanPackageOutsideScope(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&AgentPlanPackageAllocation{}, &SubscriptionPreConsumeRecord{}))
	t.Cleanup(func() {
		DB.Exec("DELETE FROM agent_plan_package_allocations")
		DB.Exec("DELETE FROM subscription_pre_consume_records")
	})

	now := time.Now().Unix()
	require.NoError(t, DB.Create(&SubscriptionPlan{Id: 703, Title: "Agent package", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 1_000}).Error)
	require.NoError(t, DB.Create(&SubscriptionPlan{Id: 704, Title: "Legacy", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 1_000}).Error)
	packageSub := &UserSubscription{UserId: 702, PlanId: 703, AmountTotal: 1_000, StartTime: now - 10, EndTime: now + 600, Status: "active"}
	legacySub := &UserSubscription{UserId: 702, PlanId: 704, AmountTotal: 1_000, StartTime: now - 10, EndTime: now + 3_600, Status: "active"}
	require.NoError(t, DB.Create(packageSub).Error)
	require.NoError(t, DB.Create(legacySub).Error)
	require.NoError(t, DB.Create(&AgentPlanPackageAllocation{
		UserSubscriptionId:      packageSub.Id,
		PoolId:                  2,
		AllocationAFPMicros:     1_000_000,
		DisplayMultiplierMicros: 1_000_000,
		ScopeGroup:              "agent",
		ScopeModels:             `["gpt-4.1"]`,
	}).Error)

	result, err := PreConsumeUserSubscription("agent-plan-scope-mismatch", 702, "gpt-4.1", "default", 0, 100)

	require.NoError(t, err)
	assert.Equal(t, legacySub.Id, result.UserSubscriptionId)
	var refreshedPackage UserSubscription
	require.NoError(t, DB.First(&refreshedPackage, packageSub.Id).Error)
	assert.Zero(t, refreshedPackage.AmountUsed)
}

func TestAgentPlanPackageReferencesAreUniquePerSubscription(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&AgentPlanPackagePlan{}, &AgentPlanPackageAllocation{}))
	t.Cleanup(func() {
		DB.Exec("DELETE FROM agent_plan_package_plans")
		DB.Exec("DELETE FROM agent_plan_package_allocations")
	})

	firstPlan := &AgentPlanPackagePlan{SubscriptionPlanId: 801, PoolId: 1, AllocationAFPMicros: 1, ScopeGroup: "agent", ScopeModels: `[]`}
	require.NoError(t, DB.Create(firstPlan).Error)
	secondPlan := &AgentPlanPackagePlan{SubscriptionPlanId: 801, PoolId: 2, AllocationAFPMicros: 1, ScopeGroup: "agent", ScopeModels: `[]`}
	require.Error(t, DB.Create(secondPlan).Error)

	firstAllocation := &AgentPlanPackageAllocation{UserSubscriptionId: 801, PoolId: 1, AllocationAFPMicros: 1, DisplayMultiplierMicros: 1, ScopeGroup: "agent", ScopeModels: `[]`}
	require.NoError(t, DB.Create(firstAllocation).Error)
	secondAllocation := &AgentPlanPackageAllocation{UserSubscriptionId: 801, PoolId: 2, AllocationAFPMicros: 1, DisplayMultiplierMicros: 1, ScopeGroup: "agent", ScopeModels: `[]`}
	require.Error(t, DB.Create(secondAllocation).Error)
}
