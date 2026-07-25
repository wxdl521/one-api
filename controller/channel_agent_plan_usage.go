package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	agentPlanUsageEndpoint        = "https://open.volcengineapi.com"
	agentPlanUsageCacheTTL        = time.Minute
	agentPlanUsageConcurrentLimit = 4
	agentPlanUsageRequestLimit    = 100
)

type agentPlanUsageRequest struct {
	ChannelIDs []int `json:"channel_ids"`
}

type agentPlanChannelUsage struct {
	service.AgentPlanUsage
	UpdatedAt int64  `json:"updated_at"`
	Stale     bool   `json:"stale"`
	Status    string `json:"status"`
}

var agentPlanUsageCache = service.NewAgentPlanUsageCache(
	agentPlanUsageCacheTTL,
	time.Now,
	service.FetchAgentPlanUsage,
)

var fetchAgentPlanUsageForChannel = func(ctx context.Context, channel *model.Channel) (service.AgentPlanUsage, int64, bool, error) {
	client, err := service.GetHttpClientWithProxy(channel.GetSetting().Proxy)
	if err != nil {
		return service.AgentPlanUsage{}, 0, false, err
	}
	credentials := strings.TrimSpace(channel.AgentPlanAccessKey) + "|" + strings.TrimSpace(channel.AgentPlanSecretKey)
	return agentPlanUsageCache.Fetch(ctx, channel.Id, client, agentPlanUsageEndpoint, credentials)
}

func GetAgentPlanChannelUsage(c *gin.Context) {
	req := agentPlanUsageRequest{}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if len(req.ChannelIDs) > agentPlanUsageRequestLimit {
		common.ApiError(c, fmt.Errorf("at most %d channel IDs may be requested", agentPlanUsageRequestLimit))
		return
	}

	channels := make([]*model.Channel, 0, len(req.ChannelIDs))
	seen := make(map[int]struct{}, len(req.ChannelIDs))
	for _, channelID := range req.ChannelIDs {
		if channelID <= 0 {
			continue
		}
		if _, ok := seen[channelID]; ok {
			continue
		}
		seen[channelID] = struct{}{}

		channel, err := model.GetChannelById(channelID, true)
		if err != nil || channel == nil || channel.ChannelInfo.IsMultiKey {
			continue
		}
		if channel.Type != constant.ChannelTypeVolcEngine && channel.Type != constant.ChannelTypeAdvancedCustom {
			continue
		}
		if !channel.GetOtherSettings().AgentPlanUsageEnabled {
			continue
		}
		channels = append(channels, channel)
	}

	data := make(map[string]agentPlanChannelUsage, len(channels))
	var dataMu sync.Mutex
	var waitGroup sync.WaitGroup
	semaphore := make(chan struct{}, agentPlanUsageConcurrentLimit)
	for _, channel := range channels {
		waitGroup.Add(1)
		go func(channel *model.Channel) {
			defer waitGroup.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := agentPlanChannelUsage{Status: "unavailable"}
			if strings.TrimSpace(channel.AgentPlanAccessKey) == "" || strings.TrimSpace(channel.AgentPlanSecretKey) == "" {
				result.Status = "credentials_required"
			} else {
				ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
				usage, updatedAt, stale, err := fetchAgentPlanUsageForChannel(ctx, channel)
				cancel()
				if err == nil {
					result.AgentPlanUsage = usage
					result.UpdatedAt = updatedAt
					result.Stale = stale
					if stale {
						result.Status = "stale"
					} else {
						result.Status = "available"
					}
				}
			}

			dataMu.Lock()
			data[strconv.Itoa(channel.Id)] = result
			dataMu.Unlock()
		}(channel)
	}
	waitGroup.Wait()

	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
