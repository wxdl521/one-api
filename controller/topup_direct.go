package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/logger"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/service"
	"github.com/QuantumNous/the-one/setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const officialPaymentOrderLifetime = 15 * time.Minute

var (
	newAlipayDirectClient = service.NewAlipayClient
	newWeChatNativeClient = service.NewWeChatNativeClient
	directPaymentQueries  sync.Map
)

type directPaymentRequest struct {
	Amount int64 `json:"amount"`
}

type directPaymentQueryState struct {
	mu          sync.Mutex
	lastQueried time.Time
}

func alipayConfig() service.AlipayConfig {
	return service.AlipayConfig{
		AppID:           setting.AlipayAppID,
		SellerID:        setting.AlipaySellerID,
		AppPrivateKey:   setting.AlipayAppPrivateKey,
		AlipayPublicKey: setting.AlipayPublicKey,
	}
}

func wechatPayConfig() service.WeChatPayConfig {
	return service.WeChatPayConfig{
		AppID:                     setting.WeChatPayAppID,
		MerchantID:                setting.WeChatPayMerchantID,
		MerchantCertificateSerial: setting.WeChatPayMerchantCertificateSerial,
		MerchantPrivateKey:        setting.WeChatPayMerchantPrivateKey,
		APIv3Key:                  setting.WeChatPayAPIv3Key,
	}
}

func officialPaymentNotifyURL(path string) string {
	return strings.TrimRight(service.GetCallbackAddress(), "/") + path
}

func directPaymentAmountCents(amount int64, group string) (int64, float64, error) {
	payMoney := getPayMoney(amount, group)
	if payMoney < 0.01 {
		return 0, 0, errors.New("topup amount is too low")
	}
	cents, err := service.ParseCNYAmountCents(decimal.NewFromFloat(payMoney).Round(2).StringFixed(2))
	if err != nil {
		return 0, 0, err
	}
	return cents, float64(cents) / 100, nil
}


func createOfficialTopUp(c *gin.Context, provider, method string, amount int64) (*model.TopUp, error) {
	if amount < getMinTopup() {
		return nil, fmt.Errorf("topup amount must be at least %d", getMinTopup())
	}
	userID := c.GetInt("id")
	group, err := model.GetUserGroup(userID, true)
	if err != nil {
		return nil, errors.New("get user group failed")
	}
	cents, money, err := directPaymentAmountCents(amount, group)
	if err != nil {
		return nil, err
	}
	normalizedAmount := normalizeTopUpUnits(amount)
	quota, clamp := common.QuotaFromDecimalChecked(decimal.NewFromInt(normalizedAmount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)))
	if clamp != nil || quota <= 0 {
		return nil, errors.New("invalid topup amount")
	}
	createdAt := time.Now()
	tradeNo := fmt.Sprintf("DIR%dNO%s%d", userID, common.GetRandomString(6), createdAt.UnixNano())
	topUp := &model.TopUp{
		UserId:             userID,
		Amount:             normalizedAmount,
		Money:              money,
		PaymentAmountCents: cents,
		TradeNo:            tradeNo,
		PaymentMethod:      method,
		PaymentProvider:    provider,
		CreateTime:         createdAt.Unix(),
		ExpireTime:         createdAt.Add(officialPaymentOrderLifetime).Unix(),
		Status:             common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		return nil, err
	}
	return topUp, nil
}

func RequestAlipayPay(c *gin.Context) {
	if !isAlipayTopUpEnabled() {
		common.ApiErrorMsg(c, "Alipay direct payment is not available")
		return
	}
	var req directPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "invalid parameters")
		return
	}
	topUp, err := createOfficialTopUp(c, model.PaymentProviderAlipay, model.PaymentMethodAlipayDirect, req.Amount)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	payment, err := service.BuildAlipayPagePayment(alipayConfig(), service.AlipayPagePaymentRequest{
		TradeNo:   topUp.TradeNo,
		Subject:   fmt.Sprintf("Wallet topup %d", topUp.Amount),
		Amount:    decimal.NewFromInt(topUp.PaymentAmountCents).Div(decimal.NewFromInt(100)).StringFixed(2),
		NotifyURL: officialPaymentNotifyURL("/api/alipay/notify"),
		ReturnURL: paymentReturnPath("/wallet?show_history=true"),
	})
	if err != nil {
		_ = model.UpdatePendingTopUpStatus(topUp.TradeNo, model.PaymentProviderAlipay, common.TopUpStatusFailed)
		logger.LogError(c.Request.Context(), fmt.Sprintf("build alipay payment failed trade_no=%s error=%q", topUp.TradeNo, err.Error()))
		common.ApiErrorMsg(c, "create payment failed")
		return
	}
	common.ApiSuccess(c, gin.H{
		"trade_no":    topUp.TradeNo,
		"gateway_url": payment.GatewayURL,
		"params":      payment.Params,
	})
}

