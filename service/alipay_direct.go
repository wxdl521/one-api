package service

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/shopspring/decimal"
)

const alipayGatewayURL = "https://openapi.alipay.com/gateway.do"

// AlipayConfig holds the server-side credentials required for the computer
// website payment flow. It is intentionally kept out of controller responses.
type AlipayConfig struct {
	AppID           string
	SellerID        string
	AppPrivateKey   string
	AlipayPublicKey string
}

type AlipayPagePaymentRequest struct {
	TradeNo   string
	Subject   string
	Amount    string
	NotifyURL string
	ReturnURL string
}

type AlipayPagePayment struct {
	GatewayURL string
	Params     map[string]string
}

const (
	AlipayTradeStatusSuccess  = "TRADE_SUCCESS"
	AlipayTradeStatusFinished = "TRADE_FINISHED"
)

type AlipayPaymentResult struct {
	TradeNo         string
	ProviderTradeNo string
	AppID           string
	SellerID        string
	AmountCents     int64
	Status          string
}

type AlipayClient interface {
	QueryTrade(context.Context, string) (*AlipayPaymentResult, error)
}

type alipayClient struct {
	config     AlipayConfig
	httpClient *http.Client
}

func NewAlipayClient(config AlipayConfig) (AlipayClient, error) {
	if strings.TrimSpace(config.AppID) == "" || strings.TrimSpace(config.SellerID) == "" ||
		strings.TrimSpace(config.AppPrivateKey) == "" || strings.TrimSpace(config.AlipayPublicKey) == "" {
		return nil, errors.New("incomplete alipay configuration")
	}
	if _, err := parseRSAPrivateKey(config.AppPrivateKey); err != nil {
		return nil, err
	}
	if _, err := parseRSAPublicKey(config.AlipayPublicKey); err != nil {
		return nil, err
	}
	return &alipayClient{config: config, httpClient: http.DefaultClient}, nil
}

// BuildAlipayPagePayment creates a signed alipay.trade.page.pay form payload.
// The browser posts these parameters directly to Alipay; no credentials leave
// the server.
func BuildAlipayPagePayment(config AlipayConfig, request AlipayPagePaymentRequest) (*AlipayPagePayment, error) {
	if strings.TrimSpace(config.AppID) == "" || strings.TrimSpace(config.AppPrivateKey) == "" {
		return nil, errors.New("alipay app id and private key are required")
	}
	if strings.TrimSpace(request.TradeNo) == "" || strings.TrimSpace(request.Subject) == "" || strings.TrimSpace(request.Amount) == "" {
		return nil, errors.New("alipay trade number, subject, and amount are required")
	}
	if _, err := url.ParseRequestURI(request.NotifyURL); err != nil {
		return nil, fmt.Errorf("invalid alipay notify url: %w", err)
	}
	if _, err := url.ParseRequestURI(request.ReturnURL); err != nil {
		return nil, fmt.Errorf("invalid alipay return url: %w", err)
	}

	bizContent, err := common.Marshal(struct {
		OutTradeNo  string `json:"out_trade_no"`
		ProductCode string `json:"product_code"`
		TotalAmount string `json:"total_amount"`
		Subject     string `json:"subject"`
		Timeout     string `json:"timeout_express"`
	}{
		OutTradeNo:  request.TradeNo,
		ProductCode: "FAST_INSTANT_TRADE_PAY",
		TotalAmount: request.Amount,
		Subject:     request.Subject,
		Timeout:     "15m",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal alipay biz content: %w", err)
	}

	params := map[string]string{
		"app_id":      config.AppID,
		"biz_content": string(bizContent),
		"charset":     "utf-8",
		"format":      "JSON",
		"method":      "alipay.trade.page.pay",
		"notify_url":  request.NotifyURL,
		"return_url":  request.ReturnURL,
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
	}

	privateKey, err := parseRSAPrivateKey(config.AppPrivateKey)
	if err != nil {
		return nil, err
	}
	signature, err := signAlipayParams(privateKey, params)
	if err != nil {
		return nil, err
	}
	params["sign"] = signature
	return &AlipayPagePayment{GatewayURL: alipayGatewayURL, Params: params}, nil
}

// VerifyAlipayNotification verifies an RSA2 signature over Alipay's raw form
// values. Callers validate the provider-specific business fields afterwards.
func VerifyAlipayNotification(config AlipayConfig, values url.Values) error {
	if strings.TrimSpace(config.AlipayPublicKey) == "" {
		return errors.New("alipay public key is required")
	}
	if !strings.EqualFold(values.Get("sign_type"), "RSA2") {
		return errors.New("unsupported alipay sign type")
	}
	signature, err := base64.StdEncoding.DecodeString(values.Get("sign"))
	if err != nil || len(signature) == 0 {
		return errors.New("invalid alipay signature")
	}
	publicKey, err := parseRSAPublicKey(config.AlipayPublicKey)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(canonicalAlipayParams(values)))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return errors.New("alipay signature verification failed")
	}
	return nil
}

