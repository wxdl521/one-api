package service

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRequireMiniProgramConfigRejectsDisabledFeatures(t *testing.T) {
	originalMiniProgramEnabled := common.MiniProgramEnabled
	originalTextTestEnabled := common.MiniProgramTextTestEnabled
	t.Cleanup(func() {
		common.MiniProgramEnabled = originalMiniProgramEnabled
		common.MiniProgramTextTestEnabled = originalTextTestEnabled
	})

	common.MiniProgramEnabled = false
	common.MiniProgramTextTestEnabled = false
	_, err := RequireMiniProgramConfig()
	require.ErrorIs(t, err, ErrMiniProgramDisabled)

	common.MiniProgramEnabled = true
	common.MiniProgramTextTestEnabled = false
	_, err = RequireMiniProgramTextTestConfig()
	require.ErrorIs(t, err, ErrMiniProgramTextTestDisabled)

	common.MiniProgramEnabled = false
	common.MiniProgramTextTestEnabled = true
	_, err = RequireMiniProgramTextTestConfig()
	require.ErrorIs(t, err, ErrMiniProgramDisabled)
}

func TestMiniProgramTextTestOptionRequiresMiniProgramOption(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}))

	originalDB := model.DB
	originalMiniProgramEnabled := common.MiniProgramEnabled
	originalTextTestEnabled := common.MiniProgramTextTestEnabled
	originalOptionMap := common.OptionMap
	t.Cleanup(func() {
		model.DB = originalDB
		common.MiniProgramEnabled = originalMiniProgramEnabled
		common.MiniProgramTextTestEnabled = originalTextTestEnabled
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})
	model.DB = db
	common.MiniProgramEnabled = false
	common.MiniProgramTextTestEnabled = false
	model.InitOptionMap()
	common.OptionMapRWMutex.RLock()
	assert.Equal(t, "false", common.OptionMap["MiniProgramEnabled"])
	assert.Equal(t, "false", common.OptionMap["MiniProgramTextTestEnabled"])
	common.OptionMapRWMutex.RUnlock()

	err = model.UpdateOption("MiniProgramTextTestEnabled", "true")
	require.Error(t, err)
	assert.False(t, common.MiniProgramTextTestEnabled)

	require.NoError(t, model.UpdateOption("MiniProgramEnabled", "true"))
	require.NoError(t, model.UpdateOption("MiniProgramTextTestEnabled", "true"))
	assert.True(t, common.MiniProgramTextTestEnabled)
}

func TestNewMiniAppConfigRejectsMissingCredentialsWithoutExposingSecrets(t *testing.T) {
	for name, credentials := range map[string]struct {
		appID     string
		appSecret string
	}{
		"missing app id":     {appSecret: "super-secret"},
		"missing app secret": {appID: "wx123"},
	} {
		t.Run(name, func(t *testing.T) {
			config, err := newMiniAppConfig(
				credentials.appID,
				credentials.appSecret,
				"https://console.example.com/miniapp/bind/",
				10*time.Second,
				false,
			)

			require.ErrorIs(t, err, ErrMiniAppConfiguration)
			assert.Empty(t, config)
			assert.NotContains(t, strings.ToLower(err.Error()), "secret")
			assert.NotContains(t, err.Error(), "super-secret")
		})
	}
}

func TestNewMiniAppConfigRejectsInvalidBindWebBaseURL(t *testing.T) {
	for name, bindWebBaseURL := range map[string]string{
		"http outside local development": "http://console.example.com/miniapp/bind",
		"missing scheme":                 "console.example.com/miniapp/bind",
		"missing host":                   "https:///miniapp/bind",
		"query string":                   "https://console.example.com/miniapp/bind?next=https://attacker.example",
		"fragment":                       "https://console.example.com/miniapp/bind#fragment",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := newMiniAppConfig("wx123", "super-secret", bindWebBaseURL, 10*time.Second, false)

			require.ErrorIs(t, err, ErrMiniAppConfiguration)
			assert.NotContains(t, err.Error(), "super-secret")
		})
	}
}

func TestNewMiniAppConfigNormalizesAndRedactsConfiguration(t *testing.T) {
	config, err := newMiniAppConfig(
		"wx123",
		"super-secret",
		"https://CONSOLE.example.com/miniapp/bind///",
		12*time.Second,
		false,
	)
	require.NoError(t, err)

	assert.Equal(t, "wx123", config.AppID)
	assert.Equal(t, "https://console.example.com/miniapp/bind", config.BindWebBaseURL)
	assert.Equal(t, 12*time.Second, config.HTTPTimeout)

	serialized, err := common.Marshal(config)
	require.NoError(t, err)
	assert.NotContains(t, string(serialized), "super-secret")
}

func TestNewMiniAppConfigAllowsHTTPOnlyForLocalDevelopment(t *testing.T) {
	config, err := newMiniAppConfig(
		"wx123",
		"super-secret",
		"http://localhost:3000/miniapp/bind/",
		10*time.Second,
		true,
	)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:3000/miniapp/bind", config.BindWebBaseURL)
}
