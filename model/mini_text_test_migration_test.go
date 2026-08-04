package model

import (
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMigrateDBCreatesMiniTextTestAttemptsTable(t *testing.T) {
	previousDB := DB
	previousDatabaseType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
	})

	require.NoError(t, migrateDB())
	assert.True(t, db.Migrator().HasTable(&MiniTextTestAttempt{}))
	assert.True(t, db.Migrator().HasIndex(&MiniTextTestAttempt{}, "idx_mini_text_test_expires_at"))
}
