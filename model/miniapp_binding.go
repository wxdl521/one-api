package model

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"gorm.io/gorm"
)

const (
	MiniAppBindingStatusPending = "pending"
	MiniAppBindingStatusBound   = "bound"
	MiniAppBindingStatusExpired = "expired"
	miniAppBindingIDBytes       = 32
)

var (
	ErrMiniAppBindingInvalid      = errors.New("mini app binding is invalid")
	ErrMiniAppBindingExpired      = errors.New("mini app binding has expired")
	ErrMiniAppBindingAlreadyBound = errors.New("mini app binding is already bound")
)

// MiniAppBinding records the pending and confirmed state of a Mini Program
// browser binding. Its opaque ID is only a locator: confirmation also requires
// the pending auth-flow ID and the HMAC-derived subject digest.
type MiniAppBinding struct {
	ID            string     `json:"id" gorm:"primaryKey;type:varchar(64)"`
	PendingFlowID int64      `json:"-" gorm:"not null;index:idx_miniapp_binding_pending_subject,priority:1"`
	AppID         string     `json:"app_id" gorm:"type:varchar(64);not null;index:idx_miniapp_binding_pending_subject,priority:2"`
	OpenIDHash    string     `json:"-" gorm:"type:char(64);not null;index:idx_miniapp_binding_pending_subject,priority:3"`
	UserID        *int       `json:"user_id,omitempty" gorm:"index"`
	Status        string     `json:"status" gorm:"type:varchar(16);not null;index:idx_miniapp_binding_status_expiry,priority:1"`
	CreatedAt     time.Time  `json:"created_at"`
	ExpiresAt     time.Time  `json:"expires_at" gorm:"not null;index:idx_miniapp_binding_status_expiry,priority:2"`
	BoundAt       *time.Time `json:"bound_at,omitempty"`
	ExpiredAt     *time.Time `json:"expired_at,omitempty" gorm:"index"`
}

func (MiniAppBinding) TableName() string {
	return "miniapp_bindings"
}

type MiniAppBindingCreate struct {
	PendingFlowID int64
	AppID         string
	OpenIDHash    string
	ExpiresAt     time.Time
}

type MiniAppBindingConfirmation struct {
	BindingID     string
	PendingFlowID int64
	AppID         string
	OpenIDHash    string
	UserID        int
}

// CreateMiniAppBinding creates one short-lived browser binding state record.
// The generated opaque ID is never sufficient to read or confirm the binding.
func CreateMiniAppBinding(input MiniAppBindingCreate) (*MiniAppBinding, error) {
	appID, openIDHash, valid := normalizeWechatMiniSubject(input.AppID, input.OpenIDHash)
	if input.PendingFlowID <= 0 || !valid || input.ExpiresAt.IsZero() || !input.ExpiresAt.After(time.Now()) {
		return nil, ErrMiniAppBindingInvalid
	}
	bytes := make([]byte, miniAppBindingIDBytes)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}
	binding := &MiniAppBinding{
		ID:            base64.RawURLEncoding.EncodeToString(bytes),
		PendingFlowID: input.PendingFlowID,
		AppID:         appID,
		OpenIDHash:    openIDHash,
		Status:        MiniAppBindingStatusPending,
		ExpiresAt:     input.ExpiresAt,
	}
	if err := DB.Create(binding).Error; err != nil {
		return nil, err
	}
	return binding, nil
}

