package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/QuantumNous/the-one/model"
	"gorm.io/gorm"
)

const (
	// AgentPlanAFPMicros is the persisted precision for official Agent Plan AFP.
	AgentPlanAFPMicros int64 = 1_000_000
	// AgentPlanMultiplierMicros is one unit of a caller-visible AFP multiplier.
	AgentPlanMultiplierMicros int64 = 1_000_000

	agentPlanQuotaPoolSnapshotMaxAge = time.Minute
	agentPlanQuotaPoolUsageEndpoint  = "https://open.volcengineapi.com"
)

var (
	ErrAgentPlanQuotaPoolSnapshotStale     = errors.New("agent plan quota pool snapshot is stale")
	ErrAgentPlanQuotaPoolStockInsufficient = errors.New("agent plan quota pool stock is insufficient")
	ErrAgentPlanQuotaPoolSync              = errors.New("agent plan quota pool sync failed")
	ErrAgentPlanPackageAllocationInvalid   = errors.New("agent plan package allocation is invalid")
)

type AgentPlanQuotaPoolStock struct {
	OfficialMonthlyRemainingAFPMicros int64 `json:"official_monthly_remaining_afp_micros"`
	ActiveReservationAFPMicros        int64 `json:"active_reservation_afp_micros"`
	SellableAFPMicros                 int64 `json:"sellable_afp_micros"`
}

type AgentPlanPackageAllocationRequest struct {
	PackagePlanId      int
	UserSubscriptionId int
	Tx                 *gorm.DB
	Freshness          *AgentPlanQuotaPoolFreshness
	// Now is only intended for deterministic internal callers and tests. A zero
	// value uses the current wall clock.
	Now time.Time
}

// AgentPlanQuotaPoolFreshness records a pool snapshot that was refreshed and
// source-validated before an enclosing booking transaction began.
type AgentPlanQuotaPoolFreshness struct {
	PoolId     int
	SyncedAt   int64
	VerifiedAt time.Time
}

type AgentPlanPackageAllocationBooking struct {
	Allocation                 *model.AgentPlanPackageAllocation `json:"allocation"`
	DisplayAllocationAFPMicros int64                             `json:"display_allocation_afp_micros"`
	Stock                      AgentPlanQuotaPoolStock           `json:"stock"`
}

var agentPlanQuotaPoolUsageCache = NewAgentPlanUsageCache(
	agentPlanQuotaPoolSnapshotMaxAge,
	time.Now,
	FetchAgentPlanUsage,
)

var agentPlanQuotaPoolNow = time.Now

// fetchAgentPlanUsageForQuotaPool keeps source credentials inside the service.
// The public pool APIs only return usage snapshots and stock values.
var fetchAgentPlanUsageForQuotaPool = func(ctx context.Context, channel *model.Channel) (AgentPlanUsage, int64, bool, error) {
	client, err := GetHttpClientWithProxy(channel.GetSetting().Proxy)
	if err != nil {
		return AgentPlanUsage{}, 0, false, err
	}
	credentials := strings.TrimSpace(channel.AgentPlanAccessKey) + "|" + strings.TrimSpace(channel.AgentPlanSecretKey)
	return agentPlanQuotaPoolUsageCache.Fetch(ctx, channel.Id, client, agentPlanQuotaPoolUsageEndpoint, credentials)
}

