package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/the-one/service"
	"github.com/QuantumNous/the-one/setting"
	"github.com/gin-gonic/gin"
)

type alipayConfigSaveRequest struct {
	Enabled         bool   `json:"enabled"`
	AppID           string `json:"app_id"`
	SellerID        string `json:"seller_id"`
	AppPrivateKey   string `json:"app_private_key"`
	AlipayPublicKey string `json:"alipay_public_key"`
}

type wechatPayConfigSaveRequest struct {
	Enabled                   bool   `json:"enabled"`
	AppID                     string `json:"app_id"`
	MerchantID                string `json:"merchant_id"`
	MerchantCertificateSerial string `json:"merchant_certificate_serial"`
	MerchantPrivateKey        string `json:"merchant_private_key"`
	APIv3Key                  string `json:"api_v3_key"`
}

func GetDirectPaymentConfigStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"alipay": gin.H{
				"enabled":    setting.AlipayEnabled,
				"configured": alipayConfigComplete(),
			},
			"wechat": gin.H{
				"enabled":    setting.WeChatPayEnabled,
				"configured": wechatPayConfigComplete(),
			},
		},
	})
}

func SaveAlipayConfig(c *gin.Context) {
	var req alipayConfigSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "invalid parameters"})
		return
	}
	if err := service.SaveAlipayConfig(service.AlipayConfigUpdate{
		Enabled:         req.Enabled,
		AppID:           req.AppID,
		SellerID:        req.SellerID,
		AppPrivateKey:   req.AppPrivateKey,
		AlipayPublicKey: req.AlipayPublicKey,
	}); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": gin.H{"configured": alipayConfigComplete()}})
}

func SaveWechatPayConfig(c *gin.Context) {
	var req wechatPayConfigSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "invalid parameters"})
		return
	}
	if err := service.SaveWeChatPayConfig(c.Request.Context(), service.WeChatPayConfigUpdate{
		Enabled:                   req.Enabled,
		AppID:                     req.AppID,
		MerchantID:                req.MerchantID,
		MerchantCertificateSerial: req.MerchantCertificateSerial,
		MerchantPrivateKey:        req.MerchantPrivateKey,
		APIv3Key:                  req.APIv3Key,
	}); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": gin.H{"configured": wechatPayConfigComplete()}})
}

func alipayConfigComplete() bool {
	return hasSecurePaymentCallback() &&
		strings.TrimSpace(setting.AlipayAppID) != "" &&
		strings.TrimSpace(setting.AlipaySellerID) != "" &&
		strings.TrimSpace(setting.AlipayAppPrivateKey) != "" &&
		strings.TrimSpace(setting.AlipayPublicKey) != ""
}

func wechatPayConfigComplete() bool {
	return hasSecurePaymentCallback() &&
		strings.TrimSpace(setting.WeChatPayAppID) != "" &&
		strings.TrimSpace(setting.WeChatPayMerchantID) != "" &&
		strings.TrimSpace(setting.WeChatPayMerchantCertificateSerial) != "" &&
		strings.TrimSpace(setting.WeChatPayMerchantPrivateKey) != "" &&
		len(setting.WeChatPayAPIv3Key) == 32
}
