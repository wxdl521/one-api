package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTopUpStatusRejectsOtherUsersOrder(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}))
	require.NoError(t, db.Create(&model.TopUp{
		UserId:          1,
		Amount:          10,
		Money:           7.30,
		TradeNo:         "official-payment-owner-check",
		PaymentMethod:   model.PaymentMethodAlipayDirect,
		PaymentProvider: model.PaymentProviderAlipay,
		Status:          common.TopUpStatusSuccess,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/topup/official-payment-owner-check/status", nil)
	ctx.Params = gin.Params{{Key: "trade_no", Value: "official-payment-owner-check"}}
	ctx.Set("id", 2)

	GetTopUpStatus(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "topup order not found")
}

func TestGetOptionsDoesNotReturnOfficialPaymentKeys(t *testing.T) {
	previousOptions := common.OptionMap
	t.Cleanup(func() {
		common.OptionMap = previousOptions
	})
	common.OptionMap = map[string]string{
		"AlipayAppID":                 "app-id",
		"AlipayAppPrivateKey":         "private-key",
		"AlipayPublicKey":             "public-key",
		"WeChatPayMerchantID":         "merchant-id",
		"WeChatPayMerchantPrivateKey": "private-key",
		"WeChatPayAPIv3Key":           "api-v3-key",
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/option/", nil)

	GetOptions(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "AlipayAppID")
	assert.Contains(t, recorder.Body.String(), "WeChatPayMerchantID")
	assert.NotContains(t, recorder.Body.String(), "private-key")
	assert.NotContains(t, recorder.Body.String(), "public-key")
	assert.NotContains(t, recorder.Body.String(), "api-v3-key")
}
