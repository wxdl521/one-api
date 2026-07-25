package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// AdminAssignSubscription creates ordinary subscriptions through the existing
// model path and package subscriptions together with their allocation snapshot.
func AdminAssignSubscription(ctx context.Context, userID int, planID int, sourceNote string) (string, error) {
	if userID <= 0 || planID <= 0 {
		return "", errors.New("invalid userId or planId")
	}
	packagePlan, err := model.GetAgentPlanPackagePlanBySubscriptionPlanID(planID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.AdminBindSubscription(userID, planID, sourceNote)
		}
		return "", err
	}
	if packagePlan.AllocationAFPMicros <= 0 {
		return "", ErrAgentPlanPackageAllocationInvalid
	}
	now := agentPlanQuotaPoolNow()
	freshness, err := PrepareAgentPlanQuotaPoolAllocation(ctx, packagePlan.PoolId, now)
	if err != nil {
		return "", err
	}
	groupChanged := false
	var assignedPlan model.SubscriptionPlan
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		plan, err := model.GetSubscriptionPlanForUpdate(tx, planID)
		if err != nil {
			return err
		}
		lockedPackagePlan, err := model.GetAgentPlanPackagePlanForUpdate(tx, packagePlan.Id)
		if err != nil {
			return err
		}
		if lockedPackagePlan.PoolId != freshness.PoolId || lockedPackagePlan.AllocationAFPMicros <= 0 {
			return ErrAgentPlanPackageAllocationInvalid
		}
		pool, err := model.GetAgentPlanQuotaPoolForUpdate(tx, lockedPackagePlan.PoolId)
		if err != nil {
			return err
		}
		if pool.DisplayMultiplierMicros <= 0 || pool.SyncedAt < freshness.SyncedAt {
			return ErrAgentPlanPackageAllocationInvalid
		}
		amount, err := common.QuotaFromFloatStrict(
			float64(lockedPackagePlan.AllocationAFPMicros) * float64(pool.DisplayMultiplierMicros) /
				float64(AgentPlanAFPMicros*AgentPlanMultiplierMicros) * common.QuotaPerUnit,
		)
		if err != nil {
			return err
		}
		if amount <= 0 {
			return ErrAgentPlanPackageAllocationInvalid
		}
		planCopy := *plan
		planCopy.TotalAmount = int64(amount)
		planCopy.QuotaResetPeriod = model.SubscriptionResetNever
		planCopy.QuotaResetCustomSeconds = 0
		assignedPlan = planCopy
		subscription, createErr := model.CreateUserSubscriptionFromPlanTx(tx, userID, &planCopy, "admin")
		if createErr != nil {
			return createErr
		}
		groupChanged = subscription.PrevUserGroup != ""
		_, createErr = CreateAgentPlanPackageAllocation(ctx, AgentPlanPackageAllocationRequest{
			PackagePlanId:      packagePlan.Id,
			UserSubscriptionId: subscription.Id,
			Tx:                 tx,
			Freshness:          freshness,
		})
		return createErr
	})
	if err != nil {
		return "", err
	}
	if groupChanged {
		if err := model.RefreshUserGroupCache(userID); err != nil {
			common.SysError(fmt.Sprintf("failed to refresh user group cache after agent plan package assignment for user %d: %v", userID, err))
		}
		return fmt.Sprintf("用户分组将升级到 %s", assignedPlan.UpgradeGroup), nil
	}
	return "", nil
}
