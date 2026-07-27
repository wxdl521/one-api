package volcengine_aicc_sdk

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"maas_seedance_sdk_1.0.0_go/volcengine_aicc_sdk/common"
	"maas_seedance_sdk_1.0.0_go/volcengine_aicc_sdk/common/log"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

// ARKClient ARK客户端
type ARKClient struct {
	*BaseInference
	APIKey       string
	EP           string
	ThinkingType string
	kwargs       map[string]interface{}
	ark_client   *arkruntime.Client
	openaiClient openai.Client
}

// NewARKClient 创建新的ARK客户端
func NewARKClient(baseURL string, apiKey string, ep string, isOpenAI bool, isSecure bool,
	ak string, sk string, region string, kwargs map[string]interface{}) *ARKClient {

	if baseURL == "" || apiKey == "" || ep == "" {
		panic(common.NewAICCException("New ARK client failed, params error"))
	}

	// 设置环境变量
	os.Setenv("VOLC_ARK_ENCRYPTION", "AICC")

	// 创建Ark SDK客户端
	arkClient := arkruntime.NewClientWithApiKey(apiKey, arkruntime.WithBaseUrl(baseURL))

	client := &ARKClient{
		BaseInference: NewBaseInference(baseURL, isOpenAI, isSecure, ak, sk, region),
		APIKey:        apiKey,
		EP:            ep,
		ThinkingType:  "disabled",
		kwargs:        kwargs,
		ark_client:    arkClient,
	}

	// 如果是OpenAI模式，创建OpenAI客户端
	if isOpenAI {
		client.openaiClient = openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL(baseURL),
		)
	}

	return client
}

// CreateChatCompletion 实现非流式聊天补全
func (c *ARKClient) CreateChatCompletion(ctx context.Context, req model.CreateChatCompletionRequest, customHeaders map[string]string) (interface{}, error) {
	// 调用底层arkruntime客户端的CreateChatCompletion方法
	resp, err := c.ark_client.CreateChatCompletion(ctx, req, arkruntime.WithCustomHeaders(customHeaders))
	if err != nil {
		log.Error(fmt.Sprintf("ARK CreateChatCompletion ERROR: %v", err))
		return model.ChatCompletionResponse{}, common.NewAICCException(fmt.Sprintf("ARK CreateChatCompletion ERROR: %v", err))
	}
	return resp, nil
}

// CreateChatCompletionWithOpenAI
func (c *ARKClient) CreateChatCompletionWithOpenAI(ctx context.Context, params openai.ChatCompletionNewParams, options ...option.RequestOption) (*openai.ChatCompletion, error) {
	// 确保使用正确的模型ID
	params.Model = c.EP

	// 发送请求
	result, err := c.openaiClient.Chat.Completions.New(ctx, params, options...)
	if err != nil {
		// 处理错误
		log.Error(fmt.Sprintf("OpenAI CreateChatCompletion ERROR: %v", err))
		return nil, common.NewAICCException(fmt.Sprintf("OpenAI CreateChatCompletion ERROR: %v", err))
	}

	// 直接返回OpenAI原始响应
	return result, nil
}

// CreateChatCompletionStream 实现流式聊天补全
func (c *ARKClient) CreateChatCompletionStream(ctx context.Context, req model.CreateChatCompletionRequest, customHeaders map[string]string) (interface{}, error) {
	// 确保customHeaders不为nil
	if customHeaders == nil {
		customHeaders = make(map[string]string)
	}

	// 根据客户端配置设置加密请求头（只有流式支持密文）
	customHeaders["x-is-encrypted"] = fmt.Sprintf("%v", c.IsSecure)

	// 调用底层arkruntime客户端的CreateChatCompletionStream方法
	stream, err := c.ark_client.CreateChatCompletionStream(ctx, req, arkruntime.WithCustomHeaders(customHeaders))
	if err != nil {
		log.Error(fmt.Sprintf("ARK CreateChatCompletionStream ERROR: %v", err))
		return nil, common.NewAICCException(fmt.Sprintf("ARK CreateChatCompletionStream ERROR: %v", err))
	}
	return stream, nil
}

