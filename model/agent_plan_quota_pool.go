package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

const (
	AgentPlanQuotaPoolSyncStatusAvailable = "available"
	AgentPlanQuotaPoolSyncStatusError     = "error"
)

var (
	ErrAgentPlanQuotaPoolSourceInvalid = errors.New("agent plan quota pool source is invalid")
)

// AgentPlanQuotaPool stores the inventory snapshot for one official Agent Plan source channel.
// AFP values use one million micro-AFP units per AFP.
type AgentPlanQuotaPool struct {
	Id int `json:"id"`

	SourceChannelId         int    `json:"source_channel_id" gorm:"type:int;not null;uniqueIndex:idx_agent_plan_quota_pool_source_channel"`
	Name                    string `json:"name" gorm:"type:varchar(128);not null"`
	DisplayMultiplierMicros int64  `json:"display_multiplier_micros" gorm:"type:bigint;not null"`

	OfficialMonthlyRemainingAFPMicros int64 `json:"official_monthly_remaining_afp_micros" gorm:"type:bigint;not null"`
	FiveHourRemainingAFPMicros        int64 `json:"five_hour_remaining_afp_micros" gorm:"type:bigint;not null"`
	FiveHourResetAt                   int64 `json:"five_hour_reset_at" gorm:"type:bigint;not null"`
	WeeklyRemainingAFPMicros          int64 `json:"weekly_remaining_afp_micros" gorm:"type:bigint;not null"`
	WeeklyResetAt                     int64 `json:"weekly_reset_at" gorm:"type:bigint;not null"`
	MonthlyResetAt                    int64 `json:"monthly_reset_at" gorm:"type:bigint;not null"`

	SyncedAt   int64  `json:"synced_at" gorm:"type:bigint;not null;index"`
	SyncStatus string `json:"sync_status" gorm:"type:varchar(32);not null"`
	SyncError  string `json:"sync_error" gorm:"type:text"`
	CreatedAt  int64  `json:"created_at" gorm:"type:bigint;not null"`
	UpdatedAt  int64  `json:"updated_at" gorm:"type:bigint;not null"`
}

func (p *AgentPlanQuotaPool) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *AgentPlanQuotaPool) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

// AgentPlanPackagePlan connects a subscription plan to a quota pool without
// changing the existing subscription billing schema.
type AgentPlanPackagePlan struct {
	Id int `json:"id"`

	SubscriptionPlanId  int    `json:"subscription_plan_id" gorm:"type:int;not null;uniqueIndex:idx_agent_plan_package_plan_subscription"`
	PoolId              int    `json:"pool_id" gorm:"type:int;not null;index"`
	AllocationAFPMicros int64  `json:"allocation_afp_micros" gorm:"type:bigint;not null"`
	ScopeGroup          string `json:"scope_group" gorm:"type:varchar(64);not null"`
	ScopeModels         string `json:"scope_models" gorm:"type:text;not null"`
	AllowWalletFallback bool   `json:"allow_wallet_fallback"`

	CreatedAt int64 `json:"created_at" gorm:"type:bigint;not null"`
	UpdatedAt int64 `json:"updated_at" gorm:"type:bigint;not null"`
}

func (p *AgentPlanPackagePlan) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *AgentPlanPackagePlan) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

// AgentPlanPackageAllocation is the immutable pool allocation snapshot for a
// concrete user subscription.
type AgentPlanPackageAllocation struct {
	Id int `json:"id"`

	UserSubscriptionId      int    `json:"user_subscription_id" gorm:"type:int;not null;uniqueIndex:idx_agent_plan_allocation_subscription"`
	PoolId                  int    `json:"pool_id" gorm:"type:int;not null;index"`
	AllocationAFPMicros     int64  `json:"allocation_afp_micros" gorm:"type:bigint;not null"`
	DisplayMultiplierMicros int64  `json:"display_multiplier_micros" gorm:"type:bigint;not null"`
	ScopeGroup              string `json:"scope_group" gorm:"type:varchar(64);not null"`
	ScopeModels             string `json:"scope_models" gorm:"type:text;not null"`
	AllowWalletFallback     bool   `json:"allow_wallet_fallback"`
	CreatedAt               int64  `json:"created_at" gorm:"type:bigint;not null"`
	UpdatedAt               int64  `json:"updated_at" gorm:"type:bigint;not null"`
}

