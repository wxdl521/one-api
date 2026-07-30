package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	payments "github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
)

const wechatPayCurrencyCNY = "CNY"

const WeChatPaymentStatusSuccess = "SUCCESS"

type WeChatPayConfig struct {
	AppID                     string
	MerchantID                string
	MerchantCertificateSerial string
	MerchantPrivateKey        string
	APIv3Key                  string
}

// WeChatNativeOrderParams is the provider-neutral data required to create a
// WeChat Pay Native transaction. A concrete SDK client consumes this payload.
type WeChatNativeOrderParams struct {
	AppID       string
	MerchantID  string
	TradeNo     string
	Description string
	NotifyURL   string
	AmountCents int64
	ExpiresAt   time.Time
}

type WeChatNativeOrder struct {
	AppID       string
	MerchantID  string
	TradeNo     string
	Description string
	NotifyURL   string
	AmountCents int64
	Currency    string
	ExpiresAt   time.Time
}

type WeChatPaymentResult struct {
	TradeNo         string
	ProviderTradeNo string
	AppID           string
	MerchantID      string
	AmountCents     int64
	Currency        string
	Status          string
}

type WeChatNativeClient interface {
	CreateNative(context.Context, WeChatNativeOrder) (string, error)
	QueryNative(context.Context, string) (*WeChatPaymentResult, error)
	VerifyNotification(context.Context, *http.Request) (*WeChatPaymentResult, error)
}

type weChatNativeClient struct {
	merchantID string
	service    native.NativeApiService
	notify     *notify.Handler
}

func BuildWeChatNativeOrder(params WeChatNativeOrderParams) (*WeChatNativeOrder, error) {
	if strings.TrimSpace(params.AppID) == "" || strings.TrimSpace(params.MerchantID) == "" || strings.TrimSpace(params.TradeNo) == "" {
		return nil, errors.New("wechat pay app id, merchant id, and trade number are required")
	}
	if params.AmountCents <= 0 {
		return nil, errors.New("wechat pay amount must be positive")
	}
	if params.ExpiresAt.IsZero() {
		return nil, errors.New("wechat pay expiry is required")
	}
	if _, err := url.ParseRequestURI(params.NotifyURL); err != nil {
		return nil, errors.New("invalid wechat pay notify url")
	}
	return &WeChatNativeOrder{
		AppID:       params.AppID,
		MerchantID:  params.MerchantID,
		TradeNo:     params.TradeNo,
		Description: params.Description,
		NotifyURL:   params.NotifyURL,
		AmountCents: params.AmountCents,
		Currency:    wechatPayCurrencyCNY,
		ExpiresAt:   params.ExpiresAt,
	}, nil
}

