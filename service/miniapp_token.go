package service

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/setting/operation_setting"
	"gorm.io/gorm"
)

const (
	miniAppTokenNameMaxRunes = 50
	miniAppTokenModelsMax    = 20
)

var (
	ErrMiniAppTokenInvalid  = errors.New("mini app token request is invalid")
	ErrMiniAppTokenNotFound = errors.New("mini app token was not found")
	ErrMiniAppTokenExpired  = errors.New("mini app token has expired")
	ErrMiniAppTokenLimit    = errors.New("mini app token limit reached")
)

// MiniAppTokenCreateRequest intentionally omits every token control that the
// Mini Program is not allowed to choose, including quota, IP allowlists, and
// retry behavior.
type MiniAppTokenCreateRequest struct {
	Name          string
	Group         string
	Models        []string
	ExpiresInDays int
}

// MiniAppTokenSummary is a display-only projection. It never carries a raw
// token key, quota allocation, IP rules, source, or retry configuration.
type MiniAppTokenSummary struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	KeyHint     string   `json:"key_hint"`
	Status      int      `json:"status"`
	CreatedAt   int64    `json:"created_at"`
	AccessedAt  int64    `json:"accessed_at"`
	ExpiresAt   int64    `json:"expires_at"`
	Group       string   `json:"group"`
	ModelLimits []string `json:"model_limits"`
}

type MiniAppCreatedToken struct {
	Token    MiniAppTokenSummary `json:"token"`
	TokenKey string              `json:"token_key"`
}

func ListMiniAppTokens(userID, startIdx, pageSize int) ([]MiniAppTokenSummary, int, error) {
	tokens, total, err := model.GetMiniAppUserTokens(userID, startIdx, pageSize)
	if err != nil {
		return nil, 0, err
	}
	summaries := make([]MiniAppTokenSummary, 0, len(tokens))
	for _, token := range tokens {
		summaries = append(summaries, miniAppTokenSummary(token))
	}
	return summaries, int(total), nil
}

func CreateMiniAppToken(userID int, request MiniAppTokenCreateRequest) (*MiniAppCreatedToken, error) {
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return nil, err
	}

	request.Name = strings.TrimSpace(request.Name)
	request.Group = strings.TrimSpace(request.Group)
	if request.Name == "" || utf8.RuneCountInString(request.Name) > miniAppTokenNameMaxRunes {
		return nil, ErrMiniAppTokenInvalid
	}
	if !GroupInUserUsableGroups(user.Group, request.Group) {
		return nil, ErrMiniAppTokenInvalid
	}
	if !miniAppTokenExpiryIsAllowed(request.ExpiresInDays) {
		return nil, ErrMiniAppTokenInvalid
	}

	allowedModels, err := model.GetEnabledModelsForGroup(request.Group)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(allowedModels))
	for _, name := range allowedModels {
		allowed[name] = struct{}{}
	}
	if len(request.Models) == 0 || len(request.Models) > miniAppTokenModelsMax {
		return nil, ErrMiniAppTokenInvalid
	}
	modelNames := make([]string, 0, len(request.Models))
	seenModels := make(map[string]struct{}, len(request.Models))
	for _, modelName := range request.Models {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" || utf8.RuneCountInString(modelName) > 128 {
			return nil, ErrMiniAppTokenInvalid
		}
		if _, exists := seenModels[modelName]; exists {
			return nil, ErrMiniAppTokenInvalid
		}
		if _, exists := allowed[modelName]; !exists {
			return nil, ErrMiniAppTokenInvalid
		}
		seenModels[modelName] = struct{}{}
		modelNames = append(modelNames, modelName)
	}

	count, err := model.CountUserTokens(userID)
	if err != nil {
		return nil, err
	}
	if count >= int64(operation_setting.GetMaxUserTokens()) {
		return nil, ErrMiniAppTokenLimit
	}
	key, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	token := &model.Token{
		UserId:             userID,
		Key:                key,
		Name:               request.Name,
		Status:             common.TokenStatusEnabled,
		CreatedTime:        now,
		AccessedTime:       now,
		ExpiredTime:        now + int64(request.ExpiresInDays)*int64((24*time.Hour)/time.Second),
		UnlimitedQuota:     true,
		ModelLimitsEnabled: true,
		ModelLimits:        strings.Join(modelNames, ","),
		Group:              request.Group,
		CrossGroupRetry:    false,
		Source:             model.TokenSourceMiniApp,
	}
	if err := token.Insert(); err != nil {
		return nil, err
	}
	return &MiniAppCreatedToken{
		Token:    miniAppTokenSummary(token),
		TokenKey: token.GetFullKey(),
	}, nil
}

func UpdateMiniAppTokenStatus(userID, tokenID, status int) (*MiniAppTokenSummary, error) {
	if status != common.TokenStatusEnabled && status != common.TokenStatusDisabled {
		return nil, ErrMiniAppTokenInvalid
	}
	token, err := model.GetMiniAppUserTokenByID(userID, tokenID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMiniAppTokenNotFound
	}
	if err != nil {
		return nil, err
	}
	if status == common.TokenStatusEnabled && token.ExpiredTime <= common.GetTimestamp() {
		return nil, ErrMiniAppTokenExpired
	}
	token.Status = status
	if err := token.Update(); err != nil {
		return nil, err
	}
	summary := miniAppTokenSummary(token)
	return &summary, nil
}

// RevokeMiniAppToken is intentionally idempotent. A missing token, a token
// owned by someone else, and a non-Mini-Program token all result in the same
// successful no-op, which prevents deletion requests from probing ownership.
func RevokeMiniAppToken(userID, tokenID int) error {
	token, err := model.GetMiniAppUserTokenByID(userID, tokenID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return token.Delete()
}

func miniAppTokenExpiryIsAllowed(days int) bool {
	return days == 7 || days == 30 || days == 90
}

func miniAppTokenSummary(token *model.Token) MiniAppTokenSummary {
	return MiniAppTokenSummary{
		ID:          token.Id,
		Name:        token.Name,
		KeyHint:     token.GetMaskedKey(),
		Status:      token.Status,
		CreatedAt:   token.CreatedTime,
		AccessedAt:  token.AccessedTime,
		ExpiresAt:   token.ExpiredTime,
		Group:       token.Group,
		ModelLimits: token.GetModelLimits(),
	}
}