func (a *AgentPlanPackageAllocation) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	a.CreatedAt = now
	a.UpdatedAt = now
	return nil
}

func (a *AgentPlanPackageAllocation) BeforeUpdate(tx *gorm.DB) error {
	a.UpdatedAt = common.GetTimestamp()
	return nil
}

// ValidateAgentPlanQuotaPoolSource verifies that a channel may back a quota pool.
// It keeps the source credentials private to service code and callers.
func ValidateAgentPlanQuotaPoolSource(channel *Channel) error {
	if channel == nil || channel.Id <= 0 {
		return ErrAgentPlanQuotaPoolSourceInvalid
	}
	if channel.Type != constant.ChannelTypeVolcEngine && channel.Type != constant.ChannelTypeAdvancedCustom {
		return fmt.Errorf("%w: channel type is not supported", ErrAgentPlanQuotaPoolSourceInvalid)
	}
	if channel.ChannelInfo.IsMultiKey {
		return fmt.Errorf("%w: multi-key channels are not supported", ErrAgentPlanQuotaPoolSourceInvalid)
	}
	if !channel.GetOtherSettings().AgentPlanUsageEnabled {
		return fmt.Errorf("%w: agent plan usage is disabled", ErrAgentPlanQuotaPoolSourceInvalid)
	}
	if strings.TrimSpace(channel.AgentPlanAccessKey) == "" || strings.TrimSpace(channel.AgentPlanSecretKey) == "" {
		return fmt.Errorf("%w: agent plan credentials are required", ErrAgentPlanQuotaPoolSourceInvalid)
	}
	return nil
}

func GetAgentPlanQuotaPoolById(id int) (*AgentPlanQuotaPool, error) {
	if id <= 0 {
		return nil, errors.New("invalid agent plan quota pool id")
	}
	pool := &AgentPlanQuotaPool{}
	if err := DB.Where("id = ?", id).First(pool).Error; err != nil {
		return nil, err
	}
	return pool, nil
}

func GetAgentPlanQuotaPoolForUpdate(tx *gorm.DB, id int) (*AgentPlanQuotaPool, error) {
	if tx == nil || id <= 0 {
		return nil, errors.New("invalid agent plan quota pool lock arguments")
	}
	pool := &AgentPlanQuotaPool{}
	if err := lockForUpdate(tx).Where("id = ?", id).First(pool).Error; err != nil {
		return nil, err
	}
	return pool, nil
}

func GetAgentPlanPackagePlanForUpdate(tx *gorm.DB, id int) (*AgentPlanPackagePlan, error) {
	if tx == nil || id <= 0 {
		return nil, errors.New("invalid agent plan package plan lock arguments")
	}
	packagePlan := &AgentPlanPackagePlan{}
	if err := lockForUpdate(tx).Where("id = ?", id).First(packagePlan).Error; err != nil {
		return nil, err
	}
	return packagePlan, nil
}

func GetAgentPlanPackagePlanBySubscriptionPlanID(subscriptionPlanID int) (*AgentPlanPackagePlan, error) {
	return GetAgentPlanPackagePlanBySubscriptionPlanIDTx(nil, subscriptionPlanID)
}

func GetAgentPlanPackagePlanBySubscriptionPlanIDTx(tx *gorm.DB, subscriptionPlanID int) (*AgentPlanPackagePlan, error) {
	if subscriptionPlanID <= 0 {
		return nil, errors.New("invalid subscription plan id")
	}
	packagePlan := &AgentPlanPackagePlan{}
	query := DB
	if tx != nil {
		query = tx
	}
	if err := query.Where("subscription_plan_id = ?", subscriptionPlanID).First(packagePlan).Error; err != nil {
		return nil, err
	}
	return packagePlan, nil
}

