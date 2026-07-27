package volcengine_aicc_sdk

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"maas_seedance_sdk_1.0.0_go/volcengine_aicc_sdk/common"
	"maas_seedance_sdk_1.0.0_go/volcengine_aicc_sdk/common/log"
	"maas_seedance_sdk_1.0.0_go/volcengine_aicc_sdk/jeddak_secure_channel"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// DeepSeekClient DeepSeek客户端
type DeepSeekClient struct {
	*BaseInference
	APIKey          string
	EP              string
	AICCServiceName string
	PolicyID        string
	kwargs          map[string]interface{}
	jscClient       *jeddak_secure_channel.Client
	MaxRetries      int
	openaiClient    openai.Client
}

// EncryptWithResponse 加密消息内容，返回加密结果
func (c *DeepSeekClient) EncryptWithResponse(plaintext string) (*jeddak_secure_channel.EncryptResult, error) {
	if c.jscClient == nil {
		return nil, errors.New("jscClient is not initialized")
	}
	return c.jscClient.EncryptStringWithResponse(plaintext)
}

type ChatCompletionRequest struct {
	Model              string             `json:"model"`
	Messages           interface{}        `json:"messages"`
	MaxTokens          int                `json:"max_tokens"`
	Temperature        float64            `json:"temperature"`
	Stream             bool               `json:"stream,omitempty"`
	Stop               string             `json:"stop,omitempty"`
	EosToken           string             `json:"eos_token,omitempty"`
	ChatTemplateKwargs ChatTemplateKwargs `json:"chat_template_kwargs"`
	SeparateReasoning  bool               `json:"separate_reasoning"`
}

// ChatCompletionResponse 聊天补全响应

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Delta struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type StreamChoice struct {
	Delta        Delta  `json:"delta"`
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
}

type StreamResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
	Usage   Usage          `json:"usage,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ResponseChoice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type ChatCompletionResponse struct {
	ID                string           `json:"id"`
	Object            string           `json:"object"`
	Created           int64            `json:"created"`
	Model             string           `json:"model"`
	SystemFingerprint string           `json:"system_fingerprint"`
	Choices           []ResponseChoice `json:"choices"`
	Usage             Usage            `json:"usage"`
}

// ChatTemplateKwargs 聊天模板参数
type ChatTemplateKwargs struct {
	Thinking bool `json:"thinking"`
}

// NewDeepSeekClient 创建新的DeepSeek客户端
func NewDeepSeekClient(baseURL string, apiKey string, ep string, aiccServiceName string, policyID string,
	isOpenAI bool, isSecure bool, ak string, sk string, region string, kwargs map[string]interface{}) *DeepSeekClient {

	if baseURL == "" || apiKey == "" || aiccServiceName == "" || policyID == "" || ak == "" || sk == "" {
		panic(common.NewAICCException("New deepseek client failed, params error"))
	}

	client := &DeepSeekClient{
		BaseInference:   NewBaseInference(baseURL, isOpenAI, isSecure, ak, sk, region),
		APIKey:          apiKey,
		EP:              ep,
		AICCServiceName: aiccServiceName,
		PolicyID:        policyID,
		kwargs:          kwargs,
		MaxRetries:      3,
	}

	// 检查SK是否有效
	skIsValid := sk != "" && sk != "=="

	// 初始化OpenAI客户端（如果需要）
	if isOpenAI {
		client.openaiClient = openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL(baseURL),
		)
		log.Info(fmt.Sprintf("DeepSeek OpenAI client initialized successfully"))
	}

	// 初始化JSC客户端（如果需要安全通道）
	if isSecure && skIsValid {
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
			panic(common.NewAICCException("New deepseek client failed"))
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
		// 执行服务器认证
		err = client.jscClient.AttestServer()
		if err != nil {
			fmt.Printf("Error attesting server: %v\n", err)
			panic(common.NewAICCException("New deepseek client failed, attest server error"))
		}

		// 如果SK无效但需要安全通道，设置IsSecure为false
		if isSecure && !skIsValid {
			fmt.Println("SK is invalid, disabling secure channel")
			client.IsSecure = false
		}
	}

	return client
}

// sendRequest 发送HTTP请求
func (c *DeepSeekClient) sendRequest(url string, body map[string]interface{}, headers map[string]string) (*http.Response, error) {
	// 构建请求
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	// 创建HTTP客户端
	client := &http.Client{}

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

// FunctionCall 实现函数调用
func (c *DeepSeekClient) FunctionCall(messages []map[string]interface{}, tools []map[string]interface{}, outputWriter io.Writer) (string, error) {
	if len(messages) == 0 || len(tools) == 0 {
		return "", common.NewAICCException("DeepSeek function_call params cannot be empty")
	}

	// DeepSeek目前仅支持通过openai的方式使用function_call
	if !c.IsOpenAI {
		errorInfo := fmt.Sprintf("Sorry, DeepSeek is_openai=%v not support function_call.", c.IsOpenAI)
		log.Error(errorInfo)
		return "", common.NewAICCException(errorInfo)
	}

	// 构建URL
	var url string
	if c.IsSecure {
		url = fmt.Sprintf("%s/v4", c.BaseURL)
	} else {
		url = fmt.Sprintf("%s/v1", c.BaseURL)
	}

	// 构建请求头
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": fmt.Sprintf("Bearer %s", c.APIKey),
	}

	// 加密数据
	var encryptResult *jeddak_secure_channel.EncryptResult
	var encryptedMessages []map[string]interface{}
	var encryptedTools string

	if c.IsSecure && c.jscClient != nil {
		encryptedMessages = make([]map[string]interface{}, len(messages))
		for i, message := range messages {
			encryptedMessage := make(map[string]interface{})
			for k, v := range message {
				if k == "content" {
					contentJSON, err := json.Marshal(v)
					if err != nil {
						return "", common.WrapError(err, "marshal content failed")
					}

					// 加密内容
					result, err := c.jscClient.EncryptBytesWithResponse(contentJSON)
					if err != nil {
						return "", common.WrapError(err, "encrypt content failed")
					}

					encryptedMessage[k] = result.Ciphertext
					if message["role"] == "user" {
						encryptResult = result
					}
				} else {
					encryptedMessage[k] = v
				}
			}
			encryptedMessages[i] = encryptedMessage
		}

		// 加密工具
		toolsJSON, err := json.Marshal(tools)
		if err != nil {
			return "", common.WrapError(err, "marshal tools failed")
		}

		toolsEncryptResult, err := c.jscClient.EncryptBytesWithResponse(toolsJSON)
		if err != nil {
			return "", common.WrapError(err, "encrypt tools failed")
		}

		encryptedTools = toolsEncryptResult.Ciphertext
	} else {
		encryptedMessages = messages
	}

	// 构建请求体
	payload := map[string]interface{}{
		"model":       c.EP,
		"messages":    encryptedMessages,
		"tools":       tools,
		"tool_choice": "required",
	}

	// 添加加密工具字段
	if c.IsSecure {
		payload["aicc_enc_tools"] = encryptedTools
	}

	// 处理流式响应
	if outputWriter != nil {
		payload["stream"] = true
		// 这里需要实现流式请求
		// 由于我们没有OpenAI SDK，这里暂时返回空
		return "", nil
	}

	// 处理非流式响应
	for attempt := 0; attempt < c.MaxRetries; attempt++ {
		// 发送请求
		resp, err := c.sendRequest(url, payload, headers)
		if err != nil {
			log.Error(fmt.Sprintf("call AICC DeepSeek service function_call error: %v, try again...", err))
			if attempt == c.MaxRetries-1 {
				return "", common.NewAICCException("DeepSeek function_call error")
			}
			time.Sleep(time.Second)
			continue
		}

		// 检查状态码
		if resp.StatusCode != http.StatusOK {
			respBody, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				log.Error(fmt.Sprintf("call AICC DeepSeek service function_call error: status code %d, try again...", resp.StatusCode))
				if attempt == c.MaxRetries-1 {
					return "", common.NewAICCException("DeepSeek function_call error")
				}
				time.Sleep(time.Second)
				continue
			}
			log.Error(fmt.Sprintf("call AICC DeepSeek service function_call error: %s, try again...", string(respBody)))
			if attempt == c.MaxRetries-1 {
				return "", common.NewAICCException("DeepSeek function_call error")
			}
			time.Sleep(time.Second)
			continue
		}

		// 解析响应
		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if err != nil {
			log.Error(fmt.Sprintf("call AICC DeepSeek service function_call error: %v, try again...", err))
			if attempt == c.MaxRetries-1 {
				return "", common.NewAICCException("DeepSeek function_call error")
			}
			time.Sleep(time.Second)
			continue
		}

		// 解密数据
		if c.IsSecure && encryptResult != nil {
			choices, ok := result["choices"].([]interface{})
			if !ok || len(choices) == 0 {
				log.Error("response format error: choices not found")
				if attempt == c.MaxRetries-1 {
					return "", common.NewAICCException("DeepSeek function_call error")
				}
				time.Sleep(time.Second)
				continue
			}

			choiceMap, ok := choices[0].(map[string]interface{})
			if !ok {
				log.Error("response format error: choice not found")
				if attempt == c.MaxRetries-1 {
					return "", common.NewAICCException("DeepSeek function_call error")
				}
				time.Sleep(time.Second)
				continue
			}

			message, ok := choiceMap["message"].(map[string]interface{})
			if !ok {
				log.Error("response format error: message not found")
				if attempt == c.MaxRetries-1 {
					return "", common.NewAICCException("DeepSeek function_call error")
				}
				time.Sleep(time.Second)
				continue
			}

			// 解密消息内容
			if content, ok := message["content"].(string); ok && content != "" {
				decryptedContent, err := encryptResult.ResponseKey.DecryptString(content)
				if err != nil {
					log.Error(fmt.Sprintf("decrypt content failed: %v, try again...", err))
					if attempt == c.MaxRetries-1 {
						return "", common.NewAICCException("DeepSeek function_call error")
					}
					time.Sleep(time.Second)
					continue
				}
				message["content"] = decryptedContent
			}

			// 解密工具调用
			if toolCalls, ok := message["tool_calls"].([]interface{}); ok {
				for _, toolCall := range toolCalls {
					toolCallMap, ok := toolCall.(map[string]interface{})
					if !ok {
						continue
					}

					if functionArgs, ok := toolCallMap["function"].(map[string]interface{}); ok {
						if args, ok := functionArgs["arguments"].(string); ok && args != "" {
							decryptedArgs, err := encryptResult.ResponseKey.DecryptString(args)
							if err != nil {
								log.Error(fmt.Sprintf("decrypt arguments failed: %v, try again...", err))
								if attempt == c.MaxRetries-1 {
									return "", common.NewAICCException("DeepSeek function_call error")
								}
								time.Sleep(time.Second)
								continue
							}
							functionArgs["arguments"] = decryptedArgs
						}
					}
				}
			}
		}

		// 转换为JSON字符串
		resultJSON, err := json.Marshal(result)
		if err != nil {
			log.Error(fmt.Sprintf("marshal result failed: %v, try again...", err))
			if attempt == c.MaxRetries-1 {
				return "", common.NewAICCException("DeepSeek function_call error")
			}
			time.Sleep(time.Second)
			continue
		}

		return string(resultJSON), nil
	}

	return "", common.NewAICCException("DeepSeek function_call error")
}

// ASRRecognize 实现ASR识别
func (c *DeepSeekClient) ASRRecognize(fileName string) (string, error) {
	errorInfo := "Sorry, this version does not support asr at this time."
	log.Error(errorInfo)
	return errorInfo, nil
}

// CreateChatCompletion 发送非流式聊天请求
func (c *DeepSeekClient) CreateChatCompletion(request ChatCompletionRequest, customHeaders map[string]string) (*ChatCompletionResponse, error) {
	if request.Messages == nil {
		return nil, common.NewAICCException("DeepSeek createChatCompletion messages cannot be empty")
	}

	// 检查Messages类型
	var messages []map[string]interface{}
	var ok bool
	if messages, ok = request.Messages.([]map[string]interface{}); !ok {
		return nil, common.NewAICCException("DeepSeek createChatCompletion messages must be a slice of map[string]interface{}")
	}

	if len(messages) == 0 {
		return nil, common.NewAICCException("DeepSeek createChatCompletion messages cannot be empty")
	}

	// 构建URL
	url := c.getEndpoint()

	// 构建请求头
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": fmt.Sprintf("Bearer %s", c.APIKey),
	}

	// 添加自定义请求头
	if customHeaders != nil {
		for k, v := range customHeaders {
			headers[k] = v
		}
	}

	// 加密数据
	var encryptResult *jeddak_secure_channel.EncryptResult
	if c.IsSecure && c.jscClient != nil {
		// 只保留messages中的role和content字段
		var filteredMessages []map[string]interface{}
		for _, msg := range messages {
			filteredMsg := make(map[string]interface{})
			if role, ok := msg["role"].(string); ok {
				filteredMsg["role"] = role
			}
			if content, ok := msg["content"].(string); ok {
				filteredMsg["content"] = content
			}
			if len(filteredMsg) > 0 {
				filteredMessages = append(filteredMessages, filteredMsg)
			}
		}

		// 使用手动实现的pickle序列化filteredMessages
		pickleData, err := pickleEncode(filteredMessages)
		if err != nil {
			return nil, common.WrapError(err, "pickle encode messages failed")
		}

		// 使用EncryptBytesWithResponse加密pickle序列化后的字节
		encryptResult, err = c.jscClient.EncryptBytesWithResponse(pickleData)
		if err != nil {
			return nil, common.WrapError(err, "encrypt messages failed")
		}

		request.Messages = encryptResult.Ciphertext
	}

	// 确保Stream为false
	request.Stream = false

	// 将请求结构体转换为map
	payload, err := structToMap(request)
	if err != nil {
		return nil, common.WrapError(err, "convert request to map failed")
	}

	// 发送请求
	resp, err := c.sendRequest(url, payload, headers)
	if err != nil {
		return nil, common.WrapError(err, "send request failed")
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, common.NewAICCException(fmt.Sprintf("DeepSeek createChatCompletion error: status code %d", resp.StatusCode))
		}
		return nil, common.NewAICCException(fmt.Sprintf("DeepSeek createChatCompletion error: %s", string(respBody)))
	}

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, common.WrapError(err, "read response failed")
	}

	// 解析响应
	// 定义两种可能的响应结构体
	type tempResponseEncrypted struct {
		ID                string `json:"id"`
		Object            string `json:"object"`
		Created           int64  `json:"created"`
		Model             string `json:"model"`
		SystemFingerprint string `json:"system_fingerprint"`
		Choices           []struct {
			Index        int    `json:"index"`
			Message      string `json:"message"` // 加密模式下是JSON字符串
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}

	type tempResponsePlaintext struct {
		ID                string `json:"id"`
		Object            string `json:"object"`
		Created           int64  `json:"created"`
		Model             string `json:"model"`
		SystemFingerprint string `json:"system_fingerprint"`
		Choices           []struct {
			Index        int     `json:"index"`
			Message      Message `json:"message"` // 明文模式下直接是JSON对象
			FinishReason string  `json:"finish_reason"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}

	// 转换为标准响应格式
	response := &ChatCompletionResponse{}

	if c.IsSecure {
		// 加密模式
		var tempRespEncrypted tempResponseEncrypted
		if err := json.Unmarshal(respBody, &tempRespEncrypted); err != nil {
			return nil, common.WrapError(err, "unmarshal encrypted response failed")
		}

		response.ID = tempRespEncrypted.ID
		response.Object = tempRespEncrypted.Object
		response.Created = tempRespEncrypted.Created
		response.SystemFingerprint = tempRespEncrypted.SystemFingerprint
		response.Model = tempRespEncrypted.Model
		response.Usage = tempRespEncrypted.Usage
		response.Choices = make([]ResponseChoice, len(tempRespEncrypted.Choices))

		// 解密数据
		for i, tempChoice := range tempRespEncrypted.Choices {

			decryptedBytes, err := decryptContent(tempChoice.Message, encryptResult, c)
			if err != nil {
				log.Error(fmt.Sprintf("DeepSeek decrypt content failed with error: %v", err))
				return nil, common.WrapError(err, "decrypt content failed")
			}

			// 解析解密后的内容为完整的Message对象
			var decryptedMsg Message

			// 首先尝试直接pickle反序列化
			pickleData, err := pickleDecode(decryptedBytes)
			if err == nil && len(pickleData) > 0 {
				// pickle反序列化成功，提取第一个元素作为消息
				if content, ok := pickleData[0]["content"].(string); ok {
					decryptedMsg.Content = content
				}
				if role, ok := pickleData[0]["role"].(string); ok {
					decryptedMsg.Role = role
				} else {
					decryptedMsg.Role = "assistant" // 默认设置为assistant角色
				}
				// 提取其他可选字段（如果存在）
				// reasoning_content和tool_calls字段暂时不使用
				_ = pickleData[0]["reasoning_content"]
				_ = pickleData[0]["tool_calls"]
			} else {
				// 如果pickle反序列化失败，尝试JSON反序列化
				if err := json.Unmarshal(decryptedBytes, &decryptedMsg); err != nil {
					// 如果解析失败，尝试只解析content字段
					var contentOnly struct {
						Content string `json:"content"`
					}
					if err := json.Unmarshal(decryptedBytes, &contentOnly); err != nil {
						// 如果还是解析失败，尝试从pickle数据中直接提取content字段
						content := extractContentFromPickle(string(decryptedBytes))
						if content != "" {
							decryptedMsg.Content = content
							decryptedMsg.Role = "assistant"
						} else {
							// 最后尝试直接使用字符串形式的解密内容
							decryptedMsg.Content = string(decryptedBytes)
						}
					} else {
						decryptedMsg.Content = contentOnly.Content
						decryptedMsg.Role = "assistant" // 默认设置为assistant角色
					}
				}
			}

			// 赋值给响应对象
			response.Choices[i] = ResponseChoice{
				Index:        tempChoice.Index,
				Message:      decryptedMsg,
				FinishReason: tempChoice.FinishReason,
			}
		}
	} else {
		// 明文模式
		var tempRespPlaintext tempResponsePlaintext
		if err := json.Unmarshal(respBody, &tempRespPlaintext); err != nil {
			return nil, common.WrapError(err, "unmarshal plaintext response failed")
		}

		response.ID = tempRespPlaintext.ID
		response.Object = tempRespPlaintext.Object
		response.Created = tempRespPlaintext.Created
		response.SystemFingerprint = tempRespPlaintext.SystemFingerprint
		response.Model = tempRespPlaintext.Model
		response.Usage = tempRespPlaintext.Usage
		response.Choices = make([]ResponseChoice, len(tempRespPlaintext.Choices))

		// 直接使用响应中的Message对象
		for i, tempChoice := range tempRespPlaintext.Choices {
			response.Choices[i] = ResponseChoice{
				Index:        tempChoice.Index,
				Message:      tempChoice.Message,
				FinishReason: tempChoice.FinishReason,
			}
		}
	}

	return response, nil
}

