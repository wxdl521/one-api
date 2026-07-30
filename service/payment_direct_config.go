package service

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/setting"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
)

type AlipayConfigUpdate struct {
	Enabled         bool
	AppID           string
	SellerID        string
	AppPrivateKey   string
	AlipayPublicKey string
}

type WeChatPayConfigUpdate struct {
	Enabled                   bool
	AppID                     string
	MerchantID                string
	MerchantCertificateSerial string
	MerchantPrivateKey        string
	APIv3Key                  string
}

func SaveAlipayConfig(update AlipayConfigUpdate) error {
	appID := strings.TrimSpace(update.AppID)
	sellerID := strings.TrimSpace(update.SellerID)
	appPrivateKey := strings.TrimSpace(update.AppPrivateKey)
	alipayPublicKey := strings.TrimSpace(update.AlipayPublicKey)
	if appPrivateKey == "" {
		appPrivateKey = setting.AlipayAppPrivateKey
	}
	if alipayPublicKey == "" {
		alipayPublicKey = setting.AlipayPublicKey
	}
	if update.Enabled {
		if appID == "" || sellerID == "" || appPrivateKey == "" || alipayPublicKey == "" {
			return errors.New("incomplete alipay configuration")
		}
		if err := validateOfficialPaymentCallbackAddress(); err != nil {
			return err
		}
		if _, err := parseRSAPrivateKey(appPrivateKey); err != nil {
			return err
		}
		if _, err := parseRSAPublicKey(alipayPublicKey); err != nil {
			return err
		}
	}
	return model.UpdateOptionsBulk(map[string]string{
		"AlipayEnabled":       strconv.FormatBool(update.Enabled),
		"AlipayAppID":         appID,
		"AlipaySellerID":      sellerID,
		"AlipayAppPrivateKey": appPrivateKey,
		"AlipayPublicKey":     alipayPublicKey,
	})
}

func SaveWeChatPayConfig(ctx context.Context, update WeChatPayConfigUpdate) error {
	appID := strings.TrimSpace(update.AppID)
	merchantID := strings.TrimSpace(update.MerchantID)
	certificateSerial := strings.TrimSpace(update.MerchantCertificateSerial)
	merchantPrivateKey := strings.TrimSpace(update.MerchantPrivateKey)
	apiV3Key := strings.TrimSpace(update.APIv3Key)
	if merchantPrivateKey == "" {
		merchantPrivateKey = setting.WeChatPayMerchantPrivateKey
	}
	if apiV3Key == "" {
		apiV3Key = setting.WeChatPayAPIv3Key
	}
	if update.Enabled {
		if appID == "" || merchantID == "" || certificateSerial == "" || merchantPrivateKey == "" || len(apiV3Key) != 32 {
			return errors.New("incomplete wechat pay configuration")
		}
		if err := validateOfficialPaymentCallbackAddress(); err != nil {
			return err
		}
		if _, err := parseRSAPrivateKey(merchantPrivateKey); err != nil {
			return err
		}
	}
	oldMerchantID := setting.WeChatPayMerchantID
	if err := model.UpdateOptionsBulk(map[string]string{
		"WeChatPayEnabled":                   strconv.FormatBool(update.Enabled),
		"WeChatPayAppID":                     appID,
		"WeChatPayMerchantID":                merchantID,
		"WeChatPayMerchantCertificateSerial": certificateSerial,
		"WeChatPayMerchantPrivateKey":        merchantPrivateKey,
		"WeChatPayAPIv3Key":                  apiV3Key,
	}); err != nil {
		return err
	}
	if oldMerchantID != "" {
		downloader.MgrInstance().RemoveDownloader(ctx, oldMerchantID)
	}
	if merchantID != "" && merchantID != oldMerchantID {
		downloader.MgrInstance().RemoveDownloader(ctx, merchantID)
	}
	return nil
}

func validateOfficialPaymentCallbackAddress() error {
	callbackAddress, err := url.Parse(GetCallbackAddress())
	if err != nil || callbackAddress.Scheme != "https" || callbackAddress.Host == "" {
		return errors.New("official payment callback address must be a public https url")
	}
	return nil
}
