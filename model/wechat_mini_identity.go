package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrWechatMiniIdentityInvalid      = errors.New("wechat mini identity is invalid")
	ErrWechatMiniIdentityAlreadyBound = errors.New("wechat mini identity is already bound")
)

// WechatMiniIdentity is the durable ownership record for a WeChat Mini
// Program identity. OpenIDHash is an HMAC-derived digest; plaintext OpenIDs
// must never be persisted here.
type WechatMiniIdentity struct {
	Id         int64     `json:"id" gorm:"primaryKey"`
	AppID      string    `json:"app_id" gorm:"type:varchar(64);not null;uniqueIndex:idx_wechat_mini_identity_subject,priority:1"`
	OpenIDHash string    `json:"-" gorm:"type:char(64);not null;uniqueIndex:idx_wechat_mini_identity_subject,priority:2"`
	UserID     int       `json:"user_id" gorm:"not null;index"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (WechatMiniIdentity) TableName() string {
	return "wechat_mini_identities"
}

// ClaimWechatMiniIdentityWithTx assigns a WeChat Mini Program subject to one
// user. Repeating the same owner is idempotent; a different owner is rejected
// by the subject's database uniqueness boundary.
func ClaimWechatMiniIdentityWithTx(tx *gorm.DB, appID, openIDHash string, userID int) (*WechatMiniIdentity, error) {
	appID, openIDHash, valid := normalizeWechatMiniSubject(appID, openIDHash)
	if tx == nil || !valid || userID <= 0 {
		return nil, ErrWechatMiniIdentityInvalid
	}
	identity := WechatMiniIdentity{AppID: appID, OpenIDHash: openIDHash, UserID: userID}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&identity)
	if result.Error != nil {
		return nil, result.Error
	}

	var stored WechatMiniIdentity
	if err := tx.Where("app_id = ? AND open_id_hash = ?", appID, openIDHash).First(&stored).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWechatMiniIdentityAlreadyBound
		}
		return nil, err
	}
	if stored.UserID != userID {
		return nil, ErrWechatMiniIdentityAlreadyBound
	}
	return &stored, nil
}

func normalizeWechatMiniSubject(appID, openIDHash string) (string, string, bool) {
	appID = strings.TrimSpace(appID)
	openIDHash = strings.TrimSpace(openIDHash)
	if appID == "" || len(appID) > 64 || len(openIDHash) != 64 {
		return "", "", false
	}
	for _, character := range openIDHash {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') && (character < 'A' || character > 'F') {
			return "", "", false
		}
	}
	return appID, strings.ToLower(openIDHash), true
}
