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

// MiniAppConfig contains the non-secret configuration that mini program
// services may use. The AppSecret stays in protected server configuration.
type MiniAppConfig struct {
	AppID          string        `json:"-"`
	BindWebBaseURL string        `json:"-"`
	HTTPTimeout    time.Duration `json:"-"`
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
			common.MiniAppBindWebBaseURL,
			common.MiniAppHTTPTimeout,
			common.DebugEnabled,
		)
	})
	return cachedMiniAppConfig, cachedMiniAppConfigErr
}

// RequireMiniProgramConfig is the feature gate for Mini Program BFF requests.
func RequireMiniProgramConfig() (MiniAppConfig, error) {
	if !common.MiniProgramEnabled {
		return MiniAppConfig{}, ErrMiniProgramDisabled
	}
	return GetMiniAppConfig()
}

// RequireMiniProgramTextTestConfig is the feature gate for mini program text
// testing. Text testing cannot be enabled independently of the mini program.
func RequireMiniProgramTextTestConfig() (MiniAppConfig, error) {
	if !common.MiniProgramEnabled {
		return MiniAppConfig{}, ErrMiniProgramDisabled
	}
	if !common.MiniProgramTextTestEnabled {
		return MiniAppConfig{}, ErrMiniProgramTextTestDisabled
	}
	return GetMiniAppConfig()
}

func newMiniAppConfig(appID string, appSecret string, bindWebBaseURL string, httpTimeout time.Duration, localDevelopment bool) (MiniAppConfig, error) {
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(appSecret) == "" || httpTimeout <= 0 {
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
	bindURL.RawPath = ""
	return MiniAppConfig{
		AppID:          strings.TrimSpace(appID),
		BindWebBaseURL: bindURL.String(),
		HTTPTimeout:    httpTimeout,
	}, nil
}
