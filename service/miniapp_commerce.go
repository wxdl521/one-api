package service

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/setting/operation_setting"
	"gorm.io/gorm"
)

const (
	miniAppCheckoutWebPath   = "/miniapp-checkout"
	miniAppCheckoutTicketTTL = 5 * time.Minute
	miniAppCheckoutPlan      = "plan"
	miniAppCheckoutProduct   = "product"
)

var (
	ErrMiniAppCheckoutInvalid     = errors.New("mini app checkout request is invalid")
	ErrMiniAppCheckoutUnavailable = errors.New("mini app checkout target is unavailable")
)

type MiniAppPlan struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	Subtitle      string  `json:"subtitle"`
	PriceAmount   float64 `json:"price_amount"`
	Currency      string  `json:"currency"`
	DurationUnit  string  `json:"duration_unit"`
	DurationValue int     `json:"duration_value"`
}

type MiniAppProduct struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	PriceCents  int    `json:"price_cents"`
	ProductType string `json:"product_type"`
}

type MiniAppOrder struct {
	ID                int    `json:"id"`
	ProductName       string `json:"product_name"`
	PriceCents        int    `json:"price_cents"`
	PaymentStatus     string `json:"payment_status"`
	FulfillmentStatus string `json:"fulfillment_status"`
	CreatedAt         int64  `json:"created_at"`
}

type MiniAppCheckoutStart struct {
	CheckoutURL string `json:"checkout_url"`
}

type MiniAppCheckoutConfirmation struct {
	CheckoutPath string `json:"checkout_path"`
}

type miniAppCheckoutPayload struct {
	TargetType string `json:"target_type"`
	TargetID   int    `json:"target_id"`
}

// ListMiniAppPlans exposes only enabled, non-administrative plans. It does
// not return payment-provider identifiers or plan-management controls.
func ListMiniAppPlans() ([]MiniAppPlan, error) {
	if !operation_setting.IsPaymentComplianceConfirmed() {
		return []MiniAppPlan{}, nil
	}
	plans := make([]model.SubscriptionPlan, 0)
	if err := model.DB.Where("enabled = ?", true).Order("sort_order desc, id desc").Find(&plans).Error; err != nil {
		return nil, err
	}
	result := make([]MiniAppPlan, 0, len(plans))
	for _, plan := range plans {
		isPackage, err := model.IsAgentPlanPackageSubscriptionPlan(plan.Id)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if isPackage {
			continue
		}
		result = append(result, MiniAppPlan{
			ID:            plan.Id,
			Title:         plan.Title,
			Subtitle:      plan.Subtitle,
			PriceAmount:   plan.PriceAmount,
			Currency:      plan.Currency,
			DurationUnit:  plan.DurationUnit,
			DurationValue: plan.DurationValue,
		})
	}
	return result, nil
}

// ListMiniAppProducts exposes the published catalog without payment
// instructions or product-to-plan administration fields.
func ListMiniAppProducts() ([]MiniAppProduct, error) {
	products, err := model.ListPublishedProducts()
	if err != nil {
		return nil, err
	}
	result := make([]MiniAppProduct, 0, len(products))
	for _, product := range products {
		result = append(result, MiniAppProduct{
			ID:          product.Id,
			Name:        product.Name,
			Summary:     product.Summary,
			Description: product.Description,
			ImageURL:    product.ImageURL,
			PriceCents:  product.PriceCents,
			ProductType: product.ProductType,
		})
	}
	return result, nil
}

// ListMiniAppOrders projects existing product orders for exactly one user.
// It neither creates nor changes payment or fulfillment state.
func ListMiniAppOrders(userID int) ([]MiniAppOrder, error) {
	if userID <= 0 {
		return nil, errors.New("invalid mini app commerce user id")
	}
	orders, err := model.ListProductOrdersByUser(userID)
	if err != nil {
		return nil, err
	}
	result := make([]MiniAppOrder, 0, len(orders))
	for _, order := range orders {
		result = append(result, MiniAppOrder{
			ID:                order.Id,
			ProductName:       order.ProductName,
			PriceCents:        order.PriceCents,
			PaymentStatus:     order.PaymentStatus,
			FulfillmentStatus: order.FulfillmentStatus,
			CreatedAt:         order.CreatedAt,
		})
	}
	return result, nil
}

