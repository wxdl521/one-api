package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildWeChatNativeOrderUsesCNYCentsAndExpiry(t *testing.T) {
	expiresAt := time.Date(2026, 7, 30, 12, 15, 0, 0, time.UTC)
	order, err := BuildWeChatNativeOrder(WeChatNativeOrderParams{
		AppID:       "wx1234567890abcdef",
		MerchantID:  "1900000001",
		TradeNo:     "USR1NO202607300001",
		Description: "TUC10",
		NotifyURL:   "https://api.example.com/api/wechat/notify",
		AmountCents: 730,
		ExpiresAt:   expiresAt,
	})

	require.NoError(t, err)
	assert.Equal(t, "wx1234567890abcdef", order.AppID)
	assert.Equal(t, "1900000001", order.MerchantID)
	assert.Equal(t, "USR1NO202607300001", order.TradeNo)
	assert.Equal(t, int64(730), order.AmountCents)
	assert.Equal(t, "CNY", order.Currency)
	assert.Equal(t, expiresAt, order.ExpiresAt)
}

func TestBuildWeChatNativeOrderRejectsNonPositiveCents(t *testing.T) {
	_, err := BuildWeChatNativeOrder(WeChatNativeOrderParams{
		AppID:      "wx1234567890abcdef",
		MerchantID: "1900000001",
		TradeNo:    "USR1NO202607300001",
		NotifyURL:  "https://api.example.com/api/wechat/notify",
	})
	require.Error(t, err)
}

func TestValidateWeChatPaymentResultRejectsMerchantMismatch(t *testing.T) {
	result := WeChatPaymentResult{
		TradeNo:         "USR1NO202607300001",
		ProviderTradeNo: "4200001234202607300000000000",
		AppID:           "wx1234567890abcdef",
		MerchantID:      "1900000001",
		AmountCents:     730,
		Currency:        "CNY",
		Status:          WeChatPaymentStatusSuccess,
	}
	assert.NoError(t, ValidateWeChatPaymentResult(WeChatPayConfig{
		AppID:      "wx1234567890abcdef",
		MerchantID: "1900000001",
	}, result, 730))

	result.MerchantID = "1900000002"
	require.Error(t, ValidateWeChatPaymentResult(WeChatPayConfig{
		AppID:      "wx1234567890abcdef",
		MerchantID: "1900000001",
	}, result, 730))
}

func TestValidateWeChatPaymentResultRejectsAppAmountAndCurrencyMismatch(t *testing.T) {
	config := WeChatPayConfig{AppID: "wx-app", MerchantID: "mch-id"}
	valid := WeChatPaymentResult{
		TradeNo:         "order-1",
		ProviderTradeNo: "provider-1",
		AppID:           "wx-app",
		MerchantID:      "mch-id",
		AmountCents:     730,
		Currency:        wechatPayCurrencyCNY,
		Status:          WeChatPaymentStatusSuccess,
	}
	require.NoError(t, ValidateWeChatPaymentResult(config, valid, 730))

	wrongApp := valid
	wrongApp.AppID = "wx-other"
	require.Error(t, ValidateWeChatPaymentResult(config, wrongApp, 730))

	wrongAmount := valid
	wrongAmount.AmountCents = 731
	require.Error(t, ValidateWeChatPaymentResult(config, wrongAmount, 730))

	wrongCurrency := valid
	wrongCurrency.Currency = "USD"
	require.Error(t, ValidateWeChatPaymentResult(config, wrongCurrency, 730))
}