// NewWeChatNativeClient uses WeChat Pay's official SDK for request signing,
// platform-certificate refresh, response verification, and notification
// verification/decryption.
func NewWeChatNativeClient(ctx context.Context, config WeChatPayConfig) (WeChatNativeClient, error) {
	if strings.TrimSpace(config.AppID) == "" || strings.TrimSpace(config.MerchantID) == "" ||
		strings.TrimSpace(config.MerchantCertificateSerial) == "" || strings.TrimSpace(config.MerchantPrivateKey) == "" ||
		len(config.APIv3Key) != 32 {
		return nil, errors.New("incomplete wechat pay configuration")
	}
	privateKey, err := parseRSAPrivateKey(config.MerchantPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("parse wechat pay merchant private key: %w", err)
	}
	client, err := core.NewClient(ctx,
		option.WithWechatPayAutoAuthCipher(
			config.MerchantID,
			config.MerchantCertificateSerial,
			privateKey,
			config.APIv3Key,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create wechat pay client: %w", err)
	}
	notifyHandler, err := notify.NewRSANotifyHandler(
		config.APIv3Key,
		verifiers.NewSHA256WithRSAVerifier(
			downloader.MgrInstance().GetCertificateVisitor(config.MerchantID),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create wechat pay notification handler: %w", err)
	}
	return &weChatNativeClient{
		merchantID: config.MerchantID,
		service:    native.NativeApiService{Client: client},
		notify:     notifyHandler,
	}, nil
}

func (client *weChatNativeClient) CreateNative(ctx context.Context, order WeChatNativeOrder) (string, error) {
	response, _, err := client.service.Prepay(ctx, native.PrepayRequest{
		Appid:       core.String(order.AppID),
		Mchid:       core.String(order.MerchantID),
		Description: core.String(order.Description),
		OutTradeNo:  core.String(order.TradeNo),
		TimeExpire:  core.Time(order.ExpiresAt),
		NotifyUrl:   core.String(order.NotifyURL),
		Amount: &native.Amount{
			Currency: core.String(order.Currency),
			Total:    core.Int64(order.AmountCents),
		},
	})
	if err != nil {
		return "", fmt.Errorf("create wechat native payment: %w", err)
	}
	if response == nil || response.CodeUrl == nil || strings.TrimSpace(*response.CodeUrl) == "" {
		return "", errors.New("wechat pay returned empty code url")
	}
	return *response.CodeUrl, nil
}

func (client *weChatNativeClient) QueryNative(ctx context.Context, tradeNo string) (*WeChatPaymentResult, error) {
	if strings.TrimSpace(tradeNo) == "" {
		return nil, errors.New("wechat pay trade number is required")
	}
	response, _, err := client.service.QueryOrderByOutTradeNo(ctx, native.QueryOrderByOutTradeNoRequest{
		OutTradeNo: core.String(tradeNo),
		Mchid:      core.String(client.merchantID),
	})
	if err != nil {
		return nil, fmt.Errorf("query wechat native payment: %w", err)
	}
	return normalizeWeChatPaymentQueryResult(response)
}

func (client *weChatNativeClient) VerifyNotification(ctx context.Context, request *http.Request) (*WeChatPaymentResult, error) {
	if request == nil {
		return nil, errors.New("missing wechat pay notification request")
	}
	content := &payments.Transaction{}
	if _, err := client.notify.ParseNotifyRequest(ctx, request, content); err != nil {
		return nil, fmt.Errorf("verify wechat pay notification: %w", err)
	}
	return normalizeWeChatPaymentResult(content)
}

func ValidateWeChatPaymentResult(config WeChatPayConfig, result WeChatPaymentResult, expectedCents int64) error {
	if result.Status != WeChatPaymentStatusSuccess {
		return errors.New("wechat pay transaction is not successful")
	}
	if result.TradeNo == "" || result.ProviderTradeNo == "" {
		return errors.New("wechat pay transaction identifiers are required")
	}
	if result.AppID != config.AppID || result.MerchantID != config.MerchantID {
		return errors.New("wechat pay merchant identity mismatch")
	}
	if result.Currency != wechatPayCurrencyCNY || result.AmountCents != expectedCents {
		return errors.New("wechat pay amount mismatch")
	}
	return nil
}

func ValidateWeChatPaymentIdentity(config WeChatPayConfig, result WeChatPaymentResult, expectedCents int64) error {
	if result.TradeNo == "" {
		return errors.New("wechat pay trade number is required")
	}
	if result.AppID != config.AppID || result.MerchantID != config.MerchantID {
		return errors.New("wechat pay merchant identity mismatch")
	}
	if result.Currency != wechatPayCurrencyCNY || result.AmountCents != expectedCents {
		return errors.New("wechat pay amount mismatch")
	}
	return nil
}

func normalizeWeChatPaymentResult(transaction *payments.Transaction) (*WeChatPaymentResult, error) {
	if transaction == nil || transaction.Amount == nil || transaction.Appid == nil || transaction.Mchid == nil ||
		transaction.OutTradeNo == nil || transaction.TransactionId == nil || transaction.TradeState == nil ||
		transaction.Amount.Currency == nil || transaction.Amount.Total == nil {
		return nil, errors.New("incomplete wechat pay transaction")
	}
	return &WeChatPaymentResult{
		TradeNo:         *transaction.OutTradeNo,
		ProviderTradeNo: *transaction.TransactionId,
		AppID:           *transaction.Appid,
		MerchantID:      *transaction.Mchid,
		AmountCents:     *transaction.Amount.Total,
		Currency:        *transaction.Amount.Currency,
		Status:          *transaction.TradeState,
	}, nil
}

func normalizeWeChatPaymentQueryResult(transaction *payments.Transaction) (*WeChatPaymentResult, error) {
	if transaction == nil || transaction.Amount == nil || transaction.Appid == nil || transaction.Mchid == nil ||
		transaction.OutTradeNo == nil || transaction.TradeState == nil || transaction.Amount.Currency == nil || transaction.Amount.Total == nil {
		return nil, errors.New("incomplete wechat pay transaction")
	}
	providerTradeNo := ""
	if transaction.TransactionId != nil {
		providerTradeNo = *transaction.TransactionId
	}
	return &WeChatPaymentResult{
		TradeNo:         *transaction.OutTradeNo,
		ProviderTradeNo: providerTradeNo,
		AppID:           *transaction.Appid,
		MerchantID:      *transaction.Mchid,
		AmountCents:     *transaction.Amount.Total,
		Currency:        *transaction.Amount.Currency,
		Status:          *transaction.TradeState,
	}, nil
}
