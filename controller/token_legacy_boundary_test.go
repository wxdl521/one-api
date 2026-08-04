package controller

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedMiniAppSourceToken(t *testing.T, db *gorm.DB, userID int, name, key string) *model.Token {
	t.Helper()
	token := seedToken(t, db, userID, name, key)
	require.NoError(t, db.Model(&model.Token{}).Where("id = ?", token.Id).Update("source", model.TokenSourceMiniApp).Error)
	token.Source = model.TokenSourceMiniApp
	return token
}

func TestLegacyTokenReadEndpointsExcludeMiniAppTokens(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	legacy := seedToken(t, db, 1, "legacy", "legacy-read-key")
	miniApp := seedMiniAppSourceToken(t, db, 1, "miniapp", "miniapp-read-key")

	listContext, listRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/?p=1&size=10", nil, 1)
	GetAllTokens(listContext)
	listResponse := decodeAPIResponse(t, listRecorder)
	require.True(t, listResponse.Success)
	var page tokenPageResponse
	require.NoError(t, common.Unmarshal(listResponse.Data, &page))
	require.Len(t, page.Items, 1)
	assert.Equal(t, 1, page.Total)
	assert.Equal(t, legacy.Id, page.Items[0].ID)

	detailContext, detailRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/"+strconv.Itoa(miniApp.Id), nil, 1)
	detailContext.Params = gin.Params{{Key: "id", Value: strconv.Itoa(miniApp.Id)}}
	GetToken(detailContext)
	detailResponse := decodeAPIResponse(t, detailRecorder)
	assert.False(t, detailResponse.Success)
	assert.NotContains(t, detailRecorder.Body.String(), miniApp.Key)
}

func TestLegacyTokenRawKeyEndpointsExcludeMiniAppTokens(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	miniApp := seedMiniAppSourceToken(t, db, 1, "miniapp", "miniapp-raw-key")

	singleContext, singleRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(miniApp.Id)+"/key", nil, 1)
	singleContext.Params = gin.Params{{Key: "id", Value: strconv.Itoa(miniApp.Id)}}
	GetTokenKey(singleContext)
	singleResponse := decodeAPIResponse(t, singleRecorder)
	assert.False(t, singleResponse.Success)
	assert.NotContains(t, singleRecorder.Body.String(), miniApp.Key)
}

func TestLegacyTokenUpdateAndStatusControlsRejectMiniAppTokens(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	miniApp := seedMiniAppSourceToken(t, db, 1, "miniapp", "miniapp-control-key")

	updateContext, updateRecorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", map[string]any{
		"id": miniApp.Id, "name": "changed", "expired_time": -1, "remain_quota": 100,
		"unlimited_quota": true, "model_limits_enabled": false, "model_limits": "", "group": "default",
	}, 1)
	UpdateToken(updateContext)
	assert.False(t, decodeAPIResponse(t, updateRecorder).Success)

	statusContext, statusRecorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/?status_only=true", map[string]any{
		"id": miniApp.Id, "status": common.TokenStatusDisabled,
	}, 1)
	UpdateToken(statusContext)
	assert.False(t, decodeAPIResponse(t, statusRecorder).Success)

	var stored model.Token
	require.NoError(t, db.First(&stored, miniApp.Id).Error)
	assert.Equal(t, "miniapp", stored.Name)
	assert.Equal(t, common.TokenStatusEnabled, stored.Status)
}

func TestLegacyTokenDeletionPathsExcludeMiniAppTokens(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	legacy := seedToken(t, db, 1, "legacy", "legacy-delete-key")
	miniApp := seedMiniAppSourceToken(t, db, 1, "miniapp", "miniapp-delete-key")

	deleteContext, deleteRecorder := newAuthenticatedContext(t, http.MethodDelete, "/api/token/"+strconv.Itoa(miniApp.Id), nil, 1)
	deleteContext.Params = gin.Params{{Key: "id", Value: strconv.Itoa(miniApp.Id)}}
	DeleteToken(deleteContext)
	assert.False(t, decodeAPIResponse(t, deleteRecorder).Success)

	batchContext, batchRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/batch", map[string]any{"ids": []int{legacy.Id, miniApp.Id}}, 1)
	DeleteTokenBatch(batchContext)
	batchResponse := decodeAPIResponse(t, batchRecorder)
	require.True(t, batchResponse.Success)
	var deleted int
	require.NoError(t, common.Unmarshal(batchResponse.Data, &deleted))
	assert.Equal(t, 1, deleted)

	var storedMiniApp model.Token
	require.NoError(t, db.First(&storedMiniApp, miniApp.Id).Error)
	assert.Equal(t, miniApp.Key, storedMiniApp.Key)
	require.ErrorIs(t, db.First(&model.Token{}, legacy.Id).Error, gorm.ErrRecordNotFound)
}
