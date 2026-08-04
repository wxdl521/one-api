package model

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCreateMiniTextTestAttemptIsIdempotentAndStoresNoPrompt(t *testing.T) {
	previousDB := DB
	previousDatabaseType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&MiniTextTestAttempt{}))
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
	})

	now := time.Now().UTC().Truncate(time.Second)
	first, created, err := CreateMiniTextTestAttempt(MiniTextTestAttemptCreate{
		UserID:          17,
		ClientRequestID: "request-123",
		Model:           "gpt-mini",
		InputHMAC:       "9f83a0d1469a9daff6c8fd7de5fe567a5be04d0f8a97f10af46369c3a4e4f63b",
		CreatedAt:       now,
		ExpiresAt:       now.Add(24 * time.Hour),
	})
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, MiniTextTestAttemptStateRunning, first.State)
	require.Equal(t, "gpt-mini", first.Model)
	assert.NotContains(t, common.GetJsonString(first), "the prompt must never persist")

	second, created, err := CreateMiniTextTestAttempt(MiniTextTestAttemptCreate{
		UserID:          17,
		ClientRequestID: "request-123",
		Model:           "another-model",
		InputHMAC:       "f883a0d1469a9daff6c8fd7de5fe567a5be04d0f8a97f10af46369c3a4e4f63b",
		CreatedAt:       now.Add(time.Minute),
		ExpiresAt:       now.Add(25 * time.Hour),
	})
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, "gpt-mini", second.Model)

	var count int64
	require.NoError(t, db.Model(&MiniTextTestAttempt{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestDeleteExpiredMiniTextTestAttemptsRetainsActiveAttempts(t *testing.T) {
	previousDB := DB
	previousDatabaseType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&MiniTextTestAttempt{}))
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
	})

	now := time.Now().UTC()
	expired := &MiniTextTestAttempt{UserID: 3, ClientRequestID: "expired-request", Model: "gpt-mini", InputHMAC: strings.Repeat("a", 64), State: MiniTextTestAttemptStateSucceeded, CreatedAt: now.Add(-25 * time.Hour), StartedAt: now.Add(-25 * time.Hour), ExpiresAt: now.Add(-time.Hour)}
	active := &MiniTextTestAttempt{UserID: 3, ClientRequestID: "active-request", Model: "gpt-mini", InputHMAC: strings.Repeat("b", 64), State: MiniTextTestAttemptStateRunning, CreatedAt: now, StartedAt: now, ExpiresAt: now.Add(time.Hour)}
	require.NoError(t, db.Create(expired).Error)
	require.NoError(t, db.Create(active).Error)

	require.NoError(t, DeleteExpiredMiniTextTestAttempts(now))
	var count int64
	require.NoError(t, db.Model(&MiniTextTestAttempt{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
	var remaining MiniTextTestAttempt
	require.NoError(t, db.First(&remaining).Error)
	assert.Equal(t, active.ID, remaining.ID)
}

func TestCompleteMiniTextTestAttemptRejectsUnsafeTerminalMetadata(t *testing.T) {
	previousDB := DB
	previousDatabaseType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&MiniTextTestAttempt{}))
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
	})

	now := time.Now().UTC()
	_, created, err := CreateMiniTextTestAttempt(MiniTextTestAttemptCreate{
		UserID:          17,
		ClientRequestID: "request-unsafe-terminal",
		Model:           "gpt-mini",
		InputHMAC:       strings.Repeat("a", 64),
		CreatedAt:       now,
		ExpiresAt:       now.Add(time.Hour),
	})
	require.NoError(t, err)
	require.True(t, created)

	_, err = CompleteMiniTextTestAttempt(
		17,
		"request-unsafe-terminal",
		MiniTextTestAttemptStateFailed,
		"",
		0,
		"upstream error containing untrusted detail",
		now,
	)
	require.ErrorIs(t, err, ErrMiniTextTestAttemptInvalid)
}