// SyncAgentPlanQuotaPool refreshes an existing pool from the official usage
// service. It never returns the source channel credentials to its caller.
func SyncAgentPlanQuotaPool(ctx context.Context, poolId int) (*model.AgentPlanQuotaPool, error) {
	pool, err := model.GetAgentPlanQuotaPoolById(poolId)
	if err != nil {
		return nil, err
	}
	channel, err := getAgentPlanQuotaPoolSource(pool)
	if err != nil {
		return nil, markAgentPlanQuotaPoolSyncError(poolId, err)
	}

	usage, updatedAt, stale, err := fetchAgentPlanUsageForQuotaPool(ctx, channel)
	if err != nil {
		if stale {
			return nil, markAgentPlanQuotaPoolStale(poolId)
		}
		return nil, markAgentPlanQuotaPoolSyncError(poolId, err)
	}
	if stale || updatedAt <= 0 {
		return nil, markAgentPlanQuotaPoolStale(poolId)
	}
	fiveHourMicros, err := agentPlanUsageAFPMicros(usage.FiveHour.Remaining)
	if err != nil {
		return nil, markAgentPlanQuotaPoolSyncError(poolId, err)
	}
	weeklyMicros, err := agentPlanUsageAFPMicros(usage.Weekly.Remaining)
	if err != nil {
		return nil, markAgentPlanQuotaPoolSyncError(poolId, err)
	}
	monthlyMicros, err := agentPlanUsageAFPMicros(usage.Monthly.Remaining)
	if err != nil {
		return nil, markAgentPlanQuotaPoolSyncError(poolId, err)
	}

	updates := map[string]interface{}{
		"official_monthly_remaining_afp_micros": monthlyMicros,
		"five_hour_remaining_afp_micros":        fiveHourMicros,
		"five_hour_reset_at":                    usage.FiveHour.ResetAt,
		"weekly_remaining_afp_micros":           weeklyMicros,
		"weekly_reset_at":                       usage.Weekly.ResetAt,
		"monthly_reset_at":                      usage.Monthly.ResetAt,
		"synced_at":                             updatedAt,
		"sync_status":                           model.AgentPlanQuotaPoolSyncStatusAvailable,
		"sync_error":                            "",
	}
	if err := model.DB.Model(&model.AgentPlanQuotaPool{}).Where("id = ?", poolId).Updates(updates).Error; err != nil {
		return nil, err
	}
	return model.GetAgentPlanQuotaPoolById(poolId)
}

func getAgentPlanQuotaPoolSource(pool *model.AgentPlanQuotaPool) (*model.Channel, error) {
	if pool == nil || pool.SourceChannelId <= 0 {
		return nil, model.ErrAgentPlanQuotaPoolSourceInvalid
	}
	channel, err := model.GetChannelById(pool.SourceChannelId, true)
	if err != nil {
		return nil, err
	}
	if err := model.ValidateAgentPlanQuotaPoolSource(channel); err != nil {
		return nil, err
	}
	return channel, nil
}

func markAgentPlanQuotaPoolSyncError(poolId int, cause error) error {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	if updateErr := model.DB.Model(&model.AgentPlanQuotaPool{}).Where("id = ?", poolId).Updates(map[string]interface{}{
		"sync_status": model.AgentPlanQuotaPoolSyncStatusError,
		"sync_error":  message,
	}).Error; updateErr != nil {
		return fmt.Errorf("%w: %v (persist sync error: %v)", ErrAgentPlanQuotaPoolSync, cause, updateErr)
	}
	return fmt.Errorf("%w: %v", ErrAgentPlanQuotaPoolSync, cause)
}

func markAgentPlanQuotaPoolStale(poolId int) error {
	if updateErr := model.DB.Model(&model.AgentPlanQuotaPool{}).Where("id = ?", poolId).Updates(map[string]interface{}{
		"sync_status": model.AgentPlanQuotaPoolSyncStatusError,
		"sync_error":  ErrAgentPlanQuotaPoolSnapshotStale.Error(),
	}).Error; updateErr != nil {
		return fmt.Errorf("%w: persist stale sync status: %v", ErrAgentPlanQuotaPoolSnapshotStale, updateErr)
	}
	return ErrAgentPlanQuotaPoolSnapshotStale
}

func agentPlanUsageAFPMicros(value float64) (int64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0, errors.New("agent plan AFP usage must be finite and non-negative")
	}
	if value > float64(math.MaxInt64)/float64(AgentPlanAFPMicros) {
		return 0, errors.New("agent plan AFP usage exceeds micro-AFP range")
	}
	rounded := math.Round(value * float64(AgentPlanAFPMicros))
	if rounded < 0 || rounded >= float64(math.MaxInt64) {
		return 0, errors.New("agent plan AFP usage exceeds micro-AFP range after rounding")
	}
	return int64(rounded), nil
}

func agentPlanQuotaPoolSnapshotFresh(pool *model.AgentPlanQuotaPool, now time.Time) bool {
	if pool == nil || pool.SyncStatus != model.AgentPlanQuotaPoolSyncStatusAvailable || pool.SyncedAt <= 0 {
		return false
	}
	syncedAt := time.Unix(pool.SyncedAt, 0)
	return !syncedAt.After(now) && now.Sub(syncedAt) < agentPlanQuotaPoolSnapshotMaxAge
}