// GetMiniAppBindingForPendingIdentity returns a binding only after the caller
// proves its opaque ID, pending auth-flow ID, and Mini Program subject digest.
// The opaque ID is a locator, not sufficient authority by itself.
func GetMiniAppBindingForPendingIdentity(bindingID string, pendingFlowID int64, appID, openIDHash string) (*MiniAppBinding, error) {
	appID, openIDHash, valid := normalizeWechatMiniSubject(appID, openIDHash)
	if bindingID == "" || pendingFlowID <= 0 || !valid {
		return nil, ErrMiniAppBindingInvalid
	}
	var binding MiniAppBinding
	err := DB.Where("id = ? AND pending_flow_id = ? AND app_id = ? AND open_id_hash = ?", bindingID, pendingFlowID, appID, openIDHash).
		First(&binding).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMiniAppBindingInvalid
		}
		return nil, err
	}
	if !binding.ExpiresAt.After(time.Now()) {
		return nil, ErrMiniAppBindingExpired
	}
	if binding.Status != MiniAppBindingStatusPending && binding.Status != MiniAppBindingStatusBound {
		return nil, ErrMiniAppBindingInvalid
	}
	return &binding, nil
}

// ConfirmMiniAppBindingWithTx atomically persists the durable Mini Program
// identity and changes this binding state from pending to bound. Callers must
// invoke it from the same transaction that consumes the matching auth flow.
func ConfirmMiniAppBindingWithTx(tx *gorm.DB, confirmation MiniAppBindingConfirmation) (*MiniAppBinding, error) {
	appID, openIDHash, valid := normalizeWechatMiniSubject(confirmation.AppID, confirmation.OpenIDHash)
	if tx == nil || confirmation.BindingID == "" || confirmation.PendingFlowID <= 0 || !valid || confirmation.UserID <= 0 {
		return nil, ErrMiniAppBindingInvalid
	}
	now := time.Now()
	result := tx.Model(&MiniAppBinding{}).
		Where("id = ? AND pending_flow_id = ? AND app_id = ? AND open_id_hash = ? AND status = ? AND expires_at > ?",
			confirmation.BindingID, confirmation.PendingFlowID, appID, openIDHash, MiniAppBindingStatusPending, now).
		Updates(map[string]any{
			"user_id":  confirmation.UserID,
			"status":   MiniAppBindingStatusBound,
			"bound_at": now,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		var stored MiniAppBinding
		err := tx.Where("id = ? AND pending_flow_id = ? AND app_id = ? AND open_id_hash = ?", confirmation.BindingID, confirmation.PendingFlowID, appID, openIDHash).
			First(&stored).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMiniAppBindingInvalid
		}
		if err != nil {
			return nil, err
		}
		if !stored.ExpiresAt.After(now) || stored.Status == MiniAppBindingStatusExpired {
			return nil, ErrMiniAppBindingExpired
		}
		return nil, ErrMiniAppBindingAlreadyBound
	}
	if _, err := ClaimWechatMiniIdentityWithTx(tx, appID, openIDHash, confirmation.UserID); err != nil {
		return nil, err
	}

	var binding MiniAppBinding
	if err := tx.Where("id = ? AND pending_flow_id = ? AND app_id = ? AND open_id_hash = ?", confirmation.BindingID, confirmation.PendingFlowID, appID, openIDHash).
		First(&binding).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

// ExpireAndDeleteMiniAppBindings retains expired binding state for the same
// period as auth flows. Fresh pending and bound records are untouched.
func ExpireAndDeleteMiniAppBindings(now time.Time) error {
	if err := DB.Model(&MiniAppBinding{}).
		Where("status IN ? AND expires_at < ?", []string{MiniAppBindingStatusPending, MiniAppBindingStatusBound}, now).
		Updates(map[string]any{
			"status":     MiniAppBindingStatusExpired,
			"expired_at": now,
		}).Error; err != nil {
		return err
	}
	if err := DB.Model(&MiniAppBinding{}).
		Where("status = ? AND expired_at IS NULL", MiniAppBindingStatusExpired).
		Update("expired_at", now).Error; err != nil {
		return err
	}
	cutoff := now.Add(-AuthFlowDefaultCleanupRetention)
	return DB.Where("status = ? AND expired_at < ?", MiniAppBindingStatusExpired, cutoff).
		Delete(&MiniAppBinding{}).Error
}
