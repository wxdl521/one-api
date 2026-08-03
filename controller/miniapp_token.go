package controller

import (
	"encoding/json"
	"strconv"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
)

func MiniAppListTokens(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	tokens, total, err := service.ListMiniAppTokens(c.GetInt("id"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	pageInfo.SetTotal(total)
	pageInfo.SetItems(tokens)
	common.ApiSuccess(c, pageInfo)
}

func MiniAppCreateToken(c *gin.Context) {
	fields, ok := decodeMiniAppRequest(c, "name", "group", "models", "expires_in_days")
	if !ok {
		return
	}
	var request service.MiniAppTokenCreateRequest
	if !miniAppTokenRequestField(fields, "name", &request.Name) ||
		!miniAppTokenRequestField(fields, "group", &request.Group) ||
		!miniAppTokenRequestField(fields, "models", &request.Models) ||
		!miniAppTokenRequestField(fields, "expires_in_days", &request.ExpiresInDays) {
		writeMiniAppInvalidRequest(c)
		return
	}

	created, err := service.CreateMiniAppToken(c.GetInt("id"), request)
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	common.ApiSuccess(c, created)
}

func MiniAppUpdateTokenStatus(c *gin.Context) {
	tokenID, ok := miniAppTokenID(c)
	if !ok {
		return
	}
	fields, ok := decodeMiniAppRequest(c, "status")
	if !ok {
		return
	}
	var status int
	if !miniAppTokenRequestField(fields, "status", &status) {
		writeMiniAppInvalidRequest(c)
		return
	}
	summary, err := service.UpdateMiniAppTokenStatus(c.GetInt("id"), tokenID, status)
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func MiniAppRevokeToken(c *gin.Context) {
	tokenID, ok := miniAppTokenID(c)
	if !ok {
		return
	}
	if err := service.RevokeMiniAppToken(c.GetInt("id"), tokenID); err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"revoked": true})
}

func miniAppTokenRequestField(fields map[string]json.RawMessage, name string, target any) bool {
	raw, ok := fields[name]
	return ok && common.Unmarshal(raw, target) == nil
}

func miniAppTokenID(c *gin.Context) (int, bool) {
	tokenID, err := strconv.Atoi(c.Param("id"))
	if err != nil || tokenID <= 0 {
		writeMiniAppInvalidRequest(c)
		return 0, false
	}
	return tokenID, true
}
