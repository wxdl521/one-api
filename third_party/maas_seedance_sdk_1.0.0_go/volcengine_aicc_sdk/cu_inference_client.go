package volcengine_aicc_sdk

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"maas_seedance_sdk_1.0.0_go/volcengine_aicc_sdk/common"
	"maas_seedance_sdk_1.0.0_go/volcengine_aicc_sdk/common/log"
	"maas_seedance_sdk_1.0.0_go/volcengine_aicc_sdk/jeddak_secure_channel"
)

// CUClient CU客户端
type CUClient struct {
	*BaseInference
	AICCServiceName string
	PolicyID        string
	kwargs          map[string]interface{}
	jscClient       *jeddak_secure_channel.Client
	MaxRetries      int
}

// NewCUClient 创建新的CU客户端
func NewCUClient(baseURL string, aiccServiceName string, policyID string, isOpenAI bool, isSecure bool,
	ak string, sk string, region string, kwargs map[string]interface{}) *CUClient {

	if baseURL == "" || aiccServiceName == "" || policyID == "" || ak == "" || sk == "" {
		panic(common.NewAICCException("New CU client failed, params error"))
	}

	client := &CUClient{
		BaseInference:   NewBaseInference(baseURL, isOpenAI, isSecure, ak, sk, region),
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
			panic(common.NewAICCException("New CU client failed"))
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
			panic(common.NewAICCException("New CU client failed, attest server error"))
		}
	}

	return client
}

// sendRequest 发送HTTP请求
func (c *CUClient) sendRequest(url string, body map[string]interface{}, headers map[string]string) (*http.Response, error) {
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

// Completions 实现文本补全（流式）
func (c *CUClient) CompletionsStream(prompt string, outputWriter io.Writer) error {
	if prompt == "" {
		return common.NewAICCException("vllm completions Prompt cannot be empty")
	}

	// 构建URL
	var url string
	if c.IsSecure {
		url = fmt.Sprintf("%s/v2/completions", c.BaseURL)
	} else {
		url = fmt.Sprintf("%s/v1/completions", c.BaseURL)
	}

	// 构建请求头
	headers := map[string]string{
		"Content-Type":     "application/json",
		"X-Api-Request-Id": common.GenerateUUID(),
	}

	// 设置默认值
	maxTokens := 100
	if val, ok := c.kwargs["max_tokens"]; ok {
		if floatVal, ok := val.(float64); ok {
			maxTokens = int(floatVal)
		}
	}

	temperature := 1.0
	if val, ok := c.kwargs["temperature"]; ok {
		if floatVal, ok := val.(float64); ok {
			temperature = floatVal
		}
	}

	// 加密数据
	var encryptResult *jeddak_secure_channel.EncryptResult
	encryptedPrompt := prompt
	if c.IsSecure && c.jscClient != nil {
		var err error
		encryptResult, err = c.jscClient.EncryptStringWithResponse(prompt)
		if err != nil {
			return common.WrapError(err, "encrypt prompt failed")
		}
		encryptedPrompt = encryptResult.Ciphertext
	}

	// 构建请求体
	payload := map[string]interface{}{
		"prompt":      encryptedPrompt,
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"stream":      true,
	}

	// 构建请求
	reqBody, err := json.Marshal(payload)
	if err != nil {
		return common.WrapError(err, "marshal payload failed")
	}

	// 创建HTTP客户端
	client := &http.Client{}

	// 创建请求
	req, err := http.NewRequest("POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return common.WrapError(err, "create request failed")
	}

	// 设置请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return common.WrapError(err, "send request failed")
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return common.NewAICCException(fmt.Sprintf("call vllm completions error: status code %d", resp.StatusCode))
		}
		return common.NewAICCException(fmt.Sprintf("call vllm completions error: %s", string(respBody)))
	}

	// 读取流式响应
	reader := bufio.NewReader(resp.Body)
	for {
		chunk, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if len(chunk) > 0 {
			if c.IsSecure && encryptResult != nil {
				// 解码base64
				decodedChunk, err := base64.StdEncoding.DecodeString(string(chunk))
				if err != nil {
					return err
				}
				// 解密数据
				decryptedData, err := encryptResult.ResponseKey.DecryptString(string(decodedChunk))
				if err != nil {
					return err
				}
				_, err = outputWriter.Write([]byte(decryptedData + "\n"))
				if err != nil {
					return common.WrapError(err, "write to output stream failed")
				}
			} else {
				// 直接写入输出流
				_, err = outputWriter.Write(chunk)
				if err != nil {
					return common.WrapError(err, "write to output stream failed")
				}
			}
		}
	}

	return nil
}

