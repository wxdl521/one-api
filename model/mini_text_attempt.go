package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/the-one/common"
	"gorm.io/gorm/clause"
)

const (
	MiniTextTestAttemptStateRunning   = "running"
	MiniTextTestAttemptStateSucceeded = "succeeded"
	MiniTextTestAttemptStateFailed    = "failed"
	MiniTextTestAttemptStateTimedOut  = "timed_out"
)

var ErrMiniTextTestAttemptInvalid = errors.New("mini text test attempt is invalid")

// MiniTextTestAttempt is the short-lived idempotency and outcome record for a
// Mini Program text test. Prompt text and generated output are intentionally
// absent: only an HMAC digest of the input is retained.
type MiniTextTestAttempt struct {
	ID              int        `json:"id"`
	UserID          int        `json:"-" gorm:"not null;uniqueIndex:idx_mini_text_test_user_request,priority:1;index:idx_mini_text_test_user_created,priority:1"`
	ClientRequestID string     `json:"request_id" gorm:"type:varchar(128);not null;uniqueIndex:idx_mini_text_test_user_request,priority:2"`
	Model           string     `json:"model" gorm:"type:varchar(255);not null"`
	InputHMAC       string     `json:"-" gorm:"type:char(64);not null"`
	ClaimNonce      string     `json:"-" gorm:"type:char(48);not null;default:''"`
	State           string     `json:"state" gorm:"type:varchar(16);not null;index:idx_mini_text_test_state_expiry,priority:1"`
	ChargeReference string     `json:"charge_ref,omitempty" gorm:"type:varchar(64);not null;default:''"`
	ChargedQuota    int        `json:"charged_quota"`
	ErrorCode       string     `json:"error_code,omitempty" gorm:"type:varchar(64);not null;default:''"`
	CreatedAt       time.Time  `json:"created_at" gorm:"not null;index:idx_mini_text_test_user_created,priority:2"`
	StartedAt       time.Time  `json:"started_at" gorm:"not null"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	ExpiresAt       time.Time  `json:"expires_at" gorm:"not null;index:idx_mini_text_test_state_expiry,priority:2;index:idx_mini_text_test_expires_at"`
	UpdatedAt       time.Time  `json:"-"`
}

func (MiniTextTestAttempt) TableName() string {
	return "mini_text_test_attempts"
}

type MiniTextTestAttemptCreate struct {
	UserID          int
	ClientRequestID string
	Model           string
	InputHMAC       string
	CreatedAt       time.Time
	ExpiresAt       time.Time
}

// CreateMiniTextTestAttempt atomically claims a client request ID for a user.
// A duplicate returns the original immutable attempt and never creates a
// second relay or billing opportunity.
func CreateMiniTextTestAttempt(input MiniTextTestAttemptCreate) (*MiniTextTestAttempt, bool, error) {
	input.ClientRequestID = strings.TrimSpace(input.ClientRequestID)
	input.Model = strings.TrimSpace(input.Model)
	input.InputHMAC = strings.TrimSpace(input.InputHMAC)
	if input.UserID <= 0 || input.ClientRequestID == "" || len(input.ClientRequestID) > 128 ||
		input.Model == "" || len(input.Model) > 255 || len(input.InputHMAC) != 64 ||
		input.CreatedAt.IsZero() || !input.ExpiresAt.After(input.CreatedAt) {
		return nil, false, ErrMiniTextTestAttemptInvalid
	}

	claimNonce, err := common.GenerateRandomCharsKey(48)
	if err != nil {
		return nil, false, err
	}
	attempt := &MiniTextTestAttempt{
		UserID:          input.UserID,
		ClientRequestID: input.ClientRequestID,
		Model:           input.Model,
		InputHMAC:       input.InputHMAC,
		ClaimNonce:      claimNonce,
		State:           MiniTextTestAttemptStateRunning,
		CreatedAt:       input.CreatedAt,
		StartedAt:       input.CreatedAt,
		ExpiresAt:       input.ExpiresAt,
	}
	if err := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "client_request_id"}},
		DoNothing: true,
	}).Create(attempt).Error; err != nil {
		return nil, false, err
	}

	var existing MiniTextTestAttempt
	if err := DB.Where("user_id = ? AND client_request_id = ?", input.UserID, input.ClientRequestID).First(&existing).Error; err != nil {
		return nil, false, err
	}
	return &existing, existing.ClaimNonce == claimNonce, nil
}

func GetMiniTextTestAttempt(userID int, clientRequestID string) (*MiniTextTestAttempt, error) {
	clientRequestID = strings.TrimSpace(clientRequestID)
	if userID <= 0 || clientRequestID == "" || len(clientRequestID) > 128 {
		return nil, ErrMiniTextTestAttemptInvalid
	}
	var attempt MiniTextTestAttempt
	if err := DB.Where("user_id = ? AND client_request_id = ?", userID, clientRequestID).First(&attempt).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

// CompleteMiniTextTestAttempt records only the stable BFF outcome and billing
// correlation. It refuses a second completion so duplicate delivery cannot
// overwrite the first terminal result.
func CompleteMiniTextTestAttempt(userID int, clientRequestID, state, chargeReference string, chargedQuota int, errorCode string, completedAt time.Time) (*MiniTextTestAttempt, error) {
	clientRequestID = strings.TrimSpace(clientRequestID)
	chargeReference = strings.TrimSpace(chargeReference)
	errorCode = strings.TrimSpace(errorCode)
	if userID <= 0 || clientRequestID == "" || len(clientRequestID) > 128 ||
		(state != MiniTextTestAttemptStateSucceeded && state != MiniTextTestAttemptStateFailed && state != MiniTextTestAttemptStateTimedOut) ||
		chargedQuota < 0 || chargeReference == "" || len(chargeReference) > 64 || len(errorCode) > 64 || completedAt.IsZero() {
		return nil, ErrMiniTextTestAttemptInvalid
	}
	if state == MiniTextTestAttemptStateSucceeded && errorCode != "" {
		return nil, ErrMiniTextTestAttemptInvalid
	}
	if state != MiniTextTestAttemptStateSucceeded &&
		errorCode != "MINIAPP_TEXT_TEST_REJECTED" &&
		errorCode != "MINIAPP_TEXT_TEST_UNAVAILABLE" &&
		errorCode != "MINIAPP_TEXT_TEST_TIMEOUT" {
		return nil, ErrMiniTextTestAttemptInvalid
	}

	updates := map[string]any{
		"state":            state,
		"charge_reference": chargeReference,
		"charged_quota":    chargedQuota,
		"error_code":       errorCode,
		"completed_at":     completedAt,
	}
	result := DB.Model(&MiniTextTestAttempt{}).
		Where("user_id = ? AND client_request_id = ? AND state = ?", userID, clientRequestID, MiniTextTestAttemptStateRunning).
		Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	var attempt MiniTextTestAttempt
	if err := DB.Where("user_id = ? AND client_request_id = ?", userID, clientRequestID).First(&attempt).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

// DeleteExpiredMiniTextTestAttempts removes short-lived idempotency records
// after their terminal status is no longer needed by the client.
func DeleteExpiredMiniTextTestAttempts(now time.Time) error {
	if DB == nil || !DB.Migrator().HasTable(&MiniTextTestAttempt{}) {
		return nil
	}
	return DB.Where("expires_at < ?", now).Delete(&MiniTextTestAttempt{}).Error
}
