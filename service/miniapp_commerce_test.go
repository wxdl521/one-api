package service

import (
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMiniAppCheckoutHandoffRequiresHTTPSBusinessDomain(t *testing.T) {
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	previousDebug := common.DebugEnabled
	previousAppID := common.WeChatMiniAppAppID
	previousAppSecret := common.WeChatMiniAppAppSecret
	previousSubjectKey := common.WeChatMiniAppSubjectHMACKey
	previousBindURL := common.MiniAppBindWebBaseURL
	previousTimeout := common.MiniAppHTTPTimeout
	previousEnabled, previousTextEnabled := common.GetMiniProgramFeatureFlags()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Product{}, &model.AuthFlow{}))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.DebugEnabled = true
	common.WeChatMiniAppAppID = "wx-checkout-test"
	common.WeChatMiniAppAppSecret = "server-only-secret"
	common.WeChatMiniAppSubjectHMACKey = "checkout-subject-key"
	common.MiniAppBindWebBaseURL = "http://localhost:3000/miniapp-bind"
	common.MiniAppHTTPTimeout = 10 * time.Second
	common.OptionMapRWMutex.Lock()
	common.MiniProgramEnabled = true
	common.MiniProgramTextTestEnabled = false
	common.OptionMapRWMutex.Unlock()
	miniAppConfigOnce = sync.Once{}
	cachedMiniAppConfig = MiniAppConfig{}
	cachedMiniAppConfigErr = nil
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
		common.DebugEnabled = previousDebug
		common.WeChatMiniAppAppID = previousAppID
		common.WeChatMiniAppAppSecret = previousAppSecret
		common.WeChatMiniAppSubjectHMACKey = previousSubjectKey
		common.MiniAppBindWebBaseURL = previousBindURL
		common.MiniAppHTTPTimeout = previousTimeout
		common.OptionMapRWMutex.Lock()
		common.MiniProgramEnabled = previousEnabled
		common.MiniProgramTextTestEnabled = previousTextEnabled
		common.OptionMapRWMutex.Unlock()
		miniAppConfigOnce = sync.Once{}
		cachedMiniAppConfig = MiniAppConfig{}
		cachedMiniAppConfigErr = nil
	})

	product := &model.Product{Name: "Checkout product", PriceCents: 2500, ProductType: model.ProductTypeManual, Enabled: true}
	require.NoError(t, db.Create(product).Error)

	_, err = StartMiniAppCheckout(7, "mini-checkout-session", miniAppCheckoutProduct, product.Id)

	require.ErrorIs(t, err, ErrMiniAppConfiguration)
}
