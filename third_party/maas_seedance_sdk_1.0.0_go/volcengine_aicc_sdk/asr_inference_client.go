package volcengine_aicc_sdk

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"maas_seedance_sdk_1.0.0_go/volcengine_aicc_sdk/common"
	"maas_seedance_sdk_1.0.0_go/volcengine_aicc_sdk/jeddak_secure_channel"
)

// ASRClient ASR客户端
type ASRClient struct {
	*BaseInference
	AppID           string
	Token           string
	ServiceID       string
	AICCServiceName string
	PolicyID        string
	kwargs          map[string]interface{}
	jscClient       *jeddak_secure_channel.Client
	MaxRetries      int
}

// NewASRClient 创建新的ASR客户端
func NewASRClient(baseURL string, aiccServiceName string, policyID string, isOpenAI bool, isSecure bool,
	ak string, sk string, region string, kwargs map[string]interface{}) *ASRClient {

	appID := ""
	if id, ok := kwargs["appid"].(string); ok {
		appID = id
	}

	token := ""
	if t, ok := kwargs["token"].(string); ok {
		token = t
	}

	serviceID := ""
	if id, ok := kwargs["service_id"].(string); ok {
		serviceID = id
	}

	if baseURL == "" || appID == "" || token == "" || serviceID == "" ||
		aiccServiceName == "" || policyID == "" || ak == "" || sk == "" {
		panic(common.NewAICCException("New ASR client failed, params error"))
	}

	client := &ASRClient{
		BaseInference:   NewBaseInference(baseURL, isOpenAI, isSecure, ak, sk, region),
		AppID:           appID,
		Token:           token,
		ServiceID:       serviceID,
		AICCServiceName: aiccServiceName,
		PolicyID:        policyID,
		kwargs:          kwargs,
		MaxRetries:      3,
	}

	// 初始化JSC客户端
	if isSecure {
		// 构建bytedance_top_info
		topInfo := map[string]string{
			"ak":      ak,
			"sk":      sk,
			"region":  region,
			"url":     "open.volcengineapi.com",
			"service": "pcc",
		}

		// 如果提供了top_url和top_service，使用提供的值
		if url, ok := kwargs["top_url"].(string); ok {
			topInfo["url"] = url
		}

		if service, ok := kwargs["top_service"].(string); ok {
			topInfo["service"] = service
		}

		topInfoJSON, err := json.Marshal(topInfo)
		if err != nil {
			fmt.Printf("Error marshaling topInfo: %v\n", err)
			panic(common.NewAICCException("New ASR client failed"))
		}

		// 构建JSC配置
		trueVal := true
		jscConfig := jeddak_secure_channel.ClientConfig{
			RaType:           jeddak_secure_channel.RA_TYPE_TCA,
			RaServiceName:    aiccServiceName,
			RaPolicyId:       policyID,
			BytedanceTopInfo: string(topInfoJSON),
			RaKeyNegotiation: &trueVal,
			RaNeedToken:      &trueVal,
		}

		// 初始化JSC客户端
		client.jscClient = jeddak_secure_channel.NewClient(jscConfig)
		client.jscClient.AttestServer()
	}

	return client
}

// ChatCompletions 实现聊天补全（空实现）
func (c *ASRClient) ChatCompletions(messages []map[string]interface{}, outputWriter io.Writer) (string, error) {
	return "", nil
}

// FunctionCall 实现函数调用（空实现）
func (c *ASRClient) FunctionCall(messages []map[string]interface{}, tools []map[string]interface{}, outputWriter io.Writer) (string, error) {
	return "", nil
}

// ASRRecognize 实现ASR识别
func (c *ASRClient) ASRRecognize(fileName string) (string, error) {
	if fileName == "" {
		return "", common.NewAICCException("ASR asr_recognize file_name cannot be empty")
	}

	appID := c.AppID
	token := c.Token
	serviceID := c.ServiceID
	modelName := "bigmodel"

	// 构建URL
	var recognizeURL string
	if strings.HasSuffix(c.BaseURL, "flash") {
		recognizeURL = c.BaseURL
	} else {
		if c.IsSecure {
			recognizeURL = fmt.Sprintf("%s/api/v2/auc/%s/recognize/flash", c.BaseURL, modelName)
		} else {
			recognizeURL = fmt.Sprintf("%s/api/v1/auc/%s/recognize/flash", c.BaseURL, modelName)
		}
	}

	// 转换文件为Base64
	encBase64Data, err := common.FileToBase64(fileName)
	if err != nil {
		return "", common.WrapError(err, "file to base64 failed")
	}

	// 加密数据
	var encryptResult *jeddak_secure_channel.EncryptResult
	if c.IsSecure && c.jscClient != nil {
		encryptResult, err = c.jscClient.EncryptStringWithResponse(encBase64Data)
		if err != nil {
			return "", common.WrapError(err, "encrypt data failed")
		}
		encBase64Data = encryptResult.Ciphertext
	}

	audioData := map[string]string{
		"data": encBase64Data,
	}

	// 构建请求头
	headers := map[string]string{
		"X-Api-App-Id":        appID,
		"X-Api-App-Key":       token,
		"X-Api-Aicc-Endpoint": serviceID,
		"X-Api-Request-Id":    fmt.Sprintf("%d", time.Now().UnixNano()),
		"Content-Type":        "application/json",
	}

	// 构建请求体
	reqBody := map[string]interface{}{
		"audio": audioData,
		"request": map[string]string{
			"model_name": modelName,
		},
	}

	// 发送请求
	resp, err := c.sendRequest(recognizeURL, reqBody, headers)
	if err != nil {
		return "", common.WrapError(err, "send request failed")
	}

	// 处理响应
	result, err := c.processResponse(resp, encryptResult)
	if err != nil {
		return "", common.WrapError(err, "process response failed")
	}

	return result, nil
}

// sendRequest 发送HTTP请求
func (c *ASRClient) sendRequest(url string, body map[string]interface{}, headers map[string]string) (*http.Response, error) {
	// 构建请求
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second, // 设置合理超时
	}

	// 创建请求
	req, err := http.NewRequest("POST", url, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}

	// 设置请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// processResponse 处理响应
func (c *ASRClient) processResponse(resp *http.Response, encryptResult *jeddak_secure_channel.EncryptResult) (string, error) {
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return "", common.NewAICCException("ASR处理失败")
	}

	// 解析响应
	var result map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", err
	}

	// 检查结果是否存在
	if _, ok := result["result"]; !ok {
		return "", common.NewAICCException("响应结果格式错误")
	}

	// 解密数据
	if c.IsSecure && encryptResult != nil {
		content, ok := result["result"]
		if !ok {
			return "", common.NewAICCException("响应结果格式错误")
		}
		contentStr, ok := content.(string)
		if !ok {
			return "", common.NewAICCException("响应结果格式错误")
		}

		// 使用响应密钥解密数据
		decryptedContent, err := encryptResult.ResponseKey.DecryptString(contentStr)
		if err != nil {
			return "", common.WrapError(err, "decrypt content failed")
		}

		// 解析解密后的JSON
		var decryptedData map[string]interface{}
		err = json.Unmarshal([]byte(decryptedContent), &decryptedData)
		if err != nil {
			return "", common.WrapError(err, "unmarshal decrypted content failed")
		}

		result["result"] = decryptedData
	}

	// 转换为JSON字符串
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	return string(resultJSON), nil
}
