package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLegacyTokenBoundaryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := DB
	previousDatabaseType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	previousCommonGroupCol := commonGroupCol
	previousCommonKeyCol := commonKeyCol
	previousCommonTrueVal := commonTrueVal
	previousCommonFalseVal := commonFalseVal
	previousLogGroupCol := logGroupCol
	previousLogKeyCol := logKeyCol
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Token{}))
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	initCol()
	common.RedisEnabled = false
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
		common.RedisEnabled = previousRedis
		commonGroupCol = previousCommonGroupCol
		commonKeyCol = previousCommonKeyCol
		commonTrueVal = previousCommonTrueVal
		commonFalseVal = previousCommonFalseVal
		logGroupCol = previousLogGroupCol
		logKeyCol = previousLogKeyCol
	})
	return db
}

func seedLegacyBoundaryToken(t *testing.T, db *gorm.DB, userID int, key, source string) *Token {
	t.Helper()
	token := &Token{
		UserId:         userID,
		Key:            key,
		Name:           key,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    1,
		AccessedTime:   1,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
		Source:         source,
	}
	require.NoError(t, db.Create(token).Error)
	return token
}

func TestLegacyTokenLookupsExcludeMiniAppTokens(t *testing.T) {
	db := setupLegacyTokenBoundaryTestDB(t)
	legacy := seedLegacyBoundaryToken(t, db, 1, "legacy-key", "")
	miniApp := seedLegacyBoundaryToken(t, db, 1, "miniapp-key", TokenSourceMiniApp)

	tokens, err := GetAllUserTokens(1, 0, 10)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	assert.Equal(t, legacy.Id, tokens[0].Id)

	searchResults, total, err := SearchUserTokens(1, "", "", 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, searchResults, 1)
	assert.Equal(t, legacy.Id, searchResults[0].Id)

	found, err := GetTokenByIds(miniApp.Id, 1)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	assert.Empty(t, found.Key)
}

func TestLegacyTokenKeyExportsExcludeMiniAppTokens(t *testing.T) {
	db := setupLegacyTokenBoundaryTestDB(t)
	legacy := seedLegacyBoundaryToken(t, db, 1, "legacy-key", "")
	miniApp := seedLegacyBoundaryToken(t, db, 1, "miniapp-key", TokenSourceMiniApp)

	tokens, err := GetTokenKeysByIds([]int{legacy.Id, miniApp.Id}, 1)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	assert.Equal(t, legacy.Id, tokens[0].Id)
	assert.Equal(t, legacy.Key, tokens[0].Key)
}

func TestLegacyTokenDeletionExcludesMiniAppTokens(t *testing.T) {
	db := setupLegacyTokenBoundaryTestDB(t)
	legacy := seedLegacyBoundaryToken(t, db, 1, "legacy-key", "")
	miniApp := seedLegacyBoundaryToken(t, db, 1, "miniapp-key", TokenSourceMiniApp)

	require.ErrorIs(t, DeleteTokenById(miniApp.Id, 1), gorm.ErrRecordNotFound)

	var storedMiniApp Token
	require.NoError(t, db.First(&storedMiniApp, miniApp.Id).Error)
	assert.Equal(t, miniApp.Key, storedMiniApp.Key)

	deleted, err := BatchDeleteTokens([]int{legacy.Id, miniApp.Id}, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)

	require.ErrorIs(t, db.First(&Token{}, legacy.Id).Error, gorm.ErrRecordNotFound)
	require.NoError(t, db.First(&storedMiniApp, miniApp.Id).Error)
	assert.Equal(t, miniApp.Key, storedMiniApp.Key)
}
