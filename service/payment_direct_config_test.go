package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/the-one/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestSaveOfficialPaymentConfigRejectsNonHTTPSCallbackBeforePersisting(t *testing.T) {
	originalCallback := operation_setting.CustomCallbackAddress
	t.Cleanup(func() {
		operation_setting.CustomCallbackAddress = originalCallback
	})
	operation_setting.CustomCallbackAddress = "http://api.example.com"

	privateKey := testRSAPrivateKeyPEM(t)
	publicKey := testRSAPublicKeyPEM(t, privateKey)
	err := SaveAlipayConfig(AlipayConfigUpdate{
		Enabled:         true,
		AppID:           "app",
		SellerID:        "seller",
		AppPrivateKey:   privateKey,
		AlipayPublicKey: publicKey,
	})
	require.Error(t, err)
}

func TestSaveOfficialPaymentConfigRejectsInvalidPrivateKeysBeforePersisting(t *testing.T) {
	originalCallback := operation_setting.CustomCallbackAddress
	t.Cleanup(func() {
		operation_setting.CustomCallbackAddress = originalCallback
	})
	operation_setting.CustomCallbackAddress = "https://api.example.com"

	err := SaveAlipayConfig(AlipayConfigUpdate{
		Enabled:         true,
		AppID:           "app",
		SellerID:        "seller",
		AppPrivateKey:   "not a key",
		AlipayPublicKey: "not a key",
	})
	require.Error(t, err)

	err = SaveWeChatPayConfig(context.Background(), WeChatPayConfigUpdate{
		Enabled:                   true,
		AppID:                     "wx-app",
		MerchantID:                "mch-id",
		MerchantCertificateSerial: "serial",
		MerchantPrivateKey:        "not a key",
		APIv3Key:                  "12345678901234567890123456789012",
	})
	require.Error(t, err)
}