// Completions 实现文本补全（非流式）
func (c *CUClient) Completions(prompt string) (string, error) {
	if prompt == "" {
		return "", common.NewAICCException("vllm completions Prompt cannot be empty")
	}

	// 构建URL
	var url string
	if c.IsSecure {
		url = fmt.Sprintf("%s/v2/completions", c.BaseURL)
	} else {
		url = fmt.Sprintf("%s/v1/completions", c.BaseURL)
	}

	// 构建请求头
	headers := map[string]string{
		"Content-Type":     "application/json",
		"X-Api-Request-Id": common.GenerateUUID(),
	}

	// 设置默认值
	maxTokens := 100
	if val, ok := c.kwargs["max_tokens"]; ok {
		if floatVal, ok := val.(float64); ok {
			maxTokens = int(floatVal)
		}
	}

	temperature := 1.0
	if val, ok := c.kwargs["temperature"]; ok {
		if floatVal, ok := val.(float64); ok {
			temperature = floatVal
		}
	}

	// 加密数据
	var encryptResult *jeddak_secure_channel.EncryptResult
	encryptedPrompt := prompt
	if c.IsSecure && c.jscClient != nil {
		promptJSON, err := json.Marshal(prompt)
		if err != nil {
			return "", common.WrapError(err, "marshal prompt failed")
		}

		encryptResult, err = c.jscClient.EncryptBytesWithResponse(promptJSON)
		if err != nil {
			return "", common.WrapError(err, "encrypt prompt failed")
		}

		encryptedPrompt = encryptResult.Ciphertext
	}

	// 构建请求体
	payload := map[string]interface{}{
		"prompt":      encryptedPrompt,
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"stream":      false,
	}

	// 处理非流式响应，支持重试
	for attempt := 0; attempt < c.MaxRetries; attempt++ {
		// 发送请求
		resp, err := c.sendRequest(url, payload, headers)
		if err != nil {
			log.Error(fmt.Sprintf("call AICC VLLM service completions error: %v, try again...", err))
			if attempt == c.MaxRetries-1 {
				return "", common.NewAICCException("call vllm completions error")
			}
			time.Sleep(time.Second)
			continue
		}

		// 检查状态码
		if resp.StatusCode != http.StatusOK {
			respBody, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				log.Error(fmt.Sprintf("call AICC VLLM service completions error: status code %d, try again...", resp.StatusCode))
				if attempt == c.MaxRetries-1 {
					return "", common.NewAICCException("call vllm completions error")
				}
				time.Sleep(time.Second)
				continue
			}
			log.Error(fmt.Sprintf("call AICC VLLM service completions error: %s, try again...", string(respBody)))
			if attempt == c.MaxRetries-1 {
				return "", common.NewAICCException("call vllm completions error")
			}
			time.Sleep(time.Second)
			continue
		}

		// 解析响应
		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if err != nil {
			log.Error(fmt.Sprintf("call AICC VLLM service completions error: %v, try again...", err))
			if attempt == c.MaxRetries-1 {
				return "", common.NewAICCException("call vllm completions error")
			}
			time.Sleep(time.Second)
			continue
		}

		// 解密数据
		if c.IsSecure && encryptResult != nil {
			choices, ok := result["choices"].([]interface{})
			if !ok {
				log.Error("response format error: choices not found")
				if attempt == c.MaxRetries-1 {
					return "", common.NewAICCException("call vllm completions error")
				}
				time.Sleep(time.Second)
				continue
			}

			for _, choice := range choices {
				choiceMap, ok := choice.(map[string]interface{})
				if !ok {
					continue
				}

				text, ok := choiceMap["text"].(string)
				if !ok {
					continue
				}

				// 解密文本
				decryptedText, err := encryptResult.ResponseKey.DecryptString(text)
				if err != nil {
					log.Error(fmt.Sprintf("decrypt text failed: %v, try again...", err))
					if attempt == c.MaxRetries-1 {
						return "", common.NewAICCException("call vllm completions error")
					}
					time.Sleep(time.Second)
					continue
				}

				choiceMap["text"] = decryptedText
			}
		}

		// 转换为JSON字符串
		resultJSON, err := json.Marshal(result)
		if err != nil {
			log.Error(fmt.Sprintf("marshal result failed: %v, try again...", err))
			if attempt == c.MaxRetries-1 {
				return "", common.NewAICCException("call vllm completions error")
			}
			time.Sleep(time.Second)
			continue
		}

		return string(resultJSON), nil
	}

	return "", common.NewAICCException("call vllm completions error")
}

