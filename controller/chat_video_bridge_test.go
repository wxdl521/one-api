package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/the-one/common"
	openai "github.com/QuantumNous/the-one/relaykit/dto"
	"github.com/QuantumNous/the-one/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildChatVideoTaskRequestUsesLastUserText(t *testing.T) {
	var request openai.GeneralOpenAIRequest
	err := common.Unmarshal([]byte(`{
		"model":"doubao-seedance-2-0-260128",
		"messages":[
			{"role":"system","content":"ignored"},
			{"role":"user","content":"first idea"},
			{"role":"assistant","content":"acknowledged"},
			{"role":"user","content":"final video prompt"}
		]
	}`), &request)
	require.NoError(t, err)

	taskRequest, err := BuildChatVideoTaskRequest(&request)

	require.NoError(t, err)
	assert.Equal(t, "doubao-seedance-2-0-260128", taskRequest.Model)
	assert.Equal(t, "final video prompt", taskRequest.Prompt)
}

func TestIsChatVideoBridgeModelRequiresEnabledAllowListedSeedanceModel(t *testing.T) {
	settings := system_setting.ChatVideoBridgeSetting{
		Enabled: true,
		Models:  []string{"doubao-seedance-2-0-260128"},
	}

	assert.True(t, IsChatVideoBridgeModel(settings, "doubao-seedance-2-0-260128"))
	assert.False(t, IsChatVideoBridgeModel(settings, "doubao-seedance-2-0-fast-260128"))
	assert.True(t, IsChatVideoBridgeModel(system_setting.ChatVideoBridgeSetting{
		Enabled: true,
		Models:  []string{"doubao-seedance-2.0"},
	}, "doubao-seedance-2.0"))
	assert.False(t, IsChatVideoBridgeModel(system_setting.ChatVideoBridgeSetting{
		Enabled: true,
		Models:  []string{"doubao-seedance-1-0-lite-i2v"},
	}, "doubao-seedance-1-0-lite-i2v"))
	assert.False(t, IsChatVideoBridgeModel(settings, "gpt-4o"))
	assert.False(t, IsChatVideoBridgeModel(system_setting.ChatVideoBridgeSetting{}, "doubao-seedance-2-0-260128"))
}

func TestIsChatVideoBridgeModelSupportsAllowListedVeoModel(t *testing.T) {
	settings := system_setting.ChatVideoBridgeSetting{
		Enabled: true,
		Models:  []string{"veo-3.1-generate-preview"},
	}

	assert.True(t, IsChatVideoBridgeModel(settings, "veo-3.1-generate-preview"))
}

func TestBuildChatVideoTaskRequestAcceptsOneFinalUserImage(t *testing.T) {
	var request openai.GeneralOpenAIRequest
	err := common.Unmarshal([]byte(`{
		"model":"veo-3.1-generate-preview",
		"messages":[{
			"role":"user",
			"content":[
				{"type":"text","text":"Animate this photo"},
				{"type":"image_url","image_url":{"url":"data:image/png;base64,aGVsbG8="}}
			]
		}]
	}`), &request)
	require.NoError(t, err)

	taskRequest, err := BuildChatVideoTaskRequest(&request)

	require.NoError(t, err)
	assert.Equal(t, "Animate this photo", taskRequest.Prompt)
	assert.Equal(t, []string{"data:image/png;base64,aGVsbG8="}, taskRequest.Images)
}

func TestBuildChatVideoTaskRequestRejectsUnsupportedChatFeatures(t *testing.T) {
	tests := []string{
		`{"model":"doubao-seedance-2-0-260128","tools":[{"type":"function"}],"messages":[{"role":"user","content":"video"}]}`,
		`{"model":"doubao-seedance-2-0-260128","response_format":{"type":"json_object"},"messages":[{"role":"user","content":"video"}]}`,
		`{"model":"doubao-seedance-2-0-260128","audio":{"format":"mp3"},"messages":[{"role":"user","content":"video"}]}`,
		`{"model":"doubao-seedance-2-0-260128","messages":[{"role":"user","content":"video"},{"role":"assistant","tool_calls":[{"id":"call_1"}]}]}`,
		`{"model":"doubao-seedance-2-0-260128","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.com/a.png"}}]}]}`,
		`{"model":"doubao-seedance-2-0-260128","messages":[{"role":"user","content":"   "}]}`,
	}

	for _, raw := range tests {
		var request openai.GeneralOpenAIRequest
		require.NoError(t, common.Unmarshal([]byte(raw), &request))

		_, err := BuildChatVideoTaskRequest(&request)

		assert.Error(t, err)
	}
}

func TestWriteChatVideoCompletionStreamsSingleResultAndDone(t *testing.T) {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)

	writeChatVideoCompletion(context, chatVideoBridgeRequest{
		Model:  "doubao-seedance-2-0-260128",
		Stream: true,
	}, "[View task progress](https://example.com/task)")

	body := response.Body.String()
	assert.Contains(t, body, `"content":"[View task progress](https://example.com/task)"`)
	assert.Contains(t, body, `"finish_reason":"stop"`)
	assert.True(t, strings.HasSuffix(body, "data: [DONE]\n\n"))
}

func TestWriteChatVideoCompletionReturnsOpenAIChatResponse(t *testing.T) {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)

	writeChatVideoCompletion(context, chatVideoBridgeRequest{
		Model: "doubao-seedance-2-0-260128",
	}, "Video is ready")

	assert.Equal(t, http.StatusOK, response.Code)
	var payload openai.OpenAITextResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.Equal(t, "chat.completion", payload.Object)
	require.Len(t, payload.Choices, 1)
	assert.Equal(t, "Video is ready", payload.Choices[0].Message.Content)
}
