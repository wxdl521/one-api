package service

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRSAPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der := x509.MarshalPKCS1PrivateKey(key)
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}))
}

func testRSAPublicKeyPEM(t *testing.T, privateKeyPEM string) string {
	t.Helper()
	block, _ := pem.Decode([]byte(privateKeyPEM))
	require.NotNil(t, block)
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func signAlipayValues(t *testing.T, privateKeyPEM string, values url.Values) string {
	t.Helper()
	keys := make([]string, 0, len(values))
	for key := range values {
		if key != "sign" && key != "sign_type" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	block, _ := pem.Decode([]byte(privateKeyPEM))
	require.NotNil(t, block)
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)
	digest := sha256.Sum256([]byte(strings.Join(parts, "&")))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(signature)
}

func TestBuildAlipayPagePaymentSignsOfficialDesktopPayment(t *testing.T) {
	privateKey := testRSAPrivateKeyPEM(t)
	publicKey := testRSAPublicKeyPEM(t, privateKey)

	payment, err := BuildAlipayPagePayment(AlipayConfig{
		AppID:           "2026000000000000",
		AppPrivateKey:   privateKey,
		AlipayPublicKey: publicKey,
	}, AlipayPagePaymentRequest{
		TradeNo:   "USR1NO202607300001",
		Subject:   "TUC10",
		Amount:    "7.30",
		NotifyURL: "https://api.example.com/api/alipay/notify",
		ReturnURL: "https://api.example.com/wallet?show_history=true",
	})

	require.NoError(t, err)
	assert.Equal(t, alipayGatewayURL, payment.GatewayURL)
	assert.Equal(t, "alipay.trade.page.pay", payment.Params["method"])
	assert.Equal(t, "RSA2", payment.Params["sign_type"])
	assert.Equal(t, "https://api.example.com/api/alipay/notify", payment.Params["notify_url"])
	assert.Contains(t, payment.Params["biz_content"], `"out_trade_no":"USR1NO202607300001"`)
	assert.Contains(t, payment.Params["biz_content"], `"total_amount":"7.30"`)

	values := make(url.Values, len(payment.Params))
	for key, value := range payment.Params {
		values.Set(key, value)
	}
	assert.NoError(t, VerifyAlipayNotification(AlipayConfig{AlipayPublicKey: publicKey}, values))
}

func TestVerifyAlipayNotificationRejectsTamperedPayload(t *testing.T) {
	privateKey := testRSAPrivateKeyPEM(t)
	publicKey := testRSAPublicKeyPEM(t, privateKey)
	values := url.Values{
		"out_trade_no": {"USR1NO202607300001"},
		"trade_no":     {"202607302200123456789"},
		"total_amount": {"7.30"},
		"app_id":       {"2026000000000000"},
		"seller_id":    {"2088000000000000"},
		"trade_status": {"TRADE_SUCCESS"},
		"sign_type":    {"RSA2"},
	}
	values.Set("sign", signAlipayValues(t, privateKey, values))
	values.Set("total_amount", "73.00")

	err := VerifyAlipayNotification(AlipayConfig{AlipayPublicKey: publicKey}, values)
	require.Error(t, err)
}

func TestValidateAlipayPaymentResultRejectsIdentityAndAmountMismatches(t *testing.T) {
	config := AlipayConfig{AppID: "app", SellerID: "seller"}
	valid := AlipayPaymentResult{
		TradeNo:         "order-1",
		ProviderTradeNo: "provider-1",
		AppID:           "app",
		SellerID:        "seller",
		AmountCents:     730,
		Status:          AlipayTradeStatusSuccess,
	}
	require.NoError(t, ValidateAlipayPaymentResult(config, valid, 730))

	wrongApp := valid
	wrongApp.AppID = "other"
	require.Error(t, ValidateAlipayPaymentResult(config, wrongApp, 730))

	wrongSeller := valid
	wrongSeller.SellerID = "other"
	require.Error(t, ValidateAlipayPaymentResult(config, wrongSeller, 730))

	wrongAmount := valid
	wrongAmount.AmountCents = 731
	require.Error(t, ValidateAlipayPaymentResult(config, wrongAmount, 730))
}

func TestParseCNYAmountCentsRejectsFractionsBelowFen(t *testing.T) {
	cents, err := ParseCNYAmountCents("7.30")
	require.NoError(t, err)
	assert.Equal(t, int64(730), cents)

	_, err = ParseCNYAmountCents("7.301")
	require.Error(t, err)
}
