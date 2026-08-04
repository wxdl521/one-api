package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMiniAppCommerceControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.UserSession{},
		&model.SubscriptionPlan{},
		&model.AgentPlanPackagePlan{},
		&model.Product{},
		&model.ProductOrder{},
		&model.AuthFlow{},
	))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
	})
	return db
}

func createMiniAppCommerceContext(t *testing.T, path string, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, path, nil)
	context.Set("id", userID)
	return context, recorder
}

func seedMiniAppCommerceUser(t *testing.T, db *gorm.DB, username string) *model.User {
	t.Helper()
	user := &model.User{
		Username:    username,
		Password:    "password-placeholder",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		Quota:       1000,
		AuthVersion: 1,
		AffCode:     username + "-aff",
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func TestMiniAppListPlansReturnsOnlySafePublishedFields(t *testing.T) {
	db := setupMiniAppCommerceControllerTest(t)
	owner := seedMiniAppCommerceUser(t, db, "mini-plan-owner")
	require.NoError(t, db.Create(&model.SubscriptionPlan{
		Title: "Starter", Subtitle: "For small teams", PriceAmount: 12.5, Currency: "USD",
		DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true,
		StripePriceId: "price_secret", CreemProductId: "creem_secret", UpgradeGroup: "vip",
	}).Error)
	hiddenPlan := &model.SubscriptionPlan{
		Title: "Hidden", PriceAmount: 99, Currency: "USD", DurationUnit: model.SubscriptionDurationMonth,
		DurationValue: 1, Enabled: false,
	}
	require.NoError(t, db.Create(hiddenPlan).Error)
	require.NoError(t, db.Model(hiddenPlan).Update("enabled", false).Error)

	context, recorder := createMiniAppCommerceContext(t, "/api/miniapp/v1/plans", owner.Id)
	MiniAppListPlans(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Starter")
	assert.NotContains(t, recorder.Body.String(), "Hidden")
	assert.NotContains(t, recorder.Body.String(), "price_secret")
	assert.NotContains(t, recorder.Body.String(), "creem_secret")
	assert.NotContains(t, recorder.Body.String(), `"upgrade_group"`)
	assert.NotContains(t, recorder.Body.String(), `"enabled"`)
}

func TestMiniAppListProductsReturnsOnlySafePublishedFields(t *testing.T) {
	db := setupMiniAppCommerceControllerTest(t)
	owner := seedMiniAppCommerceUser(t, db, "mini-product-owner")
	require.NoError(t, db.Create(&model.Product{
		Name: "Consulting", Summary: "Expert help", Description: "One hour", ImageURL: "https://images.example/consulting.png",
		PriceCents: 2500, ProductType: model.ProductTypeManual, Enabled: true,
		PaymentQRCodeURL: "https://payments.example/private-qr", PaymentInstructions: "private instructions",
	}).Error)
	hiddenProduct := &model.Product{
		Name: "Hidden product", PriceCents: 5000, ProductType: model.ProductTypeManual, Enabled: false,
	}
	require.NoError(t, db.Create(hiddenProduct).Error)
	require.NoError(t, db.Model(hiddenProduct).Update("enabled", false).Error)

	context, recorder := createMiniAppCommerceContext(t, "/api/miniapp/v1/products", owner.Id)
	MiniAppListProducts(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Consulting")
	assert.NotContains(t, recorder.Body.String(), "Hidden product")
	assert.NotContains(t, recorder.Body.String(), "private-qr")
	assert.NotContains(t, recorder.Body.String(), "private instructions")
	assert.NotContains(t, recorder.Body.String(), `"payment_qr_code_url"`)
	assert.NotContains(t, recorder.Body.String(), `"subscription_plan_id"`)
}

func TestMiniAppListOrdersScopesSafeProjectionToCurrentUser(t *testing.T) {
	db := setupMiniAppCommerceControllerTest(t)
	owner := seedMiniAppCommerceUser(t, db, "mini-order-owner")
	other := seedMiniAppCommerceUser(t, db, "mini-order-other")
	require.NoError(t, db.Create(&model.ProductOrder{
		UserId: owner.Id, ProductId: 1, ProductName: "Owner product", PriceCents: 2500,
		PaymentMethod: model.ProductOrderPaymentQRCode, PaymentStatus: model.ProductOrderPaymentPending,
		FulfillmentStatus: model.ProductOrderFulfillmentPending, PaidQuota: 77,
	}).Error)
	require.NoError(t, db.Create(&model.ProductOrder{
		UserId: other.Id, ProductId: 2, ProductName: "Other product", PriceCents: 9900,
		PaymentMethod: model.ProductOrderPaymentWallet, PaymentStatus: model.ProductOrderPaymentPaid,
		FulfillmentStatus: model.ProductOrderFulfillmentShipped, PaidQuota: 123,
	}).Error)

	context, recorder := createMiniAppCommerceContext(t, "/api/miniapp/v1/orders", owner.Id)
	MiniAppListOrders(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Owner product")
	assert.NotContains(t, recorder.Body.String(), "Other product")
	assert.NotContains(t, recorder.Body.String(), `"user_id"`)
	assert.NotContains(t, recorder.Body.String(), `"paid_quota"`)
	assert.NotContains(t, recorder.Body.String(), `"payment_method"`)
}

func TestMiniAppStartCheckoutReturnsOnlyAnOpaqueFragmentHandoff(t *testing.T) {
	db := setupMiniAppCommerceControllerTest(t)
	owner := seedMiniAppCommerceUser(t, db, "mini-checkout-owner")
	product := &model.Product{
		Name: "Checkout product", PriceCents: 2500, ProductType: model.ProductTypeManual, Enabled: true,
	}
	require.NoError(t, db.Create(product).Error)

	previousAppID := common.WeChatMiniAppAppID
	previousAppSecret := common.WeChatMiniAppAppSecret
	previousSubjectKey := common.WeChatMiniAppSubjectHMACKey
	previousBindURL := common.MiniAppBindWebBaseURL
	previousTimeout := common.MiniAppHTTPTimeout
	previousEnabled, previousTextEnabled := common.GetMiniProgramFeatureFlags()
	common.WeChatMiniAppAppID = "wx-mini-checkout"
	common.WeChatMiniAppAppSecret = "server-only-secret"
	common.WeChatMiniAppSubjectHMACKey = "checkout-subject-key"
	common.MiniAppBindWebBaseURL = "https://console.example.com/miniapp-bind"
	common.MiniAppHTTPTimeout = 10 * time.Second
	common.OptionMapRWMutex.Lock()
	common.MiniProgramEnabled = true
	common.MiniProgramTextTestEnabled = false
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.WeChatMiniAppAppID = previousAppID
		common.WeChatMiniAppAppSecret = previousAppSecret
		common.WeChatMiniAppSubjectHMACKey = previousSubjectKey
		common.MiniAppBindWebBaseURL = previousBindURL
		common.MiniAppHTTPTimeout = previousTimeout
		common.OptionMapRWMutex.Lock()
		common.MiniProgramEnabled = previousEnabled
		common.MiniProgramTextTestEnabled = previousTextEnabled
		common.OptionMapRWMutex.Unlock()
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/miniapp/v1/checkout",
		bytes.NewBufferString(`{"target_type":"product","target_id":`+strconv.Itoa(product.Id)+`}`),
	)
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("id", owner.Id)
	context.Set("session_id", "mini-checkout-session")

	MiniAppStartCheckout(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			CheckoutURL string `json:"checkout_url"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	parsed, err := url.Parse(response.Data.CheckoutURL)
	require.NoError(t, err)
	assert.Equal(t, "https", parsed.Scheme)
	assert.Equal(t, "console.example.com", parsed.Host)
	assert.Equal(t, "/miniapp-checkout", parsed.Path)
	assert.Empty(t, parsed.RawQuery)
	assert.True(t, strings.HasPrefix(parsed.Fragment, "checkout_ticket="))
	assert.NotContains(t, response.Data.CheckoutURL, "mini-checkout-session")
	assert.NotContains(t, response.Data.CheckoutURL, "user_id")

	var flow model.AuthFlow
	require.NoError(t, db.First(&flow).Error)
	assert.Equal(t, owner.Id, flow.UserId)
	assert.Equal(t, "mini-checkout-session", flow.SessionId)
	assert.NotContains(t, flow.Payload, "server-only-secret")
}

func TestMiniAppBrowserCheckoutConfirmsSameUserHandoff(t *testing.T) {
	db := setupMiniAppCommerceControllerTest(t)
	owner := seedMiniAppCommerceUser(t, db, "mini-browser-checkout-owner")
	miniSession, err := service.CreateMiniAppLoginSession(owner.Id, "127.0.0.1", "mini-checkout-test")
	require.NoError(t, err)
	browserSession, err := service.CreateLoginSession(owner.Id, "password", "127.0.0.1", "browser-checkout-test")
	require.NoError(t, err)
	payload, err := common.Marshal(map[string]any{"target_type": "product", "target_id": 42})
	require.NoError(t, err)
	ticket, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose: model.AuthFlowPurposeMiniAppCheckout, Provider: "wechat-miniapp", Intent: model.AuthFlowIntentCheckout,
		UserId: owner.Id, SessionId: miniSession.Session.SID, Payload: string(payload), ExpiresAt: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/miniapp/checkout/confirm", bytes.NewBufferString(`{"checkout_ticket":"`+ticket+`"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("id", owner.Id)
	context.Set("session_id", browserSession.Session.SID)
	context.Set("auth_version", int64(1))
	context.Set("session_version", int64(1))

	ConfirmMiniAppBrowserCheckout(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"checkout_path":"/products/42"`)
	flow, err := model.GetAuthFlowState(ticket, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeMiniAppCheckout, Provider: "wechat-miniapp", Intent: model.AuthFlowIntentCheckout, UserId: owner.Id,
	})
	require.NoError(t, err)
	assert.NotNil(t, flow.ConsumedAt)
}

func TestMiniAppBrowserCheckoutRejectsAnotherUserWithoutConsumingTheHandoff(t *testing.T) {
	db := setupMiniAppCommerceControllerTest(t)
	owner := seedMiniAppCommerceUser(t, db, "mini-browser-checkout-owner")
	other := seedMiniAppCommerceUser(t, db, "mini-browser-checkout-other")
	miniSession, err := service.CreateMiniAppLoginSession(owner.Id, "127.0.0.1", "mini-checkout-test")
	require.NoError(t, err)
	browserSession, err := service.CreateLoginSession(other.Id, "password", "127.0.0.1", "browser-checkout-test")
	require.NoError(t, err)
	payload, err := common.Marshal(map[string]any{"target_type": "plan", "target_id": 7})
	require.NoError(t, err)
	ticket, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose: model.AuthFlowPurposeMiniAppCheckout, Provider: "wechat-miniapp", Intent: model.AuthFlowIntentCheckout,
		UserId: owner.Id, SessionId: miniSession.Session.SID, Payload: string(payload), ExpiresAt: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/miniapp/checkout/confirm", bytes.NewBufferString(`{"checkout_ticket":"`+ticket+`"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("id", other.Id)
	context.Set("session_id", browserSession.Session.SID)
	context.Set("auth_version", int64(1))
	context.Set("session_version", int64(1))

	ConfirmMiniAppBrowserCheckout(context)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	flow, err := model.GetAuthFlowState(ticket, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeMiniAppCheckout, Provider: "wechat-miniapp", Intent: model.AuthFlowIntentCheckout, UserId: owner.Id,
	})
	require.NoError(t, err)
	assert.Nil(t, flow.ConsumedAt)
}
