package volcengine_aicc_sdk

import (
	"fmt"
	"io"
)

// AICCInference 定义AICC推理接口
type AICCInference interface {
	// ChatCompletions 生成聊天补全
	ChatCompletions(messages []map[string]interface{}, outputWriter io.Writer) (string, error)

	// FunctionCall 函数调用
	FunctionCall(messages []map[string]interface{}, tools []map[string]interface{}, outputWriter io.Writer) (string, error)

	// ASRRecognize ASR转文字
	ASRRecognize(fileName string) (string, error)

	// Hello 测试方法
	Hello() string
}

// BaseInference 基础推理实现
type BaseInference struct {
	BaseURL  string
	IsOpenAI bool
	IsSecure bool
	AK       string
	SK       string
	Region   string
}

// NewBaseInference 创建基础推理实例
func NewBaseInference(baseURL string, isOpenAI bool, isSecure bool, ak string, sk string, region string) *BaseInference {
	if region == "" {
		region = "cn-beijing"
	}
	return &BaseInference{
		BaseURL:  baseURL,
		IsOpenAI: isOpenAI,
		IsSecure: isSecure,
		AK:       ak,
		SK:       sk,
		Region:   region,
	}
}

// Hello 实现Hello方法
func (b *BaseInference) Hello() string {
	info := "hello aicc inference"
	fmt.Println(info)
	return info
}

// ChatCompletions 基础实现，需要子类覆盖
func (b *BaseInference) ChatCompletions(messages []map[string]interface{}, outputWriter io.Writer) (string, error) {
	return "", nil
}

// FunctionCall 基础实现，需要子类覆盖
func (b *BaseInference) FunctionCall(messages []map[string]interface{}, tools []map[string]interface{}, outputWriter io.Writer) (string, error) {
	return "", nil
}

// ASRRecognize 基础实现，需要子类覆盖
func (b *BaseInference) ASRRecognize(fileName string) (string, error) {
	return "", nil
}
