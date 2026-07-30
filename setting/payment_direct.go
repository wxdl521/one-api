package setting

// Official direct-payment credentials are persisted as options. Secret fields
// are intentionally omitted from the generic options response.
var (
	AlipayEnabled       bool
	AlipayAppID         string
	AlipaySellerID      string
	AlipayAppPrivateKey string
	AlipayPublicKey     string

	WeChatPayEnabled                   bool
	WeChatPayAppID                     string
	WeChatPayMerchantID                string
	WeChatPayMerchantCertificateSerial string
	WeChatPayMerchantPrivateKey        string
	WeChatPayAPIv3Key                  string
)