// CreateChatCompletionStream 发送流式聊天请求
func (c *DeepSeekClient) CreateChatCompletionStream(request ChatCompletionRequest, customHeaders map[string]string) (<-chan StreamResponse, error) {
	if request.Messages == nil {
		return nil, common.NewAICCException("DeepSeek createChatCompletionStream messages cannot be empty")
	}

	// 检查Messages类型
	var messages []map[string]interface{}
	var ok bool
	if messages, ok = request.Messages.([]map[string]interface{}); !ok {
		return nil, common.NewAICCException("DeepSeek createChatCompletionStream messages must be a slice of map[string]interface{}")
	}

	if len(messages) == 0 {
		return nil, common.NewAICCException("DeepSeek createChatCompletionStream messages cannot be empty")
	}

	// 创建通道用于返回流式响应
	streamChan := make(chan StreamResponse)

	// 使用goroutine处理流式响应
	go func() {
		defer close(streamChan)

		// 构建URL
		url := c.getEndpoint()

		// 构建请求头
		headers := map[string]string{
			"Content-Type":  "application/json",
			"Authorization": fmt.Sprintf("Bearer %s", c.APIKey),
		}

		// 添加自定义请求头
		if customHeaders != nil {
			for k, v := range customHeaders {
				headers[k] = v
			}
		}

		// 加密数据
		var encryptResult *jeddak_secure_channel.EncryptResult
		if c.IsSecure && c.jscClient != nil {
			// 只保留messages中的role和content字段，与Java版本保持一致
			var filteredMessages []map[string]interface{}
			for _, msg := range messages {
				filteredMsg := make(map[string]interface{})
				if role, ok := msg["role"].(string); ok {
					filteredMsg["role"] = role
				}
				if content, ok := msg["content"].(string); ok {
					filteredMsg["content"] = content
				}
				if len(filteredMsg) > 0 {
					filteredMessages = append(filteredMessages, filteredMsg)
				}
			}

			// 使用手动实现的pickle序列化filteredMessages
			pickleData, err := pickleEncode(filteredMessages)
			if err != nil {
				log.Error("pickle encode messages failed: %v", err)
				return
			}

			// 使用EncryptBytesWithResponse加密pickle序列化后的字节
			encryptResult, err = c.jscClient.EncryptBytesWithResponse(pickleData)
			if err != nil {
				log.Error("encrypt messages failed: %v", err)
				return
			}

			request.Messages = encryptResult.Ciphertext
		}

		// 确保Stream为true
		request.Stream = true

		// 将请求结构体转换为map
		payload, err := structToMap(request)
		if err != nil {
			log.Error("convert request to map failed: %v", err)
			return
		}

		// 发送请求
		client := &http.Client{}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			log.Error("marshal payload failed: %v", err)
			return
		}

		req, err := http.NewRequest("POST", url, strings.NewReader(string(payloadJSON)))
		if err != nil {
			log.Error("create request failed: %v", err)
			return
		}

		// 设置请求头
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		// 添加流式请求头
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")

		resp, err := client.Do(req)
		if err != nil {
			log.Error("send request failed: %v", err)
			return
		}
		defer resp.Body.Close()

		// 检查状态码
		if resp.StatusCode != http.StatusOK {
			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Error("DeepSeek createChatCompletionStream error: status code %d", resp.StatusCode)
				return
			}
			log.Error("DeepSeek createChatCompletionStream error: %s", string(respBody))
			return
		}

		// 处理流式响应
		reader := bufio.NewReader(resp.Body)
		lineCount := 0
		for {
			lineCount++
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Error("read stream line failed: %v", err)
				return
			}

			lineStr := strings.TrimSpace(string(line))
			if lineStr == "" {
				continue
			}

			// 检查是否是数据行（支持多种格式）
			var data string
			if strings.HasPrefix(lineStr, "data:") {
				// 标准SSE格式
				data = strings.TrimPrefix(lineStr, "data:")
			} else if strings.HasPrefix(lineStr, "d0:") {
				// DeepSeek非标准格式
				data = strings.TrimPrefix(lineStr, "d0:")
			} else if strings.HasPrefix(lineStr, "eyJ") {
				// 直接以Base64编码的JSON字符串开头（eyJ是"{"的Base64编码）
				data = lineStr
			} else if strings.HasPrefix(lineStr, "{") {
				// 直接以JSON字符串开头
				data = lineStr
			} else {
				continue
			}

			if data == "[DONE]" {
				break
			}

			// 解析响应数据
			var streamResp StreamResponse
			var processData string

			if c.IsSecure && encryptResult != nil {
				// 获取model_version，默认是deepseek-v3.1
				modelVersion := "deepseek-v3.1"
				if val, ok := c.kwargs["model_version"]; ok {
					if strVal, ok := val.(string); ok {
						modelVersion = strVal
					}
				}

				// 根据model_version选择不同的处理逻辑
				if modelVersion == "deepseek-v3" {
					// v3模式：数据是Base64编码的字符串，需要先Base64解码再解密

					// 先Base64解码
					decodedBytes, err := base64.StdEncoding.DecodeString(data)
					if err != nil {
						log.Error("Failed to base64 decode v3 encrypted content: %v", err)
						continue
					}

					// 然后解密
					decryptedBytes, decryptErr := decryptContent(string(decodedBytes), encryptResult, c)
					if decryptErr != nil {
						log.Error(fmt.Sprintf("DeepSeek v3 stream decrypt chunk content failed: %v", decryptErr))
						continue
					}

					processData = string(decryptedBytes)
				} else {
					// v3.1模式：数据是标准JSON格式，包含content字段

					// 解析JSON获取content字段
					var responseMap map[string]interface{}
					if err := json.Unmarshal([]byte(data), &responseMap); err == nil {
						if content, ok := responseMap["content"].(string); ok {
							// 对content进行Base64解码，得到完整的EncryptedMessage JSON字符串
							decodedContent, decodeErr := base64.StdEncoding.DecodeString(content)
							if decodeErr != nil {
								log.Error(fmt.Sprintf("DeepSeek v3.1 mode - Base64 decode content failed: %v", decodeErr))
								continue
							}

							// 直接解密完整的EncryptedMessage JSON字符串
							decryptedBytes, decryptErr := decryptContent(string(decodedContent), encryptResult, c)
							if decryptErr != nil {
								log.Error(fmt.Sprintf("DeepSeek v3.1 stream decrypt chunk content failed: %v", decryptErr))
								continue
							}

							processData = string(decryptedBytes)
						}
					}
				}
				// 处理完成，继续下一个块
			} else {
				// 非加密模式，直接使用数据
				processData = data

			}

			// 处理数据（统一处理加密和解密后的情况）
			if processData != "" {
				// 移除可能的"data: "前缀
				processedStr := strings.TrimSpace(processData)
				if strings.HasPrefix(processedStr, "data: ") {
					processedStr = strings.TrimPrefix(processedStr, "data: ")
					processedStr = strings.TrimSpace(processedStr)
				}

				// 检查是否是结束标记
				if processedStr == "[DONE]" {
					break
				}

				// 解析JSON响应
				if err := json.Unmarshal([]byte(processedStr), &streamResp); err != nil {
					log.Error(fmt.Sprintf("unmarshal stream response failed: %v, data: %q", err, processedStr))
					continue
				}

				// 发送到通道
				streamChan <- streamResp
			}
		}
	}()

	return streamChan, nil
}