// ChatCompletions 实现聊天补全
func (c *CUClient) ChatCompletions(messages []map[string]interface{}, outputWriter io.Writer) (string, error) {
	if len(messages) == 0 {
		return "", common.NewAICCException("vllm chat_completions Messages cannot be empty")
	}

	// 构建URL
	var url string
	if c.IsSecure {
		url = fmt.Sprintf("%s/v2/chat/completions", c.BaseURL)
	} else {
		url = fmt.Sprintf("%s/v1/chat/completions", c.BaseURL)
	}

	// 构建请求头
	headers := map[string]string{
		"Content-Type":     "application/json",
		"X-Api-Request-Id": common.GenerateUUID(),
	}

	// 设置默认值
	maxTokens := 100
	if val, ok := c.kwargs["max_tokens"]; ok {
		if floatVal, ok := val.(float64); ok {
			maxTokens = int(floatVal)
		}
	}

	temperature := 1.0
	if val, ok := c.kwargs["temperature"]; ok {
		if floatVal, ok := val.(float64); ok {
			temperature = floatVal
		}
	}

	// 加密数据
	var encryptResult *jeddak_secure_channel.EncryptResult
	var encryptedMessages interface{} = messages
	if c.IsSecure && c.jscClient != nil {
		messagesJSON, err := json.Marshal(messages)
		if err != nil {
			return "", common.WrapError(err, "marshal messages failed")
		}

		encryptResult, err = c.jscClient.EncryptBytesWithResponse(messagesJSON)
		if err != nil {
			return "", common.WrapError(err, "encrypt messages failed")
		}

		encryptedMessages = encryptResult.Ciphertext
	}

	// 构建请求体
	var isStream bool
	if outputWriter == nil {
		isStream = false
	} else {
		isStream = true
	}
	payload := map[string]interface{}{
		"messages":    encryptedMessages,
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"stream":      isStream,
	}

	if isStream {
		// 构建请求
		reqBody, err := json.Marshal(payload)
		if err != nil {
			return "", common.WrapError(err, "marshal payload failed")
		}

		// 创建HTTP客户端
		client := &http.Client{}

		// 创建请求
		req, err := http.NewRequest("POST", url, strings.NewReader(string(reqBody)))
		if err != nil {
			return "", common.WrapError(err, "create request failed")
		}

		// 设置请求头
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		// 发送请求
		resp, err := client.Do(req)
		if err != nil {
			return "", common.WrapError(err, "send request failed")
		}
		defer resp.Body.Close()

		// 检查状态码
		if resp.StatusCode != http.StatusOK {
			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				return "", common.NewAICCException(fmt.Sprintf("call vllm chat_completions error: status code %d", resp.StatusCode))
			}
			return "", common.NewAICCException(fmt.Sprintf("call vllm chat_completions error: %s", string(respBody)))
		}

		// 读取流式响应
		reader := resp.Body
		buf := make([]byte, 4096)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				if c.IsSecure && encryptResult != nil {
					// 解密响应
					decryptedChunk, err := encryptResult.ResponseKey.DecryptString(string(chunk))
					if err != nil {
						return "", common.WrapError(err, "decrypt chunk failed")
					}
					// 写入输出流
					_, err = outputWriter.Write([]byte(decryptedChunk))
					if err != nil {
						return "", common.WrapError(err, "write to output stream failed")
					}
				} else {
					// 直接写入输出流
					_, err = outputWriter.Write(chunk)
					if err != nil {
						return "", common.WrapError(err, "write to output stream failed")
					}
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", common.WrapError(err, "read response chunk failed")
			}
		}

		return "", nil
	} else {
		// 处理非流式响应，支持重试
		for attempt := 0; attempt < c.MaxRetries; attempt++ {
			// 发送请求
			resp, err := c.sendRequest(url, payload, headers)
			if err != nil {
				log.Error(fmt.Sprintf("call AICC VLLM service chat_completions error: %v, try again...", err))
				if attempt == c.MaxRetries-1 {
					return "", common.NewAICCException("call vllm chat_completions error")
				}
				time.Sleep(time.Second)
				continue
			}

			// 检查状态码
			if resp.StatusCode != http.StatusOK {
				respBody, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					log.Error(fmt.Sprintf("call AICC VLLM service chat_completions error: status code %d, try again...", resp.StatusCode))
					if attempt == c.MaxRetries-1 {
						return "", common.NewAICCException("call vllm chat_completions error")
					}
					time.Sleep(time.Second)
					continue
				}
				log.Error(fmt.Sprintf("call AICC VLLM service chat_completions error: %s, try again...", string(respBody)))
				if attempt == c.MaxRetries-1 {
					return "", common.NewAICCException("call vllm chat_completions error")
				}
				time.Sleep(time.Second)
				continue
			}

			// 解析响应
			var result map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&result)
			resp.Body.Close()
			if err != nil {
				log.Error(fmt.Sprintf("call AICC VLLM service chat_completions error: %v, try again...", err))
				if attempt == c.MaxRetries-1 {
					return "", common.NewAICCException("call vllm chat_completions error")
				}
				time.Sleep(time.Second)
				continue
			}

			// 解密数据
			if c.IsSecure && encryptResult != nil {
				choices, ok := result["choices"].([]interface{})
				if !ok {
					log.Error("response format error: choices not found")
					if attempt == c.MaxRetries-1 {
						return "", common.NewAICCException("call vllm chat_completions error")
					}
					time.Sleep(time.Second)
					continue
				}

				for _, choice := range choices {
					choiceMap, ok := choice.(map[string]interface{})
					if !ok {
						continue
					}

					message, ok := choiceMap["message"].(string)
					if !ok {
						continue
					}

					// 解密消息
					decryptedMessage, err := encryptResult.ResponseKey.DecryptString(message)
					if err != nil {
						log.Error(fmt.Sprintf("decrypt message failed: %v, try again...", err))
						if attempt == c.MaxRetries-1 {
							return "", common.NewAICCException("call vllm chat_completions error")
						}
						time.Sleep(time.Second)
						continue
					}

					// 解析解密后的消息
					var decryptedMessageMap map[string]interface{}
					err = json.Unmarshal([]byte(decryptedMessage), &decryptedMessageMap)
					if err != nil {
						log.Error(fmt.Sprintf("unmarshal decrypted message failed: %v, try again...", err))
						if attempt == c.MaxRetries-1 {
							return "", common.NewAICCException("call vllm chat_completions error")
						}
						time.Sleep(time.Second)
						continue
					}

					choiceMap["message"] = decryptedMessageMap
				}
			}

			// 转换为JSON字符串
			resultJSON, err := json.Marshal(result)
			if err != nil {
				log.Error(fmt.Sprintf("marshal result failed: %v, try again...", err))
				if attempt == c.MaxRetries-1 {
					return "", common.NewAICCException("call vllm chat_completions error")
				}
				time.Sleep(time.Second)
				continue
			}
			return string(resultJSON), nil
		}
		return "", common.NewAICCException("call vllm chat_completions error")
	}
}