func (client *alipayClient) QueryTrade(ctx context.Context, tradeNo string) (*AlipayPaymentResult, error) {
	if strings.TrimSpace(tradeNo) == "" {
		return nil, errors.New("alipay trade number is required")
	}
	params := map[string]string{
		"app_id":    client.config.AppID,
		"charset":   "utf-8",
		"format":    "JSON",
		"method":    "alipay.trade.query",
		"sign_type": "RSA2",
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		"version":   "1.0",
	}
	bizContent, err := common.Marshal(struct {
		OutTradeNo string `json:"out_trade_no"`
	}{OutTradeNo: tradeNo})
	if err != nil {
		return nil, fmt.Errorf("marshal alipay query: %w", err)
	}
	params["biz_content"] = string(bizContent)
	privateKey, err := parseRSAPrivateKey(client.config.AppPrivateKey)
	if err != nil {
		return nil, err
	}
	params["sign"], err = signAlipayParams(privateKey, params)
	if err != nil {
		return nil, err
	}
	form := make(url.Values, len(params))
	for key, value := range params {
		form.Set(key, value)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, alipayGatewayURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create alipay query request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send alipay query: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("alipay query returned status %d", response.StatusCode)
	}
	var rawResponse map[string]json.RawMessage
	if err := common.DecodeJson(response.Body, &rawResponse); err != nil {
		return nil, fmt.Errorf("decode alipay query response: %w", err)
	}
	rawResult, ok := rawResponse["alipay_trade_query_response"]
	if !ok || len(rawResult) == 0 {
		return nil, errors.New("missing alipay query response")
	}
	signature, err := base64.StdEncoding.DecodeString(stringValue(rawResponse["sign"]))
	if err != nil || len(signature) == 0 {
		return nil, errors.New("invalid alipay query response signature")
	}
	publicKey, err := parseRSAPublicKey(client.config.AlipayPublicKey)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(rawResult)
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return nil, errors.New("alipay query response verification failed")
	}
	var result struct {
		Code       string `json:"code"`
		OutTradeNo string `json:"out_trade_no"`
		TradeNo    string `json:"trade_no"`
		AppID      string `json:"app_id"`
		SellerID   string `json:"seller_id"`
		Total      string `json:"total_amount"`
		Status     string `json:"trade_status"`
	}
	if err := common.Unmarshal(rawResult, &result); err != nil {
		return nil, fmt.Errorf("decode alipay trade result: %w", err)
	}
	if result.Code != "10000" {
		return nil, errors.New("alipay trade query was unsuccessful")
	}
	amountCents, err := ParseCNYAmountCents(result.Total)
	if err != nil {
		return nil, err
	}
	return &AlipayPaymentResult{
		TradeNo:         result.OutTradeNo,
		ProviderTradeNo: result.TradeNo,
		AppID:           result.AppID,
		SellerID:        result.SellerID,
		AmountCents:     amountCents,
		Status:          result.Status,
	}, nil
}

func ValidateAlipayPaymentResult(config AlipayConfig, result AlipayPaymentResult, expectedCents int64) error {
	if result.Status != AlipayTradeStatusSuccess && result.Status != AlipayTradeStatusFinished {
		return errors.New("alipay transaction is not successful")
	}
	if result.TradeNo == "" || result.ProviderTradeNo == "" {
		return errors.New("alipay transaction identifiers are required")
	}
	if result.AppID != config.AppID || result.SellerID != config.SellerID {
		return errors.New("alipay merchant identity mismatch")
	}
	if result.AmountCents != expectedCents {
		return errors.New("alipay amount mismatch")
	}
	return nil
}

func ValidateAlipayPaymentIdentity(config AlipayConfig, result AlipayPaymentResult, expectedCents int64) error {
	if result.TradeNo == "" {
		return errors.New("alipay trade number is required")
	}
	if result.AppID != config.AppID || result.SellerID != config.SellerID {
		return errors.New("alipay merchant identity mismatch")
	}
	if result.AmountCents != expectedCents {
		return errors.New("alipay amount mismatch")
	}
	return nil
}

func ParseCNYAmountCents(amount string) (int64, error) {
	value, err := decimal.NewFromString(strings.TrimSpace(amount))
	if err != nil || value.LessThanOrEqual(decimal.Zero) {
		return 0, errors.New("invalid cny amount")
	}
	cents := value.Mul(decimal.NewFromInt(100))
	if !cents.Equal(cents.Truncate(0)) || cents.GreaterThan(decimal.NewFromInt(math.MaxInt64)) {
		return 0, errors.New("invalid cny amount precision")
	}
	return cents.IntPart(), nil
}

func stringValue(raw json.RawMessage) string {
	var value string
	if common.Unmarshal(raw, &value) != nil {
		return ""
	}
	return value
}

func signAlipayParams(privateKey *rsa.PrivateKey, params map[string]string) (string, error) {
	digest := sha256.Sum256([]byte(canonicalAlipayParamMap(params)))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign alipay request: %w", err)
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func canonicalAlipayParams(values url.Values) string {
	params := make(map[string]string, len(values))
	for key, value := range values {
		if len(value) > 0 {
			params[key] = value[0]
		}
	}
	return canonicalAlipayParamMap(params)
}

func canonicalAlipayParamMap(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		if key != "sign" && key != "sign_type" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+params[key])
	}
	return strings.Join(parts, "&")
}

func parseRSAPrivateKey(value string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(value)))
	if block == nil {
		return nil, errors.New("invalid rsa private key pem")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("invalid rsa private key")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("rsa private key is required")
	}
	return key, nil
}

func parseRSAPublicKey(value string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(value)))
	if block == nil {
		return nil, errors.New("invalid rsa public key pem")
	}
	if certificate, err := x509.ParseCertificate(block.Bytes); err == nil {
		key, ok := certificate.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("rsa public key is required")
		}
		return key, nil
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, errors.New("invalid rsa public key")
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("rsa public key is required")
	}
	return key, nil
}
