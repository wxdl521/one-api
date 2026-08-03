package model

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupWechatMiniIdentityTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&WechatMiniIdentity{}, &MiniAppBinding{}))
	require.NoError(t, DB.Exec("DELETE FROM miniapp_bindings").Error)
	require.NoError(t, DB.Exec("DELETE FROM wechat_mini_identities").Error)
	t.Cleanup(func() {
		_ = DB.Exec("DELETE FROM miniapp_bindings").Error
		_ = DB.Exec("DELETE FROM wechat_mini_identities").Error
	})
}

func setupConcurrentMiniAppBindingTest(t *testing.T) {
	t.Helper()
	previousDB := DB
	databasePath := filepath.ToSlash(filepath.Join(t.TempDir(), "miniapp-bindings.db"))
	db, err := gorm.Open(sqlite.Open("file:"+databasePath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(2)
	require.NoError(t, db.AutoMigrate(&WechatMiniIdentity{}, &MiniAppBinding{}))
	DB = db
	t.Cleanup(func() {
		DB = previousDB
		_ = sqlDB.Close()
	})
}

func TestWechatMiniIdentityRequiresUniqueAppAndSubjectDigest(t *testing.T) {
	setupWechatMiniIdentityTest(t)
	identity := WechatMiniIdentity{
		AppID:      "wx-miniapp-a",
		OpenIDHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		UserID:     101,
	}
	require.NoError(t, DB.Create(&identity).Error)

	duplicate := WechatMiniIdentity{
		AppID:      identity.AppID,
		OpenIDHash: identity.OpenIDHash,
		UserID:     202,
	}
	err := DB.Create(&duplicate).Error
	require.Error(t, err)
	require.NoError(t, DB.Create(&WechatMiniIdentity{
		AppID:      "wx-miniapp-b",
		OpenIDHash: identity.OpenIDHash,
		UserID:     202,
	}).Error)

	var count int64
	require.NoError(t, DB.Model(&WechatMiniIdentity{}).
		Where("app_id = ? AND open_id_hash = ?", identity.AppID, identity.OpenIDHash).
		Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestWechatMiniIdentityCannotBindSameSubjectToTwoUsers(t *testing.T) {
	setupWechatMiniIdentityTest(t)
	appID := "wx-miniapp-a"
	openIDHash := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		_, err := ClaimWechatMiniIdentityWithTx(tx, appID, openIDHash, 101)
		return err
	}))
	err := DB.Transaction(func(tx *gorm.DB) error {
		_, err := ClaimWechatMiniIdentityWithTx(tx, appID, openIDHash, 202)
		return err
	})
	assert.ErrorIs(t, err, ErrWechatMiniIdentityAlreadyBound)

	var identity WechatMiniIdentity
	require.NoError(t, DB.Where("app_id = ? AND open_id_hash = ?", appID, openIDHash).First(&identity).Error)
	assert.Equal(t, 101, identity.UserID)
}

func TestMiniAppBindingReadRequiresIDFlowAndSubjectDigest(t *testing.T) {
	setupWechatMiniIdentityTest(t)
	now := time.Now()
	binding, err := CreateMiniAppBinding(MiniAppBindingCreate{
		PendingFlowID: 77,
		AppID:         "wx-miniapp-a",
		OpenIDHash:    "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		ExpiresAt:     now.Add(time.Minute),
	})
	require.NoError(t, err)

	loaded, err := GetMiniAppBindingForPendingIdentity(binding.ID, binding.PendingFlowID, binding.AppID, binding.OpenIDHash)
	require.NoError(t, err)
	assert.Equal(t, binding.ID, loaded.ID)

	_, err = GetMiniAppBindingForPendingIdentity(binding.ID, binding.PendingFlowID+1, binding.AppID, binding.OpenIDHash)
	assert.ErrorIs(t, err, ErrMiniAppBindingInvalid)
	_, err = GetMiniAppBindingForPendingIdentity(binding.ID, binding.PendingFlowID, binding.AppID, "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")
	assert.ErrorIs(t, err, ErrMiniAppBindingInvalid)
	_, err = GetMiniAppBindingForPendingIdentity("different-binding-id", binding.PendingFlowID, binding.AppID, binding.OpenIDHash)
	assert.ErrorIs(t, err, ErrMiniAppBindingInvalid)
}

func TestExpiredMiniAppBindingsAreMarkedThenRetainedForCleanupWindow(t *testing.T) {
	setupWechatMiniIdentityTest(t)
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	staleExpiredAt := now.Add(-AuthFlowDefaultCleanupRetention - time.Minute)
	expiredPending := MiniAppBinding{
		ID:            "expired-pending-binding",
		PendingFlowID: 1,
		AppID:         "wx-miniapp-a",
		OpenIDHash:    "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		Status:        MiniAppBindingStatusPending,
		ExpiresAt:     staleExpiredAt,
	}
	staleExpired := MiniAppBinding{
		ID:            "stale-expired-binding",
		PendingFlowID: 2,
		AppID:         "wx-miniapp-a",
		OpenIDHash:    "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		Status:        MiniAppBindingStatusExpired,
		ExpiresAt:     staleExpiredAt,
		ExpiredAt:     &staleExpiredAt,
	}
	freshPending := MiniAppBinding{
		ID:            "fresh-pending-binding",
		PendingFlowID: 3,
		AppID:         "wx-miniapp-a",
		OpenIDHash:    "1111111111111111111111111111111111111111111111111111111111111111",
		Status:        MiniAppBindingStatusPending,
		ExpiresAt:     now.Add(time.Minute),
	}
	freshBoundUserID := 101
	freshBound := MiniAppBinding{
		ID:            "fresh-bound-binding",
		PendingFlowID: 4,
		AppID:         "wx-miniapp-a",
		OpenIDHash:    "2222222222222222222222222222222222222222222222222222222222222222",
		UserID:        &freshBoundUserID,
		Status:        MiniAppBindingStatusBound,
		ExpiresAt:     now.Add(time.Minute),
		BoundAt:       &now,
	}
	require.NoError(t, DB.Create(&[]MiniAppBinding{expiredPending, staleExpired, freshPending, freshBound}).Error)

	require.NoError(t, ExpireAndDeleteMiniAppBindings(now))

	var marked MiniAppBinding
	require.NoError(t, DB.First(&marked, "id = ?", expiredPending.ID).Error)
	assert.Equal(t, MiniAppBindingStatusExpired, marked.Status)
	require.NotNil(t, marked.ExpiredAt)
	assert.Equal(t, now, *marked.ExpiredAt)
	assert.ErrorIs(t, DB.First(&MiniAppBinding{}, "id = ?", staleExpired.ID).Error, gorm.ErrRecordNotFound)

	var live []MiniAppBinding
	require.NoError(t, DB.Where("id IN ?", []string{freshPending.ID, freshBound.ID}).Find(&live).Error)
	require.Len(t, live, 2)
	for _, binding := range live {
		if binding.ID == freshPending.ID {
			assert.Equal(t, MiniAppBindingStatusPending, binding.Status)
		} else {
			assert.Equal(t, MiniAppBindingStatusBound, binding.Status)
		}
	}

	require.NoError(t, ExpireAndDeleteMiniAppBindings(now.Add(AuthFlowDefaultCleanupRetention+time.Minute)))
	assert.ErrorIs(t, DB.First(&MiniAppBinding{}, "id = ?", expiredPending.ID).Error, gorm.ErrRecordNotFound)
}

func TestMiniAppBindingConcurrentConfirmationBindsExactlyOnce(t *testing.T) {
	setupConcurrentMiniAppBindingTest(t)
	binding, err := CreateMiniAppBinding(MiniAppBindingCreate{
		PendingFlowID: 88,
		AppID:         "wx-miniapp-a",
		OpenIDHash:    "3333333333333333333333333333333333333333333333333333333333333333",
		ExpiresAt:     time.Now().Add(time.Minute),
	})
	require.NoError(t, err)

	confirmation := MiniAppBindingConfirmation{
		BindingID:     binding.ID,
		PendingFlowID: binding.PendingFlowID,
		AppID:         binding.AppID,
		OpenIDHash:    binding.OpenIDHash,
		UserID:        101,
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- DB.Transaction(func(tx *gorm.DB) error {
				_, err := ConfirmMiniAppBindingWithTx(tx, confirmation)
				return err
			})
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	failures := 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		assert.ErrorIs(t, err, ErrMiniAppBindingAlreadyBound)
		failures++
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, failures)

	var stored MiniAppBinding
	require.NoError(t, DB.First(&stored, "id = ?", binding.ID).Error)
	require.NotNil(t, stored.UserID)
	assert.Equal(t, 101, *stored.UserID)
	assert.Equal(t, MiniAppBindingStatusBound, stored.Status)
}

func TestMiniAppBindingConfirmationRejectsExpiredRecord(t *testing.T) {
	setupWechatMiniIdentityTest(t)
	userID := 101
	binding := MiniAppBinding{
		ID:            "expired-confirmation-binding",
		PendingFlowID: 99,
		AppID:         "wx-miniapp-a",
		OpenIDHash:    "4444444444444444444444444444444444444444444444444444444444444444",
		UserID:        &userID,
		Status:        MiniAppBindingStatusPending,
		ExpiresAt:     time.Now().Add(-time.Minute),
	}
	require.NoError(t, DB.Create(&binding).Error)

	err := DB.Transaction(func(tx *gorm.DB) error {
		_, err := ConfirmMiniAppBindingWithTx(tx, MiniAppBindingConfirmation{
			BindingID: binding.ID, PendingFlowID: binding.PendingFlowID, AppID: binding.AppID,
			OpenIDHash: binding.OpenIDHash, UserID: userID,
		})
		return err
	})
	assert.ErrorIs(t, err, ErrMiniAppBindingExpired)
}