func ensureFreshAgentPlanQuotaPool(ctx context.Context, poolId int, now time.Time) (time.Time, error) {
	pool, err := model.GetAgentPlanQuotaPoolById(poolId)
	if err != nil {
		return now, err
	}
	if _, err := getAgentPlanQuotaPoolSource(pool); err != nil {
		return now, markAgentPlanQuotaPoolSyncError(poolId, err)
	}
	if agentPlanQuotaPoolSnapshotFresh(pool, now) {
		return now, nil
	}
	_, err = SyncAgentPlanQuotaPool(ctx, poolId)
	if err != nil {
		if errors.Is(err, ErrAgentPlanQuotaPoolSnapshotStale) {
			return now, err
		}
		return now, fmt.Errorf("%w: %v", ErrAgentPlanQuotaPoolSnapshotStale, err)
	}
	pool, err = model.GetAgentPlanQuotaPoolById(poolId)
	if err != nil {
		return now, err
	}
	freshNow := agentPlanQuotaPoolNow()
	if !agentPlanQuotaPoolSnapshotFresh(pool, freshNow) {
		return freshNow, ErrAgentPlanQuotaPoolSnapshotStale
	}
	return freshNow, nil
}

// PrepareAgentPlanQuotaPoolAllocation refreshes and validates the pool before
// a caller opens an enclosing transaction. The returned freshness record must
// accompany allocation work done through an existing transaction.
func PrepareAgentPlanQuotaPoolAllocation(ctx context.Context, poolId int, now time.Time) (*AgentPlanQuotaPoolFreshness, error) {
	if poolId <= 0 {
		return nil, ErrAgentPlanPackageAllocationInvalid
	}
	if now.IsZero() {
		now = agentPlanQuotaPoolNow()
	}
	verifiedAt, err := ensureFreshAgentPlanQuotaPool(ctx, poolId, now)
	if err != nil {
		return nil, err
	}
	pool, err := model.GetAgentPlanQuotaPoolById(poolId)
	if err != nil {
		return nil, err
	}
	if !agentPlanQuotaPoolSnapshotFresh(pool, verifiedAt) {
		return nil, ErrAgentPlanQuotaPoolSnapshotStale
	}
	return &AgentPlanQuotaPoolFreshness{
		PoolId:     pool.Id,
		SyncedAt:   pool.SyncedAt,
		VerifiedAt: verifiedAt,
	}, nil
}

// GetAgentPlanQuotaPoolStock returns the raw official AFP that remains sellable
// after reservations held by active, non-depleted subscriptions.
func GetAgentPlanQuotaPoolStock(poolId int, now time.Time) (AgentPlanQuotaPoolStock, error) {
	if poolId <= 0 {
		return AgentPlanQuotaPoolStock{}, ErrAgentPlanPackageAllocationInvalid
	}
	stock := AgentPlanQuotaPoolStock{}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		pool, err := model.GetAgentPlanQuotaPoolForUpdate(tx, poolId)
		if err != nil {
			return err
		}
		computed, err := agentPlanQuotaPoolStockForUpdate(tx, pool, now)
		if err != nil {
			return err
		}
		stock = computed
		return nil
	})
	return stock, err
}

