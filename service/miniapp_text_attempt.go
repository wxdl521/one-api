package service

import (
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"gorm.io/gorm"
)

const (
	miniTextTestMaxInputRunes = 4000
	miniTextTestAttemptTTL    = 24 * time.Hour
)

var (
	ErrMiniTextTestInvalid          = errors.New("mini text test request is invalid")
	ErrMiniTextTestModelUnavailable = errors.New("mini text test model is unavailable")
	ErrMiniTextTestRequestConflict  = errors.New("mini text test request id conflicts with a prior request")
	ErrMiniTextTestNotFound         = errors.New("mini text test attempt was not found")
)

// MiniTextTestRequest contains the only client-selectable fields for the
// Mini Program text-test endpoint. Relay parameters are fixed by the BFF.
type MiniTextTestRequest struct {
	ClientRequestID string
	Model           string
	Input           string
}

// MiniTextTestStatus is intentionally a safe terminal-state projection. It
// never includes prompt text, generated output, channel information, or an
// upstream error.
type MiniTextTestStatus struct {
	RequestID       string `json:"request_id"`
	Model           string `json:"model"`
	State           string `json:"state"`
	ChargeReference string `json:"charge_ref,omitempty"`
	ChargedQuota    int    `json:"charged_quota"`
	ErrorCode       string `json:"error_code,omitempty"`
	CreatedAt       int64  `json:"created_at"`
	StartedAt       int64  `json:"started_at"`
	CompletedAt     int64  `json:"completed_at,omitempty"`
}

// MiniTextTestCompletion carries only the safe terminal data allowed to be
// retained after an in-memory relay response has been discarded.
type MiniTextTestCompletion struct {
	State           string
	ChargeReference string
	ChargedQuota    int
	ErrorCode       string
}

// StartMiniTextTest validates the restricted request and atomically claims a
// client request ID. A matching duplicate returns the existing status; a
// changed payload is rejected instead of being relayed a second time.
func StartMiniTextTest(userID int, request MiniTextTestRequest) (*MiniTextTestStatus, bool, error) {
	config, err := RequireMiniProgramTextTestConfig()
	if err != nil {
		return nil, false, err
	}
	request.ClientRequestID = strings.TrimSpace(request.ClientRequestID)
	request.Model = strings.TrimSpace(request.Model)
	if userID <= 0 || !miniTextTestRequestIDValid(request.ClientRequestID) ||
		request.Model == "" || utf8.RuneCountInString(request.Model) > 255 ||
		!utf8.ValidString(request.Input) || strings.TrimSpace(request.Input) == "" ||
		utf8.RuneCountInString(request.Input) > miniTextTestMaxInputRunes {
		return nil, false, ErrMiniTextTestInvalid
	}

	user, err := model.GetUserById(userID, false)
	if err != nil {
		return nil, false, err
	}
	if user.Status != common.UserStatusEnabled {
		return nil, false, ErrMiniTextTestModelUnavailable
	}
	allowedModels, err := model.GetEnabledModelsForGroup(user.Group)
	if err != nil {
		return nil, false, err
	}
	if _, configured := miniAppAllowedModelSet(common.MiniAppAllowedModels)[request.Model]; !configured {
		return nil, false, ErrMiniTextTestModelUnavailable
	}
	modelAvailable := false
	for _, modelName := range allowedModels {
		if modelName == request.Model {
			modelAvailable = true
			break
		}
	}
	if !modelAvailable {
		return nil, false, ErrMiniTextTestModelUnavailable
	}

	inputHMAC := common.GenerateHMACWithKey(
		[]byte(config.subjectHMACKey),
		"mini-text-test-v1:"+request.Model+":"+request.Input,
	)
	now := time.Now().UTC()
	attempt, created, err := model.CreateMiniTextTestAttempt(model.MiniTextTestAttemptCreate{
		UserID:          userID,
		ClientRequestID: request.ClientRequestID,
		Model:           request.Model,
		InputHMAC:       inputHMAC,
		CreatedAt:       now,
		ExpiresAt:       now.Add(miniTextTestAttemptTTL),
	})
	if err != nil {
		return nil, false, err
	}
	if !created && (attempt.Model != request.Model || attempt.InputHMAC != inputHMAC) {
		return nil, false, ErrMiniTextTestRequestConflict
	}
	status := miniTextTestStatus(attempt)
	return &status, created, nil
}

