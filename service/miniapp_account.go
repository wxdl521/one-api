package service

import (
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/the-one/model"
)

const miniAppQuotaUnit = "quota"

// MiniAppAccountOverview is the deliberately small account projection used by
// the Mini Program. It must not grow into a dashboard user DTO because the
// Mini Program is not an administration surface.
type MiniAppAccountOverview struct {
	Username      string                       `json:"username"`
	DisplayName   string                       `json:"display_name"`
	Email         string                       `json:"email,omitempty"`
	Quota         MiniAppAccountQuota          `json:"quota"`
	EnabledGroups []string                     `json:"enabled_groups"`
	Subscriptions []MiniAppSubscriptionSummary `json:"subscriptions"`
}

type MiniAppAccountQuota struct {
	Balance int    `json:"balance"`
	Unit    string `json:"unit"`
}

type MiniAppSubscriptionSummary struct {
	PlanTitle string                   `json:"plan_title"`
	Status    string                   `json:"status"`
	EndsAt    int64                    `json:"ends_at"`
	Quota     MiniAppSubscriptionQuota `json:"quota"`
}

type MiniAppSubscriptionQuota struct {
	Remaining int64  `json:"remaining"`
	Unlimited bool   `json:"unlimited"`
	Unit      string `json:"unit"`
}

// GetMiniAppAccountOverview reads the existing account and subscription
// records without introducing a Mini Program balance or subscription store.
func GetMiniAppAccountOverview(userID int) (*MiniAppAccountOverview, error) {
	if userID <= 0 {
		return nil, errors.New("invalid mini app account user id")
	}

	user, err := model.GetUserById(userID, false)
	if err != nil {
		return nil, err
	}
	quota, err := model.GetUserQuota(userID, false)
	if err != nil {
		return nil, err
	}

	groupsByName := GetUserUsableGroups(user.Group)
	groups := make([]string, 0, len(groupsByName))
	for group := range groupsByName {
		groups = append(groups, group)
	}
	sort.Strings(groups)

	activeSubscriptions, err := model.GetAllActiveUserSubscriptions(userID)
	if err != nil {
		return nil, err
	}
	subscriptions := make([]MiniAppSubscriptionSummary, 0, len(activeSubscriptions))
	for _, summary := range activeSubscriptions {
		if summary.Subscription == nil {
			continue
		}
		plan, err := model.GetSubscriptionPlanById(summary.Subscription.PlanId)
		if err != nil {
			return nil, err
		}

		remaining := summary.Subscription.AmountTotal - summary.Subscription.AmountUsed
		if remaining < 0 {
			remaining = 0
		}
		subscriptions = append(subscriptions, MiniAppSubscriptionSummary{
			PlanTitle: plan.Title,
			Status:    summary.Subscription.Status,
			EndsAt:    summary.Subscription.EndTime,
			Quota: MiniAppSubscriptionQuota{
				Remaining: remaining,
				Unlimited: summary.Subscription.AmountTotal == 0,
				Unit:      miniAppQuotaUnit,
			},
		})
	}

	return &MiniAppAccountOverview{
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Email:       maskMiniAppEmail(user.Email),
		Quota: MiniAppAccountQuota{
			Balance: quota,
			Unit:    miniAppQuotaUnit,
		},
		EnabledGroups: groups,
		Subscriptions: subscriptions,
	}, nil
}

func maskMiniAppEmail(email string) string {
	email = strings.TrimSpace(email)
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		if email == "" {
			return ""
		}
		return "***"
	}
	localPart := []rune(email[:at])
	if len(localPart) == 1 {
		return "***" + email[at:]
	}
	return string(localPart[:1]) + "***" + email[at:]
}