// CreateChatCompletionStreamWithOpenAI 使用OpenAI SDK的流式请求
func (c *ARKClient) CreateChatCompletionStreamWithOpenAI(ctx context.Context, params openai.ChatCompletionNewParams, options ...option.RequestOption) (interface{}, error) {
	// 确保使用正确的模型ID
	params.Model = c.EP

	// 发送请求
	stream := c.openaiClient.Chat.Completions.NewStreaming(ctx, params, options...)

	// 直接返回OpenAI原始流
	return stream, nil
}

// FunctionCall 实现函数调用
func (c *ARKClient) FunctionCall(messages []map[string]interface{}, tools []map[string]interface{}, outputWriter io.Writer) (string, error) {
	if len(messages) == 0 || len(tools) == 0 {
		return "", common.NewAICCException("ARK function_call params cannot be empty")
	}

	// 构建消息列表
	chatMessages := make([]*model.ChatCompletionMessage, 0, len(messages))
	for _, msg := range messages {
		role := msg["role"].(string)
		content := msg["content"].(string)

		var chatMessageRole string
		switch role {
		case "system":
			chatMessageRole = "system"
		case "user":
			chatMessageRole = "user"
		case "assistant":
			chatMessageRole = "assistant"
		default:
			chatMessageRole = "user"
		}

		chatMessage := &model.ChatCompletionMessage{
			Role: chatMessageRole,
			Content: &model.ChatCompletionMessageContent{
				StringValue: volcengine.String(content),
			},
		}
		chatMessages = append(chatMessages, chatMessage)
	}

	// 构建请求
	var isStream bool
	if outputWriter == nil {
		isStream = false
	} else {
		isStream = true
	}
	maxCompletionTokens := 1000
	req := model.CreateChatCompletionRequest{
		Model:    c.EP,
		Messages: chatMessages,
		Stream:   volcengine.Bool(isStream),
		StreamOptions: &model.StreamOptions{
			IncludeUsage: true,
		},
		MaxCompletionTokens: &maxCompletionTokens,
		Thinking: &model.Thinking{
			Type: model.ThinkingTypeDisabled,
		},
	}

	// 设置自定义请求头
	customHeaders := map[string]string{
		"x-is-encrypted":         fmt.Sprintf("%v", c.IsSecure),
		"x-ark-moderation-scene": "aicc-skip",
	}

	ctx := context.Background()

	if isStream {
		// 流式响应
		stream, err := c.ark_client.CreateChatCompletionStream(ctx, req, arkruntime.WithCustomHeaders(customHeaders))
		if err != nil {
			log.Error(fmt.Sprintf("ARK FunctionCallStream ERROR: %v", err))
			return "", common.NewAICCException("ARK FunctionCallStream ERROR")
		}
		defer stream.Close()

		// 处理流式响应
		var result string
		for {
			recv, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Error(fmt.Sprintf("ARK FunctionCallStream ERROR: %v", err))
				return "", common.NewAICCException("ARK FunctionCallStream ERROR")
			}

			if len(recv.Choices) > 0 {
				result += recv.Choices[0].Delta.Content
			}
		}
		outputWriter.Write([]byte(result))
		return "", nil
	} else {
		// 非流式响应
		resp, err := c.ark_client.CreateChatCompletion(ctx, req, arkruntime.WithCustomHeaders(customHeaders))
		if err != nil {
			log.Error(fmt.Sprintf("ARK FunctionCall ERROR: %v", err))
			return "", common.NewAICCException("ARK FunctionCall ERROR")
		}

		if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != nil && resp.Choices[0].Message.Content.StringValue != nil {
			return *resp.Choices[0].Message.Content.StringValue, nil
		}

		return "", common.NewAICCException("ARK FunctionCall ERROR: no content")
	}
}

// ASRRecognize 实现ASR识别
func (c *ARKClient) ASRRecognize(fileName string) (string, error) {
	errorInfo := "Sorry, this version does not support asr at this time."
	log.Error(errorInfo)
	return errorInfo, nil
}

// ImageToDataURL 将图片转换为 Data URL
func (c *ARKClient) ImageToDataURL(imagePath string) (string, error) {
	// 读取图片文件
	data, err := os.ReadFile(imagePath)
	if err != nil {
		log.Error(fmt.Sprintf("Read image file error: %v", err))
		return "", common.NewAICCException(fmt.Sprintf("Read image file error: %v", err))
	}

	// 编码为 base64
	base64Str := base64.StdEncoding.EncodeToString(data)

	// 构建 Data URL
	return "data:image/jpeg;base64," + base64Str, nil
}
