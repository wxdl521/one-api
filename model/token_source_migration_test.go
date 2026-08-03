package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacyTokenWithoutSource struct {
	Id     int    `gorm:"primaryKey"`
	UserId int    `gorm:"index"`
	Key    string `gorm:"type:varchar(128);uniqueIndex"`
	Name   string
}

func (legacyTokenWithoutSource) TableName() string {
	return "tokens"
}

func TestMigrateTokenSourcePreservesLegacyTokens(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&legacyTokenWithoutSource{}))
	require.NoError(t, db.Create(&legacyTokenWithoutSource{
		Id: 1, UserId: 9, Key: "legacy-token-key", Name: "legacy token",
	}).Error)

	previousDB := DB
	DB = db
	t.Cleanup(func() {
		DB = previousDB
	})
	require.NoError(t, migrateTokenSource())
	assert.True(t, db.Migrator().HasColumn(&Token{}, "source"))

	var token legacyTokenWithoutSource
	require.NoError(t, db.First(&token, 1).Error)
	assert.Equal(t, "legacy-token-key", token.Key)
}