func ListMiniTextTestModels(userID int) ([]string, error) {
	if _, err := RequireMiniProgramTextTestConfig(); err != nil {
		return nil, err
	}
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return nil, err
	}
	if user.Status != common.UserStatusEnabled {
		return nil, ErrMiniTextTestModelUnavailable
	}
	available, err := model.GetEnabledModelsForGroup(user.Group)
	if err != nil {
		return nil, err
	}
	configured := miniAppAllowedModelSet(common.MiniAppAllowedModels)
	models := make([]string, 0, len(available))
	for _, modelName := range available {
		if _, ok := configured[modelName]; ok {
			models = append(models, modelName)
		}
	}
	sort.Strings(models)
	return models, nil
}

func GetMiniTextTestStatus(userID int, clientRequestID string) (*MiniTextTestStatus, error) {
	clientRequestID = strings.TrimSpace(clientRequestID)
	if userID <= 0 || !miniTextTestRequestIDValid(clientRequestID) {
		return nil, ErrMiniTextTestInvalid
	}
	attempt, err := model.GetMiniTextTestAttempt(userID, clientRequestID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMiniTextTestNotFound
		}
		return nil, err
	}
	status := miniTextTestStatus(attempt)
	return &status, nil
}

func CompleteMiniTextTest(userID int, clientRequestID string, completion MiniTextTestCompletion) (*MiniTextTestStatus, error) {
	completion.ChargeReference = strings.TrimSpace(completion.ChargeReference)
	completion.ErrorCode = strings.TrimSpace(completion.ErrorCode)
	if userID <= 0 || !miniTextTestRequestIDValid(strings.TrimSpace(clientRequestID)) ||
		completion.ChargeReference == "" || completion.ChargedQuota < 0 ||
		(completion.State != model.MiniTextTestAttemptStateSucceeded &&
			completion.State != model.MiniTextTestAttemptStateFailed &&
			completion.State != model.MiniTextTestAttemptStateTimedOut) {
		return nil, ErrMiniTextTestInvalid
	}
	if completion.State == model.MiniTextTestAttemptStateSucceeded && completion.ErrorCode != "" {
		return nil, ErrMiniTextTestInvalid
	}
	if completion.State != model.MiniTextTestAttemptStateSucceeded &&
		completion.ErrorCode != "MINIAPP_TEXT_TEST_REJECTED" &&
		completion.ErrorCode != "MINIAPP_TEXT_TEST_UNAVAILABLE" &&
		completion.ErrorCode != "MINIAPP_TEXT_TEST_TIMEOUT" {
		return nil, ErrMiniTextTestInvalid
	}
	attempt, err := model.CompleteMiniTextTestAttempt(
		userID,
		clientRequestID,
		completion.State,
		completion.ChargeReference,
		completion.ChargedQuota,
		completion.ErrorCode,
		time.Now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	status := miniTextTestStatus(attempt)
	return &status, nil
}

func miniTextTestRequestIDValid(value string) bool {
	if len(value) < 8 || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func miniTextTestStatus(attempt *model.MiniTextTestAttempt) MiniTextTestStatus {
	status := MiniTextTestStatus{
		RequestID:       attempt.ClientRequestID,
		Model:           attempt.Model,
		State:           attempt.State,
		ChargeReference: attempt.ChargeReference,
		ChargedQuota:    attempt.ChargedQuota,
		ErrorCode:       attempt.ErrorCode,
		CreatedAt:       attempt.CreatedAt.Unix(),
		StartedAt:       attempt.StartedAt.Unix(),
	}
	if attempt.CompletedAt != nil {
		status.CompletedAt = attempt.CompletedAt.Unix()
	}
	return status
}
