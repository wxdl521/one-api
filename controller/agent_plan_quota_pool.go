package controller

import (
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type agentPlanQuotaPoolRequest struct {
	Name              *string  `json:"name"`
	SourceChannelId   *int     `json:"source_channel_id"`
	DisplayMultiplier *float64 `json:"display_multiplier"`
}

type agentPlanQuotaPoolResponse struct {
	Id                                int                             `json:"id"`
	Name                              string                          `json:"name"`
	SourceChannelId                   int                             `json:"source_channel_id"`
	DisplayMultiplier                 float64                         `json:"display_multiplier"`
	DisplayMultiplierMicros           int64                           `json:"display_multiplier_micros"`
	OfficialMonthlyRemainingAFPMicros int64                           `json:"official_monthly_remaining_afp_micros"`
	FiveHourRemainingAFPMicros        int64                           `json:"five_hour_remaining_afp_micros"`
	FiveHourResetAt                   int64                           `json:"five_hour_reset_at"`
	WeeklyRemainingAFPMicros          int64                           `json:"weekly_remaining_afp_micros"`
	WeeklyResetAt                     int64                           `json:"weekly_reset_at"`
	MonthlyResetAt                    int64                           `json:"monthly_reset_at"`
	SyncedAt                          int64                           `json:"synced_at"`
	SyncStatus                        string                          `json:"sync_status"`
	Stock                             service.AgentPlanQuotaPoolStock `json:"stock"`
}

type agentPlanQuotaPoolEligibleSourceResponse struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
	Type int    `json:"type"`
}

func agentPlanQuotaPoolResponseFor(pool *model.AgentPlanQuotaPool, stock service.AgentPlanQuotaPoolStock) agentPlanQuotaPoolResponse {
	return agentPlanQuotaPoolResponse{
		Id:                                pool.Id,
		Name:                              pool.Name,
		SourceChannelId:                   pool.SourceChannelId,
		DisplayMultiplier:                 float64(pool.DisplayMultiplierMicros) / float64(service.AgentPlanMultiplierMicros),
		DisplayMultiplierMicros:           pool.DisplayMultiplierMicros,
		OfficialMonthlyRemainingAFPMicros: pool.OfficialMonthlyRemainingAFPMicros,
		FiveHourRemainingAFPMicros:        pool.FiveHourRemainingAFPMicros,
		FiveHourResetAt:                   pool.FiveHourResetAt,
		WeeklyRemainingAFPMicros:          pool.WeeklyRemainingAFPMicros,
		WeeklyResetAt:                     pool.WeeklyResetAt,
		MonthlyResetAt:                    pool.MonthlyResetAt,
		SyncedAt:                          pool.SyncedAt,
		SyncStatus:                        pool.SyncStatus,
		Stock:                             stock,
	}
}

func agentPlanDisplayMultiplierMicros(value float64) (int64, error) {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, errors.New("display multiplier must be finite and positive")
	}

	decimal, ok := new(big.Rat).SetString(strconv.FormatFloat(value, 'f', -1, 64))
	if !ok {
		return 0, errors.New("invalid display multiplier")
	}
	decimal.Mul(decimal, big.NewRat(service.AgentPlanMultiplierMicros, 1))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(decimal.Num(), decimal.Denom(), remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(decimal.Denom()) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if quotient.Sign() <= 0 || !quotient.IsInt64() {
		return 0, errors.New("display multiplier exceeds micro-unit range")
	}
	return quotient.Int64(), nil
}

func getAgentPlanQuotaPoolSource(channelID int) (*model.Channel, error) {
	if channelID <= 0 {
		return nil, model.ErrAgentPlanQuotaPoolSourceInvalid
	}
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return nil, err
	}
	if err := model.ValidateAgentPlanQuotaPoolSource(channel); err != nil {
		return nil, err
	}
	return channel, nil
}

func readAgentPlanQuotaPoolRequest(c *gin.Context) (agentPlanQuotaPoolRequest, bool) {
	var request agentPlanQuotaPoolRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "invalid agent plan quota pool request")
		return agentPlanQuotaPoolRequest{}, false
	}
	return request, true
}

func AdminListAgentPlanQuotaPools(c *gin.Context) {
	pools := make([]model.AgentPlanQuotaPool, 0)
	if err := model.DB.Order("id desc").Find(&pools).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	result := make([]agentPlanQuotaPoolResponse, 0, len(pools))
	now := time.Now()
	for i := range pools {
		stock, err := service.GetAgentPlanQuotaPoolStock(pools[i].Id, now)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		result = append(result, agentPlanQuotaPoolResponseFor(&pools[i], stock))
	}
	common.ApiSuccess(c, result)
}

