package controller

import (
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A top-up amount that passes the request-time cap must not overflow the int32
// quota column at settlement, otherwise the payment provider captures money for
// a top-up that can never be credited (order stuck pending).
func TestTopupUnitCapKeepsSettlementQuotaWithinInt32(t *testing.T) {
	cap := topupUnitCap()
	require.Greater(t, cap, int64(0))
	require.LessOrEqual(t, cap, maxTopupAmount)

	// Largest allowed amount converts to a quota that still fits int32.
	atCap := decimal.NewFromInt(cap).Mul(decimal.NewFromFloat(common.QuotaPerUnit))
	assert.True(t, atCap.LessThanOrEqual(decimal.NewFromInt(int64(common.MaxQuota))),
		"quota at cap %s exceeds MaxQuota %d", atCap.String(), common.MaxQuota)

	// When the int32-safe ceiling is the binding constraint (default config), one
	// unit past the cap overflows, so the cap sits exactly on the boundary.
	if cap < maxTopupAmount {
		pastCap := decimal.NewFromInt(cap+1).Mul(decimal.NewFromFloat(common.QuotaPerUnit))
		assert.True(t, pastCap.GreaterThan(decimal.NewFromInt(int64(common.MaxQuota))),
			"amount past cap %s should overflow MaxQuota %d", pastCap.String(), common.MaxQuota)
	}
}

// settlementQuotaOverflows guards the exact value a provider settles (Stripe's
// ratio-adjusted Money), so a group ratio > 1 that pushes the charge past the
// int32 ceiling is rejected before payment even though the unit cap passed.
func TestSettlementQuotaOverflowsGuardsRatioAdjustedMoney(t *testing.T) {
	safe := float64(topupUnitCap())
	assert.False(t, settlementQuotaOverflows(decimal.NewFromFloat(safe)),
		"amount at the unit cap must settle without overflow")

	// A ratio of 2 on an at-cap amount doubles the settled Money past int32.
	assert.True(t, settlementQuotaOverflows(decimal.NewFromFloat(safe*2)),
		"ratio-amplified money above the int32 ceiling must be rejected")

	assert.True(t, settlementQuotaOverflows(decimal.NewFromInt(int64(common.MaxQuota))),
		"a settlement amount at MaxQuota units overflows once multiplied by QuotaPerUnit")
}