// CreateChatCompletionWithOpenAI 使用OpenAI SDK的非流式请求（已弃用）
func (c *DeepSeekClient) CreateChatCompletionWithOpenAI(ctx context.Context, params openai.ChatCompletionNewParams, options ...option.RequestOption) (*openai.ChatCompletion, error) {
	return nil, errors.New("OpenAI模式暂不支持，因为无法获取服务端的AdditionalProperties加密字段")
}

// CreateChatCompletionStreamWithOpenAI 使用OpenAI SDK的流式请求（已弃用）
func (c *DeepSeekClient) CreateChatCompletionStreamWithOpenAI(ctx context.Context, params openai.ChatCompletionNewParams, options ...option.RequestOption) (<-chan StreamResponse, error) {
	return nil, errors.New("OpenAI模式暂不支持，因为无法获取服务端的AdditionalProperties加密字段")
}

// decryptContent 解密内容
func decryptContent(encryptedContent string, encryptResult *jeddak_secure_channel.EncryptResult, c *DeepSeekClient) ([]byte, error) {
	// 直接解密密文，获取二进制数据
	decryptedBytes, err := encryptResult.ResponseKey.DecryptBytesString(encryptedContent)
	if err != nil {
		log.Error(fmt.Sprintf("DeepSeek decrypt content failed: %v", err))
		return nil, err
	}
	return decryptedBytes, nil
}

