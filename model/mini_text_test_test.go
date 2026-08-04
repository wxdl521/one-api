package model

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var registerMiniTextTestMySQLDriver sync.Once

const miniTextTestMySQLDriverName = "mini-text-test-duplicate-upsert"

type miniTextTestMySQLDriver struct{}

func (miniTextTestMySQLDriver) Open(string) (driver.Conn, error) {
	return miniTextTestMySQLConn{}, nil
}

type miniTextTestMySQLConn struct{}

func (miniTextTestMySQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepared statements are not used by this test driver")
}

func (miniTextTestMySQLConn) Close() error { return nil }

func (miniTextTestMySQLConn) Begin() (driver.Tx, error) {
	return miniTextTestMySQLTx{}, nil
}

type miniTextTestMySQLTx struct{}

func (miniTextTestMySQLTx) Commit() error   { return nil }
func (miniTextTestMySQLTx) Rollback() error { return nil }

// ExecContext intentionally reports one affected row for every insert. This
// mirrors MySQL duplicate-key upsert result semantics that make RowsAffected
// unsuitable as an idempotency claim signal.
func (miniTextTestMySQLConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	return miniTextTestMySQLResult{}, nil
}

type miniTextTestMySQLResult struct{}

func (miniTextTestMySQLResult) LastInsertId() (int64, error) { return 7, nil }
func (miniTextTestMySQLResult) RowsAffected() (int64, error) { return 1, nil }

func (miniTextTestMySQLConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	now := time.Now().UTC()
	return &miniTextTestMySQLRows{
		columns: []string{
			"id", "user_id", "client_request_id", "model", "input_hmac", "claim_nonce", "state", "charge_reference", "charged_quota", "error_code", "created_at", "started_at", "completed_at", "expires_at", "updated_at",
		},
		values: [][]driver.Value{{
			int64(7), int64(17), "mysql-existing-request", "existing-model", strings.Repeat("a", 64), "already-claimed", MiniTextTestAttemptStateRunning, "", int64(0), "", now, now, nil, now.Add(time.Hour), now,
		}},
	}, nil
}

type miniTextTestMySQLRows struct {
	columns []string
	values  [][]driver.Value
}

func (r *miniTextTestMySQLRows) Columns() []string { return r.columns }
func (r *miniTextTestMySQLRows) Close() error      { return nil }
func (r *miniTextTestMySQLRows) Next(dest []driver.Value) error {
	if len(r.values) == 0 {
		return io.EOF
	}
	copy(dest, r.values[0])
	r.values = r.values[1:]
	return nil
}

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

func TestCreateMiniTextTestAttemptTreatsMySQLDuplicateUpsertAsExisting(t *testing.T) {
	registerMiniTextTestMySQLDriver.Do(func() {
		sql.Register(miniTextTestMySQLDriverName, miniTextTestMySQLDriver{})
	})
	sqlDB, err := sql.Open(miniTextTestMySQLDriverName, "")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	previousDB := DB
	previousDatabaseType := common.MainDatabaseType()
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeMySQL)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
	})

	now := time.Now().UTC()
	attempt, created, err := CreateMiniTextTestAttempt(MiniTextTestAttemptCreate{
		UserID:          17,
		ClientRequestID: "mysql-existing-request",
		Model:           "gpt-mini",
		InputHMAC:       strings.Repeat("b", 64),
		CreatedAt:       now,
		ExpiresAt:       now.Add(time.Hour),
	})

	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, "existing-model", attempt.Model)
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