func RequestWechatNativePay(c *gin.Context) {
	if !isWechatPayTopUpEnabled() {
		common.ApiErrorMsg(c, "WeChat Pay is not available")
		return
	}
	var req directPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "invalid parameters")
		return
	}
	topUp, err := createOfficialTopUp(c, model.PaymentProviderWechatPay, model.PaymentMethodWechatNative, req.Amount)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	order, err := service.BuildWeChatNativeOrder(service.WeChatNativeOrderParams{
		AppID:       setting.WeChatPayAppID,
		MerchantID:  setting.WeChatPayMerchantID,
		TradeNo:     topUp.TradeNo,
		Description: fmt.Sprintf("Wallet topup %d", topUp.Amount),
		NotifyURL:   officialPaymentNotifyURL("/api/wechat/notify"),
		AmountCents: topUp.PaymentAmountCents,
		ExpiresAt:   time.Unix(topUp.ExpireTime, 0),
	})
	if err != nil {
		_ = model.UpdatePendingTopUpStatus(topUp.TradeNo, model.PaymentProviderWechatPay, common.TopUpStatusFailed)
		common.ApiErrorMsg(c, "create payment failed")
		return
	}
	client, err := newWeChatNativeClient(c.Request.Context(), wechatPayConfig())
	if err != nil {
		_ = model.UpdatePendingTopUpStatus(topUp.TradeNo, model.PaymentProviderWechatPay, common.TopUpStatusFailed)
		logger.LogError(c.Request.Context(), fmt.Sprintf("create wechat client failed trade_no=%s error=%q", topUp.TradeNo, err.Error()))
		common.ApiErrorMsg(c, "create payment failed")
		return
	}
	codeURL, err := client.CreateNative(c.Request.Context(), *order)
	if err != nil {
		_ = model.UpdatePendingTopUpStatus(topUp.TradeNo, model.PaymentProviderWechatPay, common.TopUpStatusFailed)
		logger.LogError(c.Request.Context(), fmt.Sprintf("create wechat native payment failed trade_no=%s error=%q", topUp.TradeNo, err.Error()))
		common.ApiErrorMsg(c, "create payment failed")
		return
	}
	common.ApiSuccess(c, gin.H{
		"trade_no":   topUp.TradeNo,
		"code_url":   codeURL,
		"expires_at": topUp.ExpireTime,
	})
}

func AlipayNotify(c *gin.Context) {
	if !isAlipayWebhookEnabled() || c.Request.ParseForm() != nil {
		c.String(http.StatusBadRequest, "failure")
		return
	}
	values := c.Request.PostForm
	if err := service.VerifyAlipayNotification(alipayConfig(), values); err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("reject alipay notification error=%q", err.Error()))
		c.String(http.StatusUnauthorized, "failure")
		return
	}
	amountCents, err := service.ParseCNYAmountCents(values.Get("total_amount"))
	if err != nil || values.Get("app_id") != setting.AlipayAppID || values.Get("seller_id") != setting.AlipaySellerID {
		c.String(http.StatusOK, "failure")
		return
	}
	tradeNo := values.Get("out_trade_no")
	status := values.Get("trade_status")
	if status == "TRADE_CLOSED" {
		topUp := model.GetTopUpByTradeNo(tradeNo)
		if topUp == nil || topUp.PaymentAmountCents != amountCents {
			c.String(http.StatusOK, "failure")
			return
		}
		_ = model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderAlipay, common.TopUpStatusFailed)
		c.String(http.StatusOK, "success")
		return
	}
	result := service.AlipayPaymentResult{
		TradeNo:         tradeNo,
		ProviderTradeNo: values.Get("trade_no"),
		AppID:           values.Get("app_id"),
		SellerID:        values.Get("seller_id"),
		AmountCents:     amountCents,
		Status:          status,
	}
	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || service.ValidateAlipayPaymentResult(alipayConfig(), result, topUp.PaymentAmountCents) != nil {
		c.String(http.StatusOK, "failure")
		return
	}
	if err := model.RechargeOfficialPayment(tradeNo, model.PaymentProviderAlipay, result.ProviderTradeNo, result.AmountCents, c.ClientIP()); err != nil {
		if !errors.Is(err, model.ErrTopUpNotFound) && !errors.Is(err, model.ErrPaymentMethodMismatch) && !errors.Is(err, model.ErrPaymentAmountMismatch) && !errors.Is(err, model.ErrTopUpStatusInvalid) {
			logger.LogError(c.Request.Context(), fmt.Sprintf("settle alipay topup failed trade_no=%s error=%q", tradeNo, err.Error()))
			c.String(http.StatusInternalServerError, "failure")
			return
		}
	}
	c.String(http.StatusOK, "success")
}