// StartMiniAppCheckout creates a short-lived, one-time browser handoff that
// authorizes only the existing Web checkout journey. It does not create an
// order or initiate a payment.
func StartMiniAppCheckout(userID int, sessionID string, targetType string, targetID int) (*MiniAppCheckoutStart, error) {
	if userID <= 0 || strings.TrimSpace(sessionID) == "" || targetID <= 0 {
		return nil, ErrMiniAppCheckoutInvalid
	}
	targetType = strings.TrimSpace(targetType)
	switch targetType {
	case miniAppCheckoutPlan:
		if !operation_setting.IsPaymentComplianceConfirmed() {
			return nil, ErrMiniAppCheckoutUnavailable
		}
		var plan model.SubscriptionPlan
		if err := model.DB.Where("id = ? AND enabled = ?", targetID, true).First(&plan).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrMiniAppCheckoutUnavailable
			}
			return nil, err
		}
		isPackage, err := model.IsAgentPlanPackageSubscriptionPlan(plan.Id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				break
			}
			return nil, err
		}
		if isPackage {
			return nil, ErrMiniAppCheckoutUnavailable
		}
	case miniAppCheckoutProduct:
		if _, err := model.GetPublishedProductById(targetID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrMiniAppCheckoutUnavailable
			}
			return nil, err
		}
	default:
		return nil, ErrMiniAppCheckoutInvalid
	}

	config, err := RequireMiniProgramConfig()
	if err != nil {
		return nil, err
	}
	checkoutURL, err := url.Parse(config.BindWebBaseURL)
	if err != nil || checkoutURL.Scheme != "https" || checkoutURL.Host == "" || checkoutURL.User != nil {
		return nil, ErrMiniAppConfiguration
	}

	payload, err := common.Marshal(miniAppCheckoutPayload{TargetType: targetType, TargetID: targetID})
	if err != nil {
		return nil, err
	}
	ticket, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeMiniAppCheckout,
		Provider:  miniAppAuthFlowProvider,
		Intent:    model.AuthFlowIntentCheckout,
		UserId:    userID,
		SessionId: sessionID,
		Payload:   string(payload),
		ExpiresAt: time.Now().Add(miniAppCheckoutTicketTTL),
	})
	if err != nil {
		return nil, err
	}

	checkoutURL.Path = miniAppCheckoutWebPath
	checkoutURL.RawPath = ""
	checkoutURL.RawQuery = ""
	checkoutURL.Fragment = "checkout_ticket=" + ticket
	return &MiniAppCheckoutStart{CheckoutURL: checkoutURL.String()}, nil
}

// ConfirmMiniAppBrowserCheckout consumes the handoff only when a normal
// browser session belongs to the same user and the originating Mini Program
// session remains live. The returned target is a fixed internal path, never a
// client-provided redirect URL.
func ConfirmMiniAppBrowserCheckout(checkoutTicket string, browserIdentity AuthIdentity) (*MiniAppCheckoutConfirmation, error) {
	browserSession, _, err := ValidateLoginSession(browserIdentity)
	if err != nil {
		return nil, err
	}
	if browserSession.LoginMethod == "wechat-miniapp" {
		return nil, ErrMiniAppBrowserSessionRequired
	}

	confirmation := &MiniAppCheckoutConfirmation{}
	_, err = model.ConsumeAuthFlowWithAction(checkoutTicket, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeMiniAppCheckout, Provider: miniAppAuthFlowProvider,
		Intent: model.AuthFlowIntentCheckout, UserId: browserIdentity.UserID,
	}, func(tx *gorm.DB, checkoutFlow *model.AuthFlow) error {
		var miniSession model.UserSession
		if err := tx.Where(
			"sid = ? AND user_id = ? AND login_method = ? AND status = ? AND revoked_at = ? AND expires_at > ?",
			checkoutFlow.SessionId,
			browserIdentity.UserID,
			"wechat-miniapp",
			model.UserSessionStatusActive,
			0,
			time.Now().Unix(),
		).First(&miniSession).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrAuthFlowInvalid
			}
			return err
		}
		var payload miniAppCheckoutPayload
		if err := common.Unmarshal([]byte(checkoutFlow.Payload), &payload); err != nil || payload.TargetID <= 0 {
			return model.ErrAuthFlowInvalid
		}
		switch payload.TargetType {
		case miniAppCheckoutPlan:
			confirmation.CheckoutPath = fmt.Sprintf(
				"/wallet?purchase_plan_id=%d",
				payload.TargetID,
			)
		case miniAppCheckoutProduct:
			confirmation.CheckoutPath = fmt.Sprintf("/products/%d", payload.TargetID)
		default:
			return model.ErrAuthFlowInvalid
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return confirmation, nil
}