// CreateAgentPlanPackageAllocation creates the subscription snapshot and books
// its raw AFP inventory while holding the pool row lock. Existing allocations
// are returned idempotently.
func CreateAgentPlanPackageAllocation(ctx context.Context, request AgentPlanPackageAllocationRequest) (*AgentPlanPackageAllocationBooking, error) {
	if request.PackagePlanId <= 0 || request.UserSubscriptionId <= 0 {
		return nil, ErrAgentPlanPackageAllocationInvalid
	}
	now := request.Now
	if now.IsZero() {
		now = agentPlanQuotaPoolNow()
	}

	query := request.Tx
	if query == nil {
		query = model.DB
	}
	packagePlan := &model.AgentPlanPackagePlan{}
	if err := query.Where("id = ?", request.PackagePlanId).First(packagePlan).Error; err != nil {
		return nil, err
	}
	freshness := request.Freshness
	if request.Tx == nil {
		var err error
		freshness, err = PrepareAgentPlanQuotaPoolAllocation(ctx, packagePlan.PoolId, now)
		if err != nil {
			return nil, err
		}
		now = freshness.VerifiedAt
	}
	if freshness == nil || freshness.PoolId != packagePlan.PoolId || freshness.SyncedAt <= 0 || freshness.VerifiedAt.IsZero() {
		return nil, ErrAgentPlanPackageAllocationInvalid
	}

	booking := &AgentPlanPackageAllocationBooking{}
	err := query.Transaction(func(tx *gorm.DB) error {
		pool, err := model.GetAgentPlanQuotaPoolForUpdate(tx, packagePlan.PoolId)
		if err != nil {
			return err
		}
		if pool.SyncedAt < freshness.SyncedAt || !agentPlanQuotaPoolSnapshotFresh(pool, freshness.VerifiedAt) {
			return ErrAgentPlanQuotaPoolSnapshotStale
		}
		channel, err := model.GetChannelByIdForUpdate(tx, pool.SourceChannelId)
		if err != nil {
			return err
		}
		if err := model.ValidateAgentPlanQuotaPoolSource(channel); err != nil {
			return ErrAgentPlanPackageAllocationInvalid
		}
		lockedPackagePlan, err := model.GetAgentPlanPackagePlanForUpdate(tx, request.PackagePlanId)
		if err != nil {
			return err
		}
		if lockedPackagePlan.PoolId != pool.Id || lockedPackagePlan.AllocationAFPMicros <= 0 {
			return ErrAgentPlanPackageAllocationInvalid
		}
		if pool.DisplayMultiplierMicros <= 0 {
			return ErrAgentPlanPackageAllocationInvalid
		}
		subscription, err := model.GetUserSubscriptionForUpdate(tx, request.UserSubscriptionId)
		if err != nil {
			return err
		}
		if subscription.PlanId != lockedPackagePlan.SubscriptionPlanId || !agentPlanSubscriptionCanReserve(subscription, freshness.VerifiedAt) {
			return ErrAgentPlanPackageAllocationInvalid
		}

		existing, err := model.GetAgentPlanPackageAllocationForUpdate(tx, subscription.Id, pool.Id)
		if err == nil {
			stock, stockErr := agentPlanQuotaPoolStockForUpdate(tx, pool, freshness.VerifiedAt)
			if stockErr != nil {
				return stockErr
			}
			displayAFP, displayErr := agentPlanDisplayAFPMicros(existing.AllocationAFPMicros, existing.DisplayMultiplierMicros)
			if displayErr != nil {
				return displayErr
			}
			booking.Allocation = existing
			booking.DisplayAllocationAFPMicros = displayAFP
			booking.Stock = stock
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		stock, err := agentPlanQuotaPoolStockForUpdate(tx, pool, freshness.VerifiedAt)
		if err != nil {
			return err
		}
		reservation, err := agentPlanAllocationReservation(lockedPackagePlan.AllocationAFPMicros, subscription)
		if err != nil {
			return err
		}
		if stock.SellableAFPMicros < reservation {
			return ErrAgentPlanQuotaPoolStockInsufficient
		}

		allocation := &model.AgentPlanPackageAllocation{
			UserSubscriptionId:      subscription.Id,
			PoolId:                  pool.Id,
			AllocationAFPMicros:     lockedPackagePlan.AllocationAFPMicros,
			DisplayMultiplierMicros: pool.DisplayMultiplierMicros,
			ScopeGroup:              lockedPackagePlan.ScopeGroup,
			ScopeModels:             lockedPackagePlan.ScopeModels,
			AllowWalletFallback:     lockedPackagePlan.AllowWalletFallback,
		}
		if err := tx.Create(allocation).Error; err != nil {
			return err
		}
		stock, err = agentPlanQuotaPoolStockForUpdate(tx, pool, freshness.VerifiedAt)
		if err != nil {
			return err
		}
		displayAFP, err := agentPlanDisplayAFPMicros(allocation.AllocationAFPMicros, allocation.DisplayMultiplierMicros)
		if err != nil {
			return err
		}
		booking.Allocation = allocation
		booking.DisplayAllocationAFPMicros = displayAFP
		booking.Stock = stock
		return nil
	})
	if err != nil {
		return nil, err
	}
	return booking, nil
}

func agentPlanQuotaPoolStockForUpdate(tx *gorm.DB, pool *model.AgentPlanQuotaPool, now time.Time) (AgentPlanQuotaPoolStock, error) {
	if tx == nil || pool == nil {
		return AgentPlanQuotaPoolStock{}, ErrAgentPlanPackageAllocationInvalid
	}
	stock := AgentPlanQuotaPoolStock{OfficialMonthlyRemainingAFPMicros: pool.OfficialMonthlyRemainingAFPMicros}
	allocations, err := model.ListAgentPlanPackageAllocationsForUpdate(tx, pool.Id)
	if err != nil {
		return AgentPlanQuotaPoolStock{}, err
	}
	ids := make([]int, 0, len(allocations))
	for _, allocation := range allocations {
		ids = append(ids, allocation.UserSubscriptionId)
	}
	subscriptions, err := model.ListUserSubscriptionsForUpdate(tx, ids)
	if err != nil {
		return AgentPlanQuotaPoolStock{}, err
	}
	subscriptionsById := make(map[int]*model.UserSubscription, len(subscriptions))
	for i := range subscriptions {
		subscriptionsById[subscriptions[i].Id] = &subscriptions[i]
	}
	for _, allocation := range allocations {
		subscription := subscriptionsById[allocation.UserSubscriptionId]
		if !agentPlanSubscriptionCanReserve(subscription, now) {
			continue
		}
		reservation, err := agentPlanAllocationReservation(allocation.AllocationAFPMicros, subscription)
		if err != nil {
			return AgentPlanQuotaPoolStock{}, err
		}
		if reservation > math.MaxInt64-stock.ActiveReservationAFPMicros {
			return AgentPlanQuotaPoolStock{}, errors.New("agent plan quota pool reservation exceeds int64")
		}
		stock.ActiveReservationAFPMicros += reservation
	}
	if stock.OfficialMonthlyRemainingAFPMicros > stock.ActiveReservationAFPMicros {
		stock.SellableAFPMicros = stock.OfficialMonthlyRemainingAFPMicros - stock.ActiveReservationAFPMicros
	}
	return stock, nil
}

func agentPlanSubscriptionCanReserve(subscription *model.UserSubscription, now time.Time) bool {
	if subscription == nil || subscription.Status != "active" || subscription.EndTime <= now.Unix() {
		return false
	}
	return subscription.AmountTotal > 0 && subscription.AmountUsed < subscription.AmountTotal
}

func agentPlanAllocationReservation(allocationAFPMicros int64, subscription *model.UserSubscription) (int64, error) {
	if allocationAFPMicros <= 0 || subscription == nil {
		return 0, ErrAgentPlanPackageAllocationInvalid
	}
	if subscription.AmountTotal <= 0 {
		return 0, ErrAgentPlanPackageAllocationInvalid
	}
	remaining := subscription.AmountTotal - subscription.AmountUsed
	if remaining <= 0 {
		return 0, nil
	}
	return agentPlanCeilMulDiv(allocationAFPMicros, remaining, subscription.AmountTotal)
}

func agentPlanDisplayAFPMicros(allocationAFPMicros int64, multiplierMicros int64) (int64, error) {
	if allocationAFPMicros <= 0 || multiplierMicros <= 0 {
		return 0, ErrAgentPlanPackageAllocationInvalid
	}
	return agentPlanCeilMulDiv(allocationAFPMicros, multiplierMicros, AgentPlanMultiplierMicros)
}

func agentPlanCeilMulDiv(left int64, right int64, divisor int64) (int64, error) {
	if left < 0 || right < 0 || divisor <= 0 {
		return 0, ErrAgentPlanPackageAllocationInvalid
	}
	product := new(big.Int).Mul(big.NewInt(left), big.NewInt(right))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(product, big.NewInt(divisor), remainder)
	if remainder.Sign() > 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, errors.New("agent plan AFP calculation exceeds int64")
	}
	return quotient.Int64(), nil
}