func WechatNotify(c *gin.Context) {
	if !isWechatPayWebhookEnabled() {
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "payment disabled"})
		return
	}
	client, err := newWeChatNativeClient(c.Request.Context(), wechatPayConfig())
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("create wechat notification client failed error=%q", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "internal error"})
		return
	}
	result, err := client.VerifyNotification(c.Request.Context(), c.Request)
	if err != nil || result == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "FAIL", "message": "invalid signature"})
		return
	}
	topUp := model.GetTopUpByTradeNo(result.TradeNo)
	if topUp == nil || service.ValidateWeChatPaymentResult(wechatPayConfig(), *result, topUp.PaymentAmountCents) != nil {
		c.JSON(http.StatusOK, gin.H{"code": "FAIL", "message": "payment mismatch"})
		return
	}
	if err := model.RechargeOfficialPayment(result.TradeNo, model.PaymentProviderWechatPay, result.ProviderTradeNo, result.AmountCents, c.ClientIP()); err != nil {
		if !errors.Is(err, model.ErrTopUpNotFound) && !errors.Is(err, model.ErrPaymentMethodMismatch) && !errors.Is(err, model.ErrPaymentAmountMismatch) && !errors.Is(err, model.ErrTopUpStatusInvalid) {
			logger.LogError(c.Request.Context(), fmt.Sprintf("settle wechat topup failed trade_no=%s error=%q", result.TradeNo, err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "internal error"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "成功"})
}

func GetTopUpStatus(c *gin.Context) {
	tradeNo := c.Param("trade_no")
	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.UserId != c.GetInt("id") {
		common.ApiErrorMsg(c, "topup order not found")
		return
	}
	isOfficialPayment := topUp.PaymentProvider == model.PaymentProviderAlipay || topUp.PaymentProvider == model.PaymentProviderWechatPay
	if isOfficialPayment && topUp.Status == common.TopUpStatusPending && time.Now().Unix() >= topUp.ExpireTime {
		_ = model.UpdatePendingTopUpStatus(topUp.TradeNo, topUp.PaymentProvider, common.TopUpStatusFailed)
		topUp = model.GetTopUpByTradeNo(tradeNo)
	} else if isOfficialPayment && topUp.Status == common.TopUpStatusPending && shouldQueryDirectPayment(tradeNo) {
		queryOfficialPayment(c, topUp)
		topUp = model.GetTopUpByTradeNo(tradeNo)
	}
	common.ApiSuccess(c, gin.H{
		"trade_no":   topUp.TradeNo,
		"status":     topUp.Status,
		"expires_at": topUp.ExpireTime,
	})
}

func shouldQueryDirectPayment(tradeNo string) bool {
	stateValue, _ := directPaymentQueries.LoadOrStore(tradeNo, &directPaymentQueryState{})
	state := stateValue.(*directPaymentQueryState)
	state.mu.Lock()
	defer state.mu.Unlock()
	if time.Since(state.lastQueried) < 5*time.Second {
		return false
	}
	state.lastQueried = time.Now()
	return true
}

func queryOfficialPayment(c *gin.Context, topUp *model.TopUp) {
	if topUp == nil {
		return
	}
	switch topUp.PaymentProvider {
	case model.PaymentProviderAlipay:
		client, err := newAlipayDirectClient(alipayConfig())
		if err != nil {
			return
		}
		result, err := client.QueryTrade(c.Request.Context(), topUp.TradeNo)
		if err == nil && result != nil {
			if service.ValidateAlipayPaymentResult(alipayConfig(), *result, topUp.PaymentAmountCents) == nil {
				_ = model.RechargeOfficialPayment(topUp.TradeNo, model.PaymentProviderAlipay, result.ProviderTradeNo, result.AmountCents, c.ClientIP())
			} else if result.Status == "TRADE_CLOSED" && service.ValidateAlipayPaymentIdentity(alipayConfig(), *result, topUp.PaymentAmountCents) == nil {
				_ = model.UpdatePendingTopUpStatus(topUp.TradeNo, model.PaymentProviderAlipay, common.TopUpStatusFailed)
			}
		}
	case model.PaymentProviderWechatPay:
		client, err := newWeChatNativeClient(c.Request.Context(), wechatPayConfig())
		if err != nil {
			return
		}
		result, err := client.QueryNative(c.Request.Context(), topUp.TradeNo)
		if err == nil && result != nil {
			if service.ValidateWeChatPaymentResult(wechatPayConfig(), *result, topUp.PaymentAmountCents) == nil {
				_ = model.RechargeOfficialPayment(topUp.TradeNo, model.PaymentProviderWechatPay, result.ProviderTradeNo, result.AmountCents, c.ClientIP())
			} else if (result.Status == "CLOSED" || result.Status == "REVOKED" || result.Status == "PAYERROR") && service.ValidateWeChatPaymentIdentity(wechatPayConfig(), *result, topUp.PaymentAmountCents) == nil {
				_ = model.UpdatePendingTopUpStatus(topUp.TradeNo, model.PaymentProviderWechatPay, common.TopUpStatusFailed)
			}
		}
	}
}
