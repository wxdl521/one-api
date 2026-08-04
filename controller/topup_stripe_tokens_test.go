package controller

import (
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/setting/operation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalizeTopUpUnitsTokensChargeBasis locks the unit basis the Stripe
// Checkout quantity, the price quote, and the settlement/credit all derive
// from. In TOKENS display mode a top-up amount is a token count; charging the
// raw token count against the fixed StripePriceId would over-charge the card
// by QuotaPerUnit while crediting only the $-equivalent quota.
// 口径说明：这里锁的是**单位数量**一致；topupGroupRatio/AmountDiscount 只作用于
// 报价与结算 Money，固定单价的 Checkout 不承载该系数（默认 ratio=1 时无偏差），
// 属已存档的已知例外，见 stripeCheckoutParams 注释。
func TestNormalizeTopUpUnitsTokensChargeBasis(t *testing.T) {
	gs := operation_setting.GetGeneralSetting()
	original := gs.QuotaDisplayType
	t.Cleanup(func() { gs.QuotaDisplayType = original })

	// Non-TOKENS mode: the amount is already a billing unit count, passed through.
	gs.QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	assert.Equal(t, int64(7), normalizeTopUpUnits(7), "non-TOKENS mode passes amount through unchanged")

	// TOKENS mode: a token count normalizes to the USD-equivalent unit count that
	// every settle path bills on, so QuotaPerUnit tokens == exactly 1 billing unit.
	gs.QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens
	tokens := int64(common.QuotaPerUnit)
	assert.Equal(t, int64(1), normalizeTopUpUnits(tokens), "QuotaPerUnit tokens = 1 billing unit")
	assert.NotEqual(t, tokens, normalizeTopUpUnits(tokens),
		"TOKENS-mode charge quantity must be the normalized unit count, not the raw token count")
}

// TestStripeCheckoutParamsNormalizesTokensQuantity 锁定真正被 R5 修复的那一环：
// Checkout Session 的 LineItems Quantity 必须是归一化后的单位数量。归一化收敛在
// stripeCheckoutParams 内部（接收原始请求数量），调用点无法再传错口径。
func TestStripeCheckoutParamsNormalizesTokensQuantity(t *testing.T) {
	gs := operation_setting.GetGeneralSetting()
	original := gs.QuotaDisplayType
	t.Cleanup(func() { gs.QuotaDisplayType = original })

	// TOKENS mode: QuotaPerUnit tokens must become exactly 1 Checkout unit.
	gs.QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens
	params := stripeCheckoutParams("ref_test", "", "user@example.com", int64(common.QuotaPerUnit), "", "")
	require.Len(t, params.LineItems, 1)
	require.NotNil(t, params.LineItems[0].Quantity)
	assert.Equal(t, int64(1), *params.LineItems[0].Quantity,
		"TOKENS-mode Checkout quantity must be the normalized unit count")

	// Non-TOKENS mode: quantity passes through unchanged.
	gs.QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	params = stripeCheckoutParams("ref_test", "cus_x", "", 7, "", "")
	require.Len(t, params.LineItems, 1)
	require.NotNil(t, params.LineItems[0].Quantity)
	assert.Equal(t, int64(7), *params.LineItems[0].Quantity,
		"non-TOKENS-mode Checkout quantity is the raw unit amount")
}