// ChatCompletionsStream 实现聊天补全（流式）
func (c *CUClient) ChatCompletionsStream(messages []map[string]interface{}, outputWriter io.Writer) error {
	if len(messages) == 0 {
		return common.NewAICCException("vllm chat_completions Messages cannot be empty")
	}

	// 构建URL
	var url string
	if c.IsSecure {
		url = fmt.Sprintf("%s/v2/chat/completions", c.BaseURL)
	} else {
		url = fmt.Sprintf("%s/v1/chat/completions", c.BaseURL)
	}

	// 构建请求头
	headers := map[string]string{
		"Content-Type":     "application/json",
		"X-Api-Request-Id": common.GenerateUUID(),
	}

	// 设置默认值
	maxTokens := 100
	if val, ok := c.kwargs["max_tokens"]; ok {
		if floatVal, ok := val.(float64); ok {
			maxTokens = int(floatVal)
		}
	}

	temperature := 1.0
	if val, ok := c.kwargs["temperature"]; ok {
		if floatVal, ok := val.(float64); ok {
			temperature = floatVal
		}
	}

	// 加密数据
	var encryptResult *jeddak_secure_channel.EncryptResult
	var encryptedMessages interface{} = messages
	if c.IsSecure && c.jscClient != nil {
		messagesJSON, err := json.Marshal(messages)
		if err != nil {
			return common.WrapError(err, "marshal messages failed")
		}

		encryptResultTmp, err := c.jscClient.EncryptBytesWithResponse(messagesJSON)
		if err != nil {
			return common.WrapError(err, "encrypt messages failed")
		}

		encryptedMessages = encryptResultTmp.Ciphertext
	}

	// 构建请求体
	payload := map[string]interface{}{
		"messages":    encryptedMessages,
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"stream":      true,
	}

	// 构建请求
	reqBody, err := json.Marshal(payload)
	if err != nil {
		return common.WrapError(err, "marshal payload failed")
	}

	// 创建HTTP客户端
	client := &http.Client{}

	// 创建请求
	req, err := http.NewRequest("POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		return common.WrapError(err, "create request failed")
	}

	// 设置请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return common.WrapError(err, "send request failed")
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return common.NewAICCException(fmt.Sprintf("call vllm chat_completions error: status code %d", resp.StatusCode))
		}
		return common.NewAICCException(fmt.Sprintf("call vllm chat_completions error: %s", string(respBody)))
	}

	// 读取流式响应
	reader := resp.Body
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if c.IsSecure && encryptResult != nil {
				// 解密响应
				decryptedChunk, err := encryptResult.ResponseKey.DecryptString(string(chunk))
				if err != nil {
					return common.WrapError(err, "decrypt chunk failed")
				}
				// 写入输出流
				_, err = outputWriter.Write([]byte(decryptedChunk + "\n"))
				if err != nil {
					return common.WrapError(err, "write to output stream failed")
				}
			} else {
				// 直接写入输出流
				_, err = outputWriter.Write(chunk)
				if err != nil {
					return common.WrapError(err, "write to output stream failed")
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return common.WrapError(err, "read response chunk failed")
		}
	}

	return nil
}

// FunctionCall 实现函数调用
func (c *CUClient) FunctionCall(messages []map[string]interface{}, tools []map[string]interface{}, outputWriter io.Writer) (string, error) {
	errorInfo := "Sorry, this version does not support function_call at this time."
	log.Error(errorInfo)
	return "", common.NewAICCException(errorInfo)
}

// ASRRecognize 实现ASR识别
func (c *CUClient) ASRRecognize(fileName string) (string, error) {
	errorInfo := "Sorry, this version does not support asr at this time."
	log.Error(errorInfo)
	return errorInfo, nil
}