// getMapKeys 获取map的键列表
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// getEndpoint 根据model_version和IsSecure选择正确的API接口
func (c *DeepSeekClient) getEndpoint() string {
	// 如果是OpenAI模式，直接返回BaseURL，不添加任何后缀
	if c.IsOpenAI {
		return c.BaseURL
	}

	// 默认使用v1接口
	endpoint := "/v1/chat/completions"

	// 如果是安全模式，根据model_version选择接口
	if c.IsSecure {
		// 获取model_version，默认是deepseek-v3.1
		modelVersion := "deepseek-v3.1"
		if val, ok := c.kwargs["model_version"]; ok {
			if strVal, ok := val.(string); ok {
				modelVersion = strVal
			}
		}

		// 根据model_version选择接口
		switch modelVersion {
		case "deepseek-v3":
			endpoint = "/v2/chat/completions"
		default: // deepseek-v3.1等版本
			endpoint = "/v4/chat/completions"
		}
	}

	return fmt.Sprintf("%s%s", c.BaseURL, endpoint)
}

// flusher 定义刷新接口
type flusher interface {
	Flush()
}

// structToMap 将结构体转换为map[string]interface{}
func structToMap(v interface{}) (map[string]interface{}, error) {
	// 将结构体转换为JSON
	jsonData, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	// 将JSON转换为map
	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// pickleEncode 实现与Java razorvine.pickle兼容的pickle序列化
// 支持[]map[string]interface{}格式，使用协议2
func pickleEncode(messages []map[string]interface{}) ([]byte, error) {
	var buf bytes.Buffer

	// 写入pickle协议头 (协议2)
	buf.WriteString("\x80\x02")

	// 开始列表标记
	buf.WriteString("]")
	buf.WriteByte('(')

	for _, msg := range messages {
		// 开始字典标记
		buf.WriteString("}")
		buf.WriteByte('(')

		// 写入role键 (二进制字符串格式)
		buf.WriteByte('X')
		buf.WriteString("\x04\x00\x00\x00") // 长度4
		buf.WriteString("role")

		// 写入role值 (二进制字符串格式)
		role, ok := msg["role"].(string)
		if !ok {
			return nil, fmt.Errorf("pickleEncode: message missing string role: %v", msg)
		}
		buf.WriteByte('X')
		buf.WriteString(fmt.Sprintf("%c%c%c%c", byte(len(role)), byte(len(role)>>8), byte(len(role)>>16), byte(len(role)>>24)))
		buf.WriteString(role)

		// 写入content键 (二进制字符串格式)
		buf.WriteByte('X')
		buf.WriteString("\x07\x00\x00\x00") // 长度7
		buf.WriteString("content")

		// 写入content值 (二进制字符串格式)
		content, ok := msg["content"].(string)
		if !ok {
			return nil, fmt.Errorf("pickleEncode: message missing string content: %v", msg)
		}
		buf.WriteByte('X')
		buf.WriteString(fmt.Sprintf("%c%c%c%c", byte(len(content)), byte(len(content)>>8), byte(len(content)>>16), byte(len(content)>>24)))
		buf.WriteString(content)

		// 结束字典 (使用单个'u'字符)
		buf.WriteByte('u')
	}

	// 结束列表
	buf.WriteByte('e')
	buf.WriteByte('.')

	return buf.Bytes(), nil
}

// writeBinaryString 写入pickle二进制字符串格式 (协议2)
// 格式：X{4字节小端长度}{字符串内容}u
func writeBinaryString(buf *bytes.Buffer, s string) {
	// 写入二进制字符串标记
	buf.WriteByte('X')

	// 写入4字节小端长度
	length := uint32(len(s))
	buf.WriteByte(byte(length))
	buf.WriteByte(byte(length >> 8))
	buf.WriteByte(byte(length >> 16))
	buf.WriteByte(byte(length >> 24))

	// 写入字符串内容
	buf.WriteString(s)

	// 写入结束标记
	buf.WriteByte('u')
}

// pickleDecode 实现与Java razorvine.pickle兼容的pickle反序列化
// 支持解析pickle格式的[]map[string]interface{}
func pickleDecode(data []byte) ([]map[string]interface{}, error) {
	// Check if data is empty
	if len(data) == 0 {
		return nil, fmt.Errorf("empty pickle data")
	}

	// log.Info("pickleDecode: input data length %d, hex: %x", len(data), data)

	// Check if it's protocol 4 (starts with 0x80 0x04)
	if data[0] == 0x80 && data[1] == 0x04 {
		// Protocol 4: magic bytes + [FRAME + frame size] + data
		startPos := 2

		// Skip FRAME (0x95) and 8 bytes frame size if present
		if startPos < len(data) && data[startPos] == 0x95 {
			if startPos+9 <= len(data) {
				startPos += 9
				// log.Info("pickleDecode: skipped FRAME, new startPos %d", startPos)
			} else {
				// log.Info("pickleDecode: invalid frame structure, returning raw data")
				// 如果帧结构无效，尝试将整个数据作为内容返回
				return []map[string]interface{}{{"content": string(data)}}, nil
			}
		}

		// Use a stack to handle pickle operations
		var stack []interface{}
		i := startPos

		// log.Info("pickleDecode: starting protocol 4 parsing at position %d", i)

		for i < len(data) {
			// log.Info("pickleDecode: processing token 0x%x at position %d", data[i], i)
			switch data[i] {
			case 0x7d: // EMPTY_DICT
				// log.Info("pickleDecode: found EMPTY_DICT (0x7d)")
				stack = append(stack, make(map[string]interface{}))
				i++

			case 0x28: // MARK
				// log.Info("pickleDecode: found MARK (0x28)")
				stack = append(stack, "MARK")
				i++

			case 0x75: // SETITEMS
				// log.Info("pickleDecode: found SETITEMS (0x75)")
				// Pop items from stack until we find MARK
				var items []interface{}
				for len(stack) > 0 {
					item := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					if mark, ok := item.(string); ok && mark == "MARK" {
						// log.Info("pickleDecode: found matching MARK for SETITEMS")
						break
					}
					items = append(items, item)
					// log.Info("pickleDecode: popped item %v from stack", item)
				}

				// Check if we have a dictionary on top of the stack
				if len(stack) == 0 {
					log.Info("pickleDecode: no dictionary found on stack for SETITEMS")
					i++
					continue
				}
				dict, ok := stack[len(stack)-1].(map[string]interface{})
				if !ok {
					log.Info("pickleDecode: top of stack is not a dictionary for SETITEMS")
					i++
					continue
				}

				// Add key-value pairs to the dictionary (items are in reverse order)
				// log.Info("pickleDecode: adding %d items to dictionary", len(items)/2)
				for j := len(items) - 1; j > 0; j -= 2 {
					key, ok := items[j].(string)
					if !ok {
						// log.Info("pickleDecode: invalid key at position %d", j)
						continue
					}
					value := items[j-1]
					dict[key] = value
					// log.Info("pickleDecode: added key-value pair %s: %v", key, value)
				}
				i++

			case 0x8c: // SHORT_BINUNICODE
				// log.Info("pickleDecode: found SHORT_BINUNICODE (0x8c)")
				if i+1 >= len(data) {
					// log.Info("pickleDecode: SHORT_BINUNICODE incomplete, skipping")
					i++
					continue
				}
				length := int(data[i+1])
				// log.Info("pickleDecode: SHORT_BINUNICODE length %d", length)
				if i+2+length > len(data) {
					// log.Info("pickleDecode: SHORT_BINUNICODE data too short, skipping")
					i++
					continue
				}
				str := string(data[i+2 : i+2+length])
				// log.Info("pickleDecode: SHORT_BINUNICODE value '%s'", str)
				stack = append(stack, str)
				i += 2 + length

			case 0x58: // BINUNICODE
				// log.Info("pickleDecode: found BINUNICODE (0x58)")
				if i+4 >= len(data) {
					// log.Info("pickleDecode: BINUNICODE incomplete, skipping")
					i++
					continue
				}
				length := int(binary.LittleEndian.Uint32(data[i+1 : i+5]))
				// log.Info("pickleDecode: BINUNICODE length %d", length)
				if i+5+length > len(data) {
					// log.Info("pickleDecode: BINUNICODE data too short, skipping")
					i++
					continue
				}
				str := string(data[i+5 : i+5+length])
				// log.Info("pickleDecode: BINUNICODE value '%s'", str)
				stack = append(stack, str)
				i += 5 + length

			case 0x5d: // EMPTY_LIST
				// log.Info("pickleDecode: found EMPTY_LIST (0x5d)")
				stack = append(stack, []interface{}{})
				i++

			case 0x4e: // NONE
				// log.Info("pickleDecode: found NONE (0x4e)")
				stack = append(stack, nil)
				i++

			case 0x94: // MEMOIZE - skip for our purposes
				// log.Info("pickleDecode: found MEMOIZE (0x94), skipping")
				i++

			case 0x2e: // STOP
				// log.Info("pickleDecode: found STOP (0x2e), breaking")
				i++
				break

			default:
				// log.Info("pickleDecode: skipping unknown token 0x%x at position %d", data[i], i)
				// Skip other tokens
				i++
			}
		}

		// Find the dictionary in the stack
		// log.Info("pickleDecode: stack contents after parsing:")
		var resultDict map[string]interface{}
		for _, item := range stack {
			// log.Info("pickleDecode: stack[%d] = %v (type: %T)", j, item, item)
			if dict, ok := item.(map[string]interface{}); ok {
				resultDict = dict
				break
			}
		}

		if resultDict == nil {
			// log.Info("pickleDecode: no dictionary found in stack, returning raw data")
			// 如果没有找到字典，尝试将整个数据作为内容返回
			return []map[string]interface{}{{"content": string(data)}}, nil
		}

		// log.Info("pickleDecode: found dictionary with keys: %v", getKeys(resultDict))
		return []map[string]interface{}{resultDict}, nil
	}

	// Protocol 2 or older
	if len(data) < 3 {
		return nil, fmt.Errorf("invalid pickle data: too short")
	}

	// 检查协议头
	if data[0] != 0x80 || data[1] != 0x02 {
		return nil, fmt.Errorf("invalid pickle protocol: expected 2")
	}

	// 检查结束标记
	if data[len(data)-1] != '.' {
		return nil, fmt.Errorf("invalid pickle data: missing end marker")
	}

	var result []map[string]interface{}
	var i = 2 // 跳过协议头

	// 检查列表开始标记
	if data[i] != ']' {
		return nil, fmt.Errorf("unsupported pickle format: protocol %d, data format not recognized", data[1])
	}
	i++

	// 检查列表元素开始标记
	if data[i] != '(' {
		return nil, fmt.Errorf("invalid pickle data: expected list items start '('")
	}
	i++

	// 解析列表元素
	for i < len(data)-1 {
		// 检查字典开始标记
		if data[i] != '}' {
			return nil, fmt.Errorf("invalid pickle data: expected dict start '}' at position %d", i)
		}
		i++

		// 检查字典元素开始标记
		if data[i] != '(' {
			return nil, fmt.Errorf("invalid pickle data: expected dict items start '(' at position %d", i)
		}
		i++

		// 创建字典
		dict := make(map[string]interface{})

		// 解析字典元素 (key, value) 对
		for i < len(data)-1 {
			// 检查是否到达字典结束标记
			if data[i] == 'u' {
				i++
				break
			}

			// 解析key
			key, readBytes, err := readBinaryString(data[i:])
			if err != nil {
				return nil, fmt.Errorf("failed to read key: %v at position %d", err, i)
			}
			i += readBytes

			// 解析value
			value, readBytes, err := readBinaryString(data[i:])
			if err != nil {
				return nil, fmt.Errorf("failed to read value: %v at position %d", err, i)
			}
			i += readBytes

			// 存储到字典
			dict[key] = value
		}

		// 添加到结果列表
		result = append(result, dict)

		// 检查是否到达列表结束标记
		if data[i] == 'e' {
			i++
			break
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no valid data found in pickle")
	}

	return result, nil
}

// extractContentFromPickle 从pickle数据中直接提取content字段
// 基于观察到的pickle格式：\x80\x04...\x8c\x07content...
// parseEscapedString 解析包含转义序列的字符串为真实的二进制数据
// 例如将"\x80\x04\x95{"解析为[]byte{0x80, 0x04, 0x95, '{'}
func parseEscapedString(s string) ([]byte, error) {
	var result []byte

	for i := 0; i < len(s); {
		if s[i] == '\\' && i+1 < len(s) {
			// 处理转义序列
			switch s[i+1] {
			case 'x':
				// 处理\xHH格式的转义序列
				if i+3 < len(s) {
					// 读取后面的两个十六进制字符
					hexStr := s[i+2 : i+4]
					num, err := strconv.ParseUint(hexStr, 16, 8)
					if err != nil {
						return nil, err
					}
					result = append(result, byte(num))
					i += 4
				} else {
					return nil, errors.New("invalid escape sequence: \\x followed by less than 2 hex digits")
				}
			case '\\':
				// 处理\\格式的转义序列
				result = append(result, '\\')
				i += 2
			case 'n':
				// 处理\n格式的转义序列
				result = append(result, '\n')
				i += 2
			case 'r':
				// 处理\r格式的转义序列
				result = append(result, '\r')
				i += 2
			case 't':
				// 处理\t格式的转义序列
				result = append(result, '\t')
				i += 2
			case 'b':
				// 处理\b格式的转义序列
				result = append(result, '\b')
				i += 2
			case 'f':
				// 处理\f格式的转义序列
				result = append(result, '\f')
				i += 2
			case '"':
				// 处理\"格式的转义序列
				result = append(result, '"')
				i += 2
			case 'a':
				// 处理\a格式的转义序列（响铃字符，ASCII 0x07）
				result = append(result, 0x07)
				i += 2
			case 'v':
				// 处理\v格式的转义序列
				result = append(result, '\v')
				i += 2
			default:
				// 处理未知的转义序列，直接保留原字符
				result = append(result, s[i+1])
				i += 2
			}
		} else {
			// 处理普通字符
			result = append(result, s[i])
			i += 1
		}
	}

	return result, nil
}

func extractContentFromPickle(pickleData string) string {
	// 首先将字符串表示的pickle数据转换为真实的二进制数据
	binaryData, err := parseEscapedString(pickleData)
	if err != nil {
		log.Error(fmt.Sprintf("DeepSeek parse pickle data failed in extractContent: %v", err))
		return ""
	}

	// 查找content字段的二进制标记: \x8c\x07content (0x07是"content"字符串的长度)
	contentMarker := []byte{0x8c, 0x07, 'c', 'o', 'n', 't', 'e', 'n', 't'}
	idx := bytes.Index(binaryData, contentMarker)
	if idx == -1 {
		return ""
	}

	// 跳过content标记
	idx += len(contentMarker)

	// 查找content值的开始位置
	// 在content标记后，通常是\x8c(SHORT_BINUNICODE)或X(BINUNICODE)标记
	if idx >= len(binaryData) {
		return ""
	}

	// 检查值的类型标记
	switch binaryData[idx] {
	case 0x8c:
		// 短字符串格式：\x8c + 长度字节 + 字符串内容
		idx++ // 跳过\x8c

		// 读取长度字节
		if idx >= len(binaryData) {
			return ""
		}

		length := int(binaryData[idx])
		idx++ // 跳过长度字节

		// 读取字符串内容
		if idx+length > len(binaryData) {
			return ""
		}

		content := binaryData[idx : idx+length]
		return string(content)

	case 'X':
		// BINUNICODE格式：X + 4字节长度 + 字符串内容
		idx++ // 跳过X

		// 读取4字节长度（小端序）
		if idx+4 > len(binaryData) {
			return ""
		}

		length := uint32(binaryData[idx]) | uint32(binaryData[idx+1])<<8 | uint32(binaryData[idx+2])<<16 | uint32(binaryData[idx+3])<<24
		idx += 4 // 跳过长度字节

		// 读取字符串内容
		if idx+int(length) > len(binaryData) {
			return ""
		}

		contentBytes := binaryData[idx : idx+int(length)]
		return string(contentBytes)
	}

	return ""
}

// getKeys 获取map的所有键
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// readBinaryString 读取pickle二进制字符串格式 (协议2)
// 格式：X{4字节小端长度}{字符串内容}u
// 返回：字符串值，读取的字节数，错误
func readBinaryString(data []byte) (string, int, error) {
	if len(data) < 6 {
		return "", 0, fmt.Errorf("invalid binary string: too short")
	}

	// 检查标记
	if data[0] != 'X' {
		return "", 0, fmt.Errorf("invalid binary string: expected 'X' marker")
	}

	// 读取4字节小端长度
	length := uint32(data[1]) | uint32(data[2])<<8 | uint32(data[3])<<16 | uint32(data[4])<<24

	// 检查数据长度是否足够
	if int(length)+6 > len(data) {
		return "", 0, fmt.Errorf("invalid binary string: length exceeds data")
	}

	// 检查结束标记
	if data[5+length] != 'u' {
		return "", 0, fmt.Errorf("invalid binary string: missing 'u' end marker")
	}

	// 提取字符串内容
	content := string(data[5 : 5+length])

	return content, int(length) + 6, nil
}
