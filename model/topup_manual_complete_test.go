package model

import (
	"testing"

	"github.com/QuantumNous/the-one/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManualCompleteTopUpCreemCreditsAmountDirectly 锁定 Creem 手动补单的入账口径：
// Creem 订单的 Amount 已是充值额度（quota），补单必须直接入账，不能再乘 QuotaPerUnit，
// 否则会对用户超额充值（回归自 R4-1）。
func TestManualCompleteTopUpCreemCreditsAmountDirectly(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&User{}, &TopUp{}))
	require.NoError(t, DB.Where("trade_no = ?", "creem-manual-1").Delete(&TopUp{}).Error)
	require.NoError(t, DB.Where("username = ?", "creem-manual-user").Delete(&User{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Where("trade_no = ?", "creem-manual-1").Delete(&TopUp{}).Error)
		require.NoError(t, DB.Where("username = ?", "creem-manual-user").Delete(&User{}).Error)
	})

	user := User{Username: "creem-manual-user", AffCode: "creem-manual-user", Status: common.UserStatusEnabled, Quota: 0}
	require.NoError(t, DB.Create(&user).Error)

	topUp := TopUp{
		UserId:          user.Id,
		Amount:          1000, // Creem: Amount 即充值额度（quota）
		Money:           9.99,
		TradeNo:         "creem-manual-1",
		PaymentProvider: PaymentProviderCreem,
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, DB.Create(&topUp).Error)

	require.NoError(t, ManualCompleteTopUp(topUp.TradeNo, "127.0.0.1"))

	var updated User
	require.NoError(t, DB.First(&updated, user.Id).Error)
	assert.Equal(t, 1000, updated.Quota, "Creem 补单应直接入账 Amount，不得乘 QuotaPerUnit")

	var settled TopUp
	require.NoError(t, DB.Where("trade_no = ?", topUp.TradeNo).First(&settled).Error)
	assert.Equal(t, common.TopUpStatusSuccess, settled.Status)
}

// TestManualCompleteTopUpNonCreemScalesByQuotaPerUnit 确认非 Creem 订单（如易支付）的
// Amount 仍按美元数量处理，补单时乘 QuotaPerUnit，不被 R4 修复误伤。
func TestManualCompleteTopUpNonCreemScalesByQuotaPerUnit(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&User{}, &TopUp{}))
	require.NoError(t, DB.Where("trade_no = ?", "epay-manual-1").Delete(&TopUp{}).Error)
	require.NoError(t, DB.Where("username = ?", "epay-manual-user").Delete(&User{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Where("trade_no = ?", "epay-manual-1").Delete(&TopUp{}).Error)
		require.NoError(t, DB.Where("username = ?", "epay-manual-user").Delete(&User{}).Error)
	})

	user := User{Username: "epay-manual-user", AffCode: "epay-manual-user", Status: common.UserStatusEnabled, Quota: 0}
	require.NoError(t, DB.Create(&user).Error)

	topUp := TopUp{
		UserId:          user.Id,
		Amount:          5, // Epay: Amount 为美元数量
		Money:           5,
		TradeNo:         "epay-manual-1",
		PaymentProvider: PaymentProviderEpay,
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, DB.Create(&topUp).Error)

	require.NoError(t, ManualCompleteTopUp(topUp.TradeNo, "127.0.0.1"))

	var updated User
	require.NoError(t, DB.First(&updated, user.Id).Error)
	assert.Equal(t, int(5*common.QuotaPerUnit), updated.Quota, "非 Creem 订单应按美元数量乘 QuotaPerUnit")
}

// TestRechargeCreemRejectsInt32Overflow 锁定 Creem webhook 入账的 int32 校验：
// Amount 超过配额列上界时必须整单拒绝（事务回滚），不得饱和入账或回绕，
// 用户余额与订单状态均不变（回归自 R5-3 / R7）。
func TestRechargeCreemRejectsInt32Overflow(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&User{}, &TopUp{}))
	require.NoError(t, DB.Where("trade_no = ?", "creem-overflow-1").Delete(&TopUp{}).Error)
	require.NoError(t, DB.Where("username = ?", "creem-overflow-user").Delete(&User{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Where("trade_no = ?", "creem-overflow-1").Delete(&TopUp{}).Error)
		require.NoError(t, DB.Where("username = ?", "creem-overflow-user").Delete(&User{}).Error)
	})

	user := User{Username: "creem-overflow-user", AffCode: "creem-overflow-user", Status: common.UserStatusEnabled, Quota: 0}
	require.NoError(t, DB.Create(&user).Error)

	topUp := TopUp{
		UserId:          user.Id,
		Amount:          int64(common.MaxQuota) + 1, // 超 int32 配额列上界
		Money:           9.99,
		TradeNo:         "creem-overflow-1",
		PaymentProvider: PaymentProviderCreem,
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, DB.Create(&topUp).Error)

	err := RechargeCreem(topUp.TradeNo, "", "", "127.0.0.1")
	require.Error(t, err, "超上界订单必须被拒绝")

	var updatedUser User
	require.NoError(t, DB.First(&updatedUser, user.Id).Error)
	assert.Equal(t, 0, updatedUser.Quota, "拒绝的订单不得改动用户余额")

	var updatedTopUp TopUp
	require.NoError(t, DB.Where("trade_no = ?", "creem-overflow-1").First(&updatedTopUp).Error)
	assert.Equal(t, common.TopUpStatusPending, updatedTopUp.Status, "事务回滚后订单应保持 Pending")
}