func IsAgentPlanPackageSubscriptionPlan(subscriptionPlanID int) (bool, error) {
	return IsAgentPlanPackageSubscriptionPlanTx(nil, subscriptionPlanID)
}

func IsAgentPlanPackageSubscriptionPlanTx(tx *gorm.DB, subscriptionPlanID int) (bool, error) {
	if subscriptionPlanID <= 0 {
		return false, errors.New("invalid subscription plan id")
	}
	var count int64
	query := DB
	if tx != nil {
		query = tx
	}
	if err := query.Model(&AgentPlanPackagePlan{}).Where("subscription_plan_id = ?", subscriptionPlanID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func AgentPlanQuotaPoolHasReferencesTx(tx *gorm.DB, poolID int) (bool, error) {
	if tx == nil || poolID <= 0 {
		return false, errors.New("invalid agent plan quota pool reference arguments")
	}
	var count int64
	if err := tx.Model(&AgentPlanPackagePlan{}).Where("pool_id = ?", poolID).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	if err := tx.Model(&AgentPlanPackageAllocation{}).Where("pool_id = ?", poolID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// DeleteAgentPlanQuotaPoolIfUnreferenced serializes the reference check and
// delete under the pool row lock so an issuance cannot race a deletion.
func DeleteAgentPlanQuotaPoolIfUnreferenced(poolID int) (bool, error) {
	if poolID <= 0 {
		return false, errors.New("invalid agent plan quota pool id")
	}
	deleted := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		if _, err := GetAgentPlanQuotaPoolForUpdate(tx, poolID); err != nil {
			return err
		}
		hasReferences, err := AgentPlanQuotaPoolHasReferencesTx(tx, poolID)
		if err != nil {
			return err
		}
		if hasReferences {
			return nil
		}
		result := tx.Delete(&AgentPlanQuotaPool{}, poolID)
		if result.Error != nil {
			return result.Error
		}
		deleted = result.RowsAffected > 0
		return nil
	})
	return deleted, err
}

func GetUserSubscriptionForUpdate(tx *gorm.DB, id int) (*UserSubscription, error) {
	if tx == nil || id <= 0 {
		return nil, errors.New("invalid user subscription lock arguments")
	}
	subscription := &UserSubscription{}
	if err := lockForUpdate(tx).Where("id = ?", id).First(subscription).Error; err != nil {
		return nil, err
	}
	return subscription, nil
}

func GetAgentPlanPackageAllocationForUpdate(tx *gorm.DB, userSubscriptionId int, poolId int) (*AgentPlanPackageAllocation, error) {
	if tx == nil || userSubscriptionId <= 0 || poolId <= 0 {
		return nil, errors.New("invalid agent plan allocation lock arguments")
	}
	allocation := &AgentPlanPackageAllocation{}
	if err := lockForUpdate(tx).
		Where("user_subscription_id = ? AND pool_id = ?", userSubscriptionId, poolId).
		First(allocation).Error; err != nil {
		return nil, err
	}
	return allocation, nil
}

func ListAgentPlanPackageAllocationsForUpdate(tx *gorm.DB, poolId int) ([]AgentPlanPackageAllocation, error) {
	if tx == nil || poolId <= 0 {
		return nil, errors.New("invalid agent plan allocation list lock arguments")
	}
	allocations := make([]AgentPlanPackageAllocation, 0)
	if err := lockForUpdate(tx).Where("pool_id = ?", poolId).Find(&allocations).Error; err != nil {
		return nil, err
	}
	return allocations, nil
}

func ListUserSubscriptionsForUpdate(tx *gorm.DB, ids []int) ([]UserSubscription, error) {
	if tx == nil {
		return nil, errors.New("transaction is required")
	}
	if len(ids) == 0 {
		return []UserSubscription{}, nil
	}
	subscriptions := make([]UserSubscription, 0, len(ids))
	if err := lockForUpdate(tx).Where("id IN ?", ids).Find(&subscriptions).Error; err != nil {
		return nil, err
	}
	return subscriptions, nil
}
