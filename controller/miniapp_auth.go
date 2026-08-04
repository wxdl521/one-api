package controller

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/middleware"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
)

const (
	miniAppMaxCodeLength   = 2048
	miniAppMaxTicketLength = 512
	miniAppMaxSIDLength    = 64
	miniAppRequestMaxBytes = 8 << 10
)

func MiniAppWechatLogin(c *gin.Context) {
	fields, ok := decodeMiniAppRequest(c, "code")
	if !ok {
		return
	}
	code, ok := miniAppRequiredString(c, fields, "code", miniAppMaxCodeLength)
	if !ok {
		return
	}
	result, err := service.StartMiniAppLogin(c.Request.Context(), code, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func MiniAppRegister(c *gin.Context) {
	fields, ok := decodeMiniAppRequest(c, "pending_identity_ticket", "username", "password")
	if !ok {
		return
	}
	pendingTicket, ok := miniAppRequiredString(c, fields, "pending_identity_ticket", miniAppMaxTicketLength)
	if !ok {
		return
	}
	username, ok := miniAppRequiredString(c, fields, "username", model.UserNameMaxLength)
	if !ok {
		return
	}
	password, ok := miniAppRequiredString(c, fields, "password", 20)
	if !ok {
		return
	}
	bundle, err := service.RegisterMiniAppUser(pendingTicket, service.MiniAppRegistration{
		Username: username,
		Password: password,
	}, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, bundle)
}

func MiniAppStartBinding(c *gin.Context) {
	fields, ok := decodeMiniAppRequest(c, "pending_identity_ticket")
	if !ok {
		return
	}
	pendingTicket, ok := miniAppRequiredString(c, fields, "pending_identity_ticket", miniAppMaxTicketLength)
	if !ok {
		return
	}
	binding, err := service.CreateMiniAppBinding(pendingTicket)
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, binding)
}

func MiniAppBindingStatus(c *gin.Context) {
	bindingID := strings.TrimSpace(c.Param("id"))
	if bindingID == "" || len(bindingID) > miniAppMaxSIDLength {
		writeMiniAppInvalidRequest(c)
		return
	}
	pendingTicket, ok := miniAppAuthorizationTicket(c)
	if !ok {
		return
	}
	status, err := service.GetMiniAppBindingStatusForBinding(pendingTicket, bindingID)
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": bindingID, "status": status})
}

func MiniAppRenewLogin(c *gin.Context) {
	fields, ok := decodeMiniAppRequest(c, "code", "sid")
	if !ok {
		return
	}
	code, ok := miniAppRequiredString(c, fields, "code", miniAppMaxCodeLength)
	if !ok {
		return
	}
	sid, ok := miniAppRequiredString(c, fields, "sid", miniAppMaxSIDLength)
	if !ok {
		return
	}
	bundle, err := service.RenewMiniAppLogin(c.Request.Context(), code, sid, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, bundle)
}

func MiniAppLogout(c *gin.Context) {
	revoked, err := model.RevokeUserSession(c.GetInt("id"), c.GetString("session_id"), "miniapp_logout")
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"revoked_sid": c.GetString("session_id"),
		"revoked":     revoked,
	})
}

func ConfirmMiniAppBrowserBinding(c *gin.Context) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		writeMiniAppBrowserSessionRequired(c)
		return
	}
	session, _, err := service.ValidateLoginSession(identity)
	if err != nil || session.LoginMethod == "wechat-miniapp" {
		writeMiniAppBrowserSessionRequired(c)
		return
	}
	fields, ok := decodeMiniAppRequest(c, "binding_ticket")
	if !ok {
		return
	}
	bindingTicket, ok := miniAppRequiredString(c, fields, "binding_ticket", miniAppMaxTicketLength)
	if !ok {
		return
	}
	if err := service.ConfirmMiniAppBinding(bindingTicket, identity); err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"confirmed": true})
}

func decodeMiniAppRequest(c *gin.Context, allowedFields ...string) (map[string]json.RawMessage, bool) {
	return decodeMiniAppRequestWithMaxBytes(c, miniAppRequestMaxBytes, allowedFields...)
}

func decodeMiniAppRequestWithMaxBytes(c *gin.Context, maxBytes int, allowedFields ...string) (map[string]json.RawMessage, bool) {
	if maxBytes <= 0 {
		writeMiniAppInvalidRequest(c)
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, int64(maxBytes)+1))
	if err != nil || len(body) > maxBytes {
		writeMiniAppInvalidRequest(c)
		return nil, false
	}
	fields := make(map[string]json.RawMessage)
	if err := common.Unmarshal(body, &fields); err != nil {
		writeMiniAppInvalidRequest(c)
		return nil, false
	}
	allowed := make(map[string]struct{}, len(allowedFields))
	for _, field := range allowedFields {
		allowed[field] = struct{}{}
	}
	for field := range fields {
		if _, ok := allowed[field]; !ok {
			writeMiniAppInvalidRequest(c)
			return nil, false
		}
	}
	return fields, true
}

func miniAppRequiredString(c *gin.Context, fields map[string]json.RawMessage, field string, maxLength int) (string, bool) {
	raw, ok := fields[field]
	if !ok {
		writeMiniAppInvalidRequest(c)
		return "", false
	}
	var value string
	if err := common.Unmarshal(raw, &value); err != nil {
		writeMiniAppInvalidRequest(c)
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxLength {
		writeMiniAppInvalidRequest(c)
		return "", false
	}
	return value, true
}

func miniAppAuthorizationTicket(c *gin.Context) (string, bool) {
	parts := strings.Fields(c.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		writeMiniAppInvalidRequest(c)
		return "", false
	}
	return miniAppOpaqueValue(c, parts[1])
}

func miniAppOpaqueValue(c *gin.Context, value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > miniAppMaxTicketLength {
		writeMiniAppInvalidRequest(c)
		return "", false
	}
	return value, true
}

func writeMiniAppInvalidRequest(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{
		"success": false,
		"code":    "MINIAPP_INVALID_REQUEST",
		"message": http.StatusText(http.StatusBadRequest),
	})
}

func writeMiniAppBrowserSessionRequired(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{
		"success": false,
		"code":    "MINIAPP_BROWSER_SESSION_REQUIRED",
		"message": http.StatusText(http.StatusForbidden),
	})
}

func writeMiniAppError(c *gin.Context, err error) {
	status, code := service.MiniAppAuthErrorCode(err)
	if code == "MINIAPP_INTERNAL_ERROR" {
		common.SysError("mini program request failed: " + err.Error())
	}
	c.JSON(status, gin.H{
		"success": false,
		"code":    code,
		"message": http.StatusText(status),
	})
}
