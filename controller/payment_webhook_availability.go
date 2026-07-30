package controller

import (
	"net/url"
	"strings"

	"github.com/QuantumNous/the-one/service"
	"github.com/QuantumNous/the-one/setting"
	"github.com/QuantumNous/the-one/setting/operation_setting"
)

func isPaymentComplianceConfirmed() bool {
	return operation_setting.IsPaymentComplianceConfirmed()
}

func isStripeTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	return strings.TrimSpace(setting.StripeApiSecret) != "" &&
		strings.TrimSpace(setting.StripeWebhookSecret) != "" &&
		strings.TrimSpace(setting.StripePriceId) != ""
}

func isStripeWebhookConfigured() bool {
	return strings.TrimSpace(setting.StripeWebhookSecret) != ""
}

func isStripeWebhookEnabled() bool {
	return isStripeTopUpEnabled()
}

func isCreemTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	products := strings.TrimSpace(setting.CreemProducts)
	return strings.TrimSpace(setting.CreemApiKey) != "" &&
		products != "" &&
		products != "[]"
}

func isCreemWebhookConfigured() bool {
	return strings.TrimSpace(setting.CreemWebhookSecret) != ""
}

func isCreemWebhookEnabled() bool {
	return isCreemTopUpEnabled() && isCreemWebhookConfigured()
}

func isWaffoTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	if !setting.WaffoEnabled {
		return false
	}

	return isWaffoWebhookConfigured()
}

func isWaffoWebhookConfigured() bool {
	if setting.WaffoSandbox {
		return strings.TrimSpace(setting.WaffoSandboxApiKey) != "" &&
			strings.TrimSpace(setting.WaffoSandboxPrivateKey) != "" &&
			strings.TrimSpace(setting.WaffoSandboxPublicCert) != ""
	}

	return strings.TrimSpace(setting.WaffoApiKey) != "" &&
		strings.TrimSpace(setting.WaffoPrivateKey) != "" &&
		strings.TrimSpace(setting.WaffoPublicCert) != ""
}

func isWaffoWebhookEnabled() bool {
	return isWaffoTopUpEnabled()
}

func isWaffoPancakeTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	// Presence-of-credentials = enabled. Webhook public keys ship inside
	// the SDK; mode (test/prod) is read from each event.
	return strings.TrimSpace(setting.WaffoPancakeMerchantID) != "" &&
		strings.TrimSpace(setting.WaffoPancakePrivateKey) != "" &&
		strings.TrimSpace(setting.WaffoPancakeProductID) != ""
}

func isWaffoPancakeWebhookConfigured() bool {
	return isWaffoPancakeTopUpEnabled()
}

func isWaffoPancakeWebhookEnabled() bool {
	return isWaffoPancakeTopUpEnabled()
}

func isEpayTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() {
		return false
	}
	return isEpayWebhookConfigured() && len(operation_setting.PayMethods) > 0
}

func isEpayWebhookConfigured() bool {
	return strings.TrimSpace(operation_setting.PayAddress) != "" &&
		strings.TrimSpace(operation_setting.EpayId) != "" &&
		strings.TrimSpace(operation_setting.EpayKey) != ""
}

func isEpayWebhookEnabled() bool {
	return isEpayTopUpEnabled()
}

func isAlipayTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() || !setting.AlipayEnabled || !hasSecurePaymentCallback() {
		return false
	}
	return strings.TrimSpace(setting.AlipayAppID) != "" &&
		strings.TrimSpace(setting.AlipaySellerID) != "" &&
		strings.TrimSpace(setting.AlipayAppPrivateKey) != "" &&
		strings.TrimSpace(setting.AlipayPublicKey) != ""
}

func isAlipayWebhookEnabled() bool {
	return isAlipayTopUpEnabled()
}

func isWechatPayTopUpEnabled() bool {
	if !isPaymentComplianceConfirmed() || !setting.WeChatPayEnabled || !hasSecurePaymentCallback() {
		return false
	}
	return strings.TrimSpace(setting.WeChatPayAppID) != "" &&
		strings.TrimSpace(setting.WeChatPayMerchantID) != "" &&
		strings.TrimSpace(setting.WeChatPayMerchantCertificateSerial) != "" &&
		strings.TrimSpace(setting.WeChatPayMerchantPrivateKey) != "" &&
		len(setting.WeChatPayAPIv3Key) == 32
}

func isWechatPayWebhookEnabled() bool {
	return isWechatPayTopUpEnabled()
}

func hasSecurePaymentCallback() bool {
	callbackAddress, err := url.Parse(service.GetCallbackAddress())
	return err == nil && callbackAddress.Scheme == "https" && callbackAddress.Host != ""
}