func AdminCreateAgentPlanQuotaPool(c *gin.Context) {
	request, ok := readAgentPlanQuotaPoolRequest(c)
	if !ok {
		return
	}
	if request.Name == nil || request.SourceChannelId == nil || request.DisplayMultiplier == nil {
		common.ApiErrorMsg(c, "name, source_channel_id, and display_multiplier are required")
		return
	}
	name := strings.TrimSpace(*request.Name)
	if name == "" {
		common.ApiErrorMsg(c, "agent plan quota pool name is required")
		return
	}
	if _, err := getAgentPlanQuotaPoolSource(*request.SourceChannelId); err != nil {
		common.ApiErrorMsg(c, "invalid agent plan quota pool source channel")
		return
	}
	multiplierMicros, err := agentPlanDisplayMultiplierMicros(*request.DisplayMultiplier)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pool := &model.AgentPlanQuotaPool{
		SourceChannelId:         *request.SourceChannelId,
		Name:                    name,
		DisplayMultiplierMicros: multiplierMicros,
		SyncStatus:              model.AgentPlanQuotaPoolSyncStatusError,
		SyncError:               "snapshot unavailable",
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.AgentPlanQuotaPool{}).Where("source_channel_id = ?", pool.SourceChannelId).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("agent plan quota pool source channel is already bound")
		}
		return tx.Create(pool).Error
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, agentPlanQuotaPoolResponseFor(pool, service.AgentPlanQuotaPoolStock{}))
}

func AdminUpdateAgentPlanQuotaPool(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid agent plan quota pool id")
		return
	}
	request, ok := readAgentPlanQuotaPoolRequest(c)
	if !ok {
		return
	}
	updates := map[string]interface{}{}
	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if name == "" {
			common.ApiErrorMsg(c, "agent plan quota pool name is required")
			return
		}
		updates["name"] = name
	}
	if request.DisplayMultiplier != nil {
		multiplierMicros, err := agentPlanDisplayMultiplierMicros(*request.DisplayMultiplier)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		updates["display_multiplier_micros"] = multiplierMicros
	}
	if request.SourceChannelId != nil {
		if _, err := getAgentPlanQuotaPoolSource(*request.SourceChannelId); err != nil {
			common.ApiErrorMsg(c, "invalid agent plan quota pool source channel")
			return
		}
		updates["source_channel_id"] = *request.SourceChannelId
		updates["official_monthly_remaining_afp_micros"] = int64(0)
		updates["five_hour_remaining_afp_micros"] = int64(0)
		updates["five_hour_reset_at"] = int64(0)
		updates["weekly_remaining_afp_micros"] = int64(0)
		updates["weekly_reset_at"] = int64(0)
		updates["monthly_reset_at"] = int64(0)
		updates["synced_at"] = int64(0)
		updates["sync_status"] = model.AgentPlanQuotaPoolSyncStatusError
		updates["sync_error"] = "snapshot unavailable"
	}
	if len(updates) == 0 {
		common.ApiErrorMsg(c, "at least one agent plan quota pool field is required")
		return
	}
	updated := false
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		pool, err := model.GetAgentPlanQuotaPoolForUpdate(tx, id)
		if err != nil {
			return err
		}
		if request.SourceChannelId != nil && *request.SourceChannelId != pool.SourceChannelId {
			var duplicateCount int64
			if err := tx.Model(&model.AgentPlanQuotaPool{}).
				Where("source_channel_id = ? AND id <> ?", *request.SourceChannelId, id).
				Count(&duplicateCount).Error; err != nil {
				return err
			}
			if duplicateCount > 0 {
				return errors.New("agent plan quota pool source channel is already bound")
			}
			hasReferences, err := model.AgentPlanQuotaPoolHasReferencesTx(tx, id)
			if err != nil {
				return err
			}
			if hasReferences {
				return errors.New("agent plan quota pool source cannot change after package references exist")
			}
		}
		result := tx.Model(&model.AgentPlanQuotaPool{}).Where("id = ?", id).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		updated = result.RowsAffected > 0
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "agent plan quota pool not found")
			return
		}
		common.ApiError(c, err)
		return
	}
	if !updated {
		common.ApiErrorMsg(c, "agent plan quota pool not found")
		return
	}
	pool, err := model.GetAgentPlanQuotaPoolById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	stock, err := service.GetAgentPlanQuotaPoolStock(pool.Id, time.Now())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, agentPlanQuotaPoolResponseFor(pool, stock))
}

func AdminDeleteAgentPlanQuotaPool(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid agent plan quota pool id")
		return
	}
	deleted, err := model.DeleteAgentPlanQuotaPoolIfUnreferenced(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "agent plan quota pool not found")
			return
		}
		common.ApiError(c, err)
		return
	}
	if !deleted {
		common.ApiErrorMsg(c, "agent plan quota pool has package references")
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminSyncAgentPlanQuotaPool(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid agent plan quota pool id")
		return
	}
	pool, err := service.SyncAgentPlanQuotaPool(c.Request.Context(), id)
	if err != nil {
		common.ApiErrorMsg(c, "agent plan quota pool sync failed")
		return
	}
	stock, err := service.GetAgentPlanQuotaPoolStock(pool.Id, time.Now())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, agentPlanQuotaPoolResponseFor(pool, stock))
}

func AdminListAgentPlanQuotaPoolEligibleSourceChannels(c *gin.Context) {
	channels := make([]model.Channel, 0)
	if err := model.DB.Order("id asc").Find(&channels).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	result := make([]agentPlanQuotaPoolEligibleSourceResponse, 0)
	for i := range channels {
		if model.ValidateAgentPlanQuotaPoolSource(&channels[i]) != nil {
			continue
		}
		result = append(result, agentPlanQuotaPoolEligibleSourceResponse{Id: channels[i].Id, Name: channels[i].Name, Type: channels[i].Type})
	}
	common.ApiSuccess(c, result)
}
