package service

import (
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/the-one/common"
)

var ErrMiniProgramDisabled = errors.New("mini program is disabled")
var ErrMiniProgramTextTestDisabled = errors.New("mini program text testing is disabled")
var ErrMiniAppConfiguration = errors.New("mini program configuration is incomplete")

const miniAppBindWebPath = "/miniapp-bind"

// MiniAppConfig contains Mini Program configuration. The server-only secrets
// are deliberately unexported and excluded from serialization. The subject
// HMAC key must remain stable across restarts; rotating its v1 value requires
// an explicit identity migration.
type MiniAppConfig struct {
	AppID          string        `json:"-"`
	BindWebBaseURL string        `json:"-"` // Must resolve to the frontend's fixed /miniapp-bind route.
	HTTPTimeout    time.Duration `json:"-"`
	appSecret      string
	subjectHMACKey string
}

var miniAppConfigOnce sync.Once
var cachedMiniAppConfig MiniAppConfig
var cachedMiniAppConfigErr error

// GetMiniAppConfig loads and normalizes server-side mini program configuration
// once after common.InitEnv has read the environment.
func GetMiniAppConfig() (MiniAppConfig, error) {
	miniAppConfigOnce.Do(func() {
		cachedMiniAppConfig, cachedMiniAppConfigErr = newMiniAppConfig(
			common.WeChatMiniAppAppID,
			common.WeChatMiniAppAppSecret,
			common.WeChatMiniAppSubjectHMACKey,
			common.MiniAppBindWebBaseURL,
			common.MiniAppHTTPTimeout,
			common.DebugEnabled,
		)
	})
	return cachedMiniAppConfig, cachedMiniAppConfigErr
}

// RequireMiniProgramConfig is the feature gate for Mini Program BFF requests.
func RequireMiniProgramConfig() (MiniAppConfig, error) {
	miniProgramEnabled, _ := common.GetMiniProgramFeatureFlags()
	if !miniProgramEnabled {
		return MiniAppConfig{}, ErrMiniProgramDisabled
	}
	return GetMiniAppConfig()
}

// RequireMiniProgramTextTestConfig is the feature gate for mini program text
// testing. Text testing cannot be enabled independently of the mini program.
func RequireMiniProgramTextTestConfig() (MiniAppConfig, error) {
	miniProgramEnabled, miniProgramTextTestEnabled := common.GetMiniProgramFeatureFlags()
	if !miniProgramEnabled {
		return MiniAppConfig{}, ErrMiniProgramDisabled
	}
	if !miniProgramTextTestEnabled {
		return MiniAppConfig{}, ErrMiniProgramTextTestDisabled
	}
	return GetMiniAppConfig()
}

func newMiniAppConfig(appID string, appSecret string, subjectHMACKey string, bindWebBaseURL string, httpTimeout time.Duration, localDevelopment bool) (MiniAppConfig, error) {
	appID = strings.TrimSpace(appID)
	appSecret = strings.TrimSpace(appSecret)
	subjectHMACKey = strings.TrimSpace(subjectHMACKey)
	if appID == "" || appSecret == "" || subjectHMACKey == "" || httpTimeout <= 0 {
		return MiniAppConfig{}, ErrMiniAppConfiguration
	}

	bindURL, err := url.Parse(strings.TrimSpace(bindWebBaseURL))
	if err != nil || bindURL.Host == "" || bindURL.User != nil || bindURL.RawQuery != "" || bindURL.Fragment != "" {
		return MiniAppConfig{}, ErrMiniAppConfiguration
	}

	bindURL.Scheme = strings.ToLower(bindURL.Scheme)
	bindURL.Host = strings.ToLower(bindURL.Host)
	if bindURL.Scheme != "https" && !(localDevelopment && bindURL.Scheme == "http" && bindURL.Hostname() == "localhost") {
		return MiniAppConfig{}, ErrMiniAppConfiguration
	}

	bindURL.Path = strings.TrimRight(bindURL.Path, "/")
	if bindURL.Path != miniAppBindWebPath || bindURL.RawPath != "" {
		return MiniAppConfig{}, ErrMiniAppConfiguration
	}
	bindURL.RawPath = ""
	return MiniAppConfig{
		AppID:          appID,
		BindWebBaseURL: bindURL.String(),
		HTTPTimeout:    httpTimeout,
		appSecret:      appSecret,
		subjectHMACKey: subjectHMACKey,
	}, nil
}
