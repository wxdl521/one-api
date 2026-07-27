package volcengine_aicc_sdk

import (
	"fmt"
	"strings"

	"maas_seedance_sdk_1.0.0_go/volcengine_aicc_sdk/common"
	"maas_seedance_sdk_1.0.0_go/volcengine_aicc_sdk/common/log"
)

const (
	DOUBAO   = "DOUBAO"   // 机密豆包
	AICC_CU  = "AICC-CU"  // AICC CU版模型
	OSLM     = "OSLM"     // 开源模型
	ASR      = "ASR"      // 语音识别
	SEEDANCE = "SEEDANCE" // Seedance视频生成
)

type AICCClient struct {
	BaseURL    string
	APIKey     string
	Endpoint   string
	AICCConfig *AICCConfig
	Kwargs     map[string]interface{}
	config     map[string]interface{}
	logger     *log.Logger
}

type AICCClientOption func(*AICCClient)

func WithBaseURL(baseURL string) AICCClientOption {
	return func(c *AICCClient) {
		c.BaseURL = baseURL
	}
}

func WithAPIKey(apiKey string) AICCClientOption {
	return func(c *AICCClient) {
		c.APIKey = apiKey
	}
}

func WithEndpoint(endpoint string) AICCClientOption {
	return func(c *AICCClient) {
		c.Endpoint = endpoint
	}
}

func WithAICCConfig(aiccConfig *AICCConfig) AICCClientOption {
	return func(c *AICCClient) {
		c.AICCConfig = aiccConfig
	}
}

func WithKwargs(kwargs map[string]interface{}) AICCClientOption {
	return func(c *AICCClient) {
		c.Kwargs = kwargs
	}
}

func NewAICCClient(opts ...AICCClientOption) (*AICCClient, error) {
	client := &AICCClient{
		Kwargs: make(map[string]interface{}),
		config: make(map[string]interface{}),
		logger: log.NewLogger(),
	}

	for _, opt := range opts {
		opt(client)
	}

	if client.BaseURL == "" || client.APIKey == "" {
		return nil, common.NewAICCException(fmt.Sprintf("New AICC client failed, base_url=%s, api_key=%s", client.BaseURL, client.APIKey))
	}

	if client.Endpoint == "" {
		if modelName, ok := client.Kwargs["model_name"].(string); ok {
			client.Endpoint = modelName
		}
	}

	client._initConfig()
	client._initLog()

	return client, nil
}

func (c *AICCClient) GetLLMInstance(modelType string) (*SeedanceClient, error) {
	modelType = strings.ToUpper(modelType)

	if strings.Contains(modelType, SEEDANCE) {
		opts := []SeedanceClientOption{
			WithSeedanceBaseURL(c.BaseURL),
			WithSeedanceAPIKey(c.APIKey),
			WithSeedanceEndpoint(c.Endpoint),
			WithSeedanceAICCConfig(c.AICCConfig),
		}

		if isSecure, ok := c.Kwargs["is_secure"].(bool); ok {
			opts = append(opts, WithSeedanceIsSecure(isSecure))
		}

		if enableEncrypt, ok := c.Kwargs["is_enable_video_encrypt"].(string); ok {
			opts = append(opts, WithSeedanceIsEnableVideoEncrypt(strings.ToLower(enableEncrypt) == "true"))
		}

		if location, ok := c.Kwargs["video_file_storage_location"].(string); ok {
			opts = append(opts, WithSeedanceVideoFileStorageLocation(location))
		}

		if timeout, ok := c.Kwargs["timeout"].(float64); ok {
			opts = append(opts, WithSeedanceTimeout(timeout))
		}

		return NewSeedanceClient(opts...), nil
	}

	return nil, common.NewAICCException(fmt.Sprintf("New AICC client failed, model_type=%s", modelType))
}

func (c *AICCClient) GetInferenceInstance(modelType string, baseURL string, apiKey string, ep string,
	aiccServiceName string, policyID string, isOpenAI bool, isSecure bool,
	ak string, sk string, region string, kwargs map[string]interface{}) (AICCInference, error) {

	modelType = strings.ToUpper(modelType)

	switch modelType {
	case DOUBAO:
		return NewARKClient(baseURL, apiKey, ep, isOpenAI, isSecure, ak, sk, region, kwargs), nil
	case AICC_CU:
		return NewCUClient(baseURL, aiccServiceName, policyID, isOpenAI, isSecure, ak, sk, region, kwargs), nil
	case OSLM:
		return NewDeepSeekClient(baseURL, apiKey, ep, aiccServiceName, policyID, isOpenAI, isSecure, ak, sk, region, kwargs), nil
	case ASR:
		return NewASRClient(baseURL, aiccServiceName, policyID, isOpenAI, isSecure, ak, sk, region, kwargs), nil
	default:
		errStr := fmt.Sprintf("can not support model type:%s", modelType)
		c.logger.Error(errStr)
		return nil, common.NewAICCException(errStr)
	}
}

func (c *AICCClient) GetVolcengineArk(baseURL string, apiKey string, kwargs map[string]interface{}) interface{} {
	return nil
}

func (c *AICCClient) Hello() string {
	info := "hello volcengine_aicc_sdk"
	fmt.Println(info)
	return info
}

func (c *AICCClient) _initConfig() {
}

func (c *AICCClient) _initLog() {
	config := log.NewLogConfig()

	if logDir, ok := c.config["dir"].(string); ok && logDir != "" {
		config.Dir = logDir
	} else {
		config.Dir = "aicc_log"
	}

	if filename, ok := c.config["filename"].(string); ok && filename != "" {
		config.Filename = filename
	} else {
		config.Filename = "aicc.log"
	}

	if rotation, ok := c.config["rotation"].(int); ok && rotation > 0 {
		config.Rotation = rotation
	} else {
		config.Rotation = 100
	}

	if retention, ok := c.config["retention"].(int); ok && retention > 0 {
		config.Retention = retention
	} else {
		config.Retention = 30
	}

	if level, ok := c.config["level"].(string); ok && level != "" {
		config.Level = level
	}

	c.logger.InitLogConfig(config)
}

func (c *AICCClient) GetConfig() map[string]interface{} {
	return c.config
}

func (c *AICCClient) SetConfig(key string, value interface{}) {
	c.config[key] = value
}
