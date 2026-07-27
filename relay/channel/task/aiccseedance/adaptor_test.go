package aiccseedance

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/QuantumNous/the-one/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSeedanceClient struct {
	createData map[string]any
	taskID     string
	task       map[string]any
	createErr  error
	queryErr   error
}

func (c *fakeSeedanceClient) CreateVideoGenerationTask(data map[string]any) (string, error) {
	c.createData = data
	return c.taskID, c.createErr
}

func (c *fakeSeedanceClient) QueryVideoGenerationTask(string) (map[string]any, error) {
	return c.task, c.queryErr
}

func TestTaskAdaptorSubmitsTaskThroughSeedanceClient(t *testing.T) {
	client := &fakeSeedanceClient{taskID: "upstream-task"}
	previousFactory := newSeedanceClient
	newSeedanceClient = func(baseURL, apiKey, model string) (seedanceClient, error) {
		assert.Equal(t, "https://zhenze-huhehaote.cmecloud.cn/api/v3", baseURL)
		assert.Equal(t, "api-key", apiKey)
		assert.Equal(t, "doubao-seedance-2.0", model)
		return client, nil
	}
	t.Cleanup(func() { newSeedanceClient = previousFactory })

	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelBaseUrl:    "https://zhenze-huhehaote.cmecloud.cn/api/v3",
		ApiKey:            "api-key",
		UpstreamModelName: "doubao-seedance-2.0",
	}}
	adaptor.Init(info)

	response, err := adaptor.DoRequest(nil, info, bytes.NewBufferString(`{"model":"doubao-seedance-2.0","content":[{"type":"text","text":"hello"}]}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "doubao-seedance-2.0", client.createData["model"])

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	var result map[string]string
	require.NoError(t, common.Unmarshal(body, &result))
	assert.Equal(t, "upstream-task", result["id"])
}

func TestTaskAdaptorReturnsSeedanceSubmissionError(t *testing.T) {
	client := &fakeSeedanceClient{createErr: errors.New("upstream rejected task")}
	previousFactory := newSeedanceClient
	newSeedanceClient = func(string, string, string) (seedanceClient, error) {
		return client, nil
	}
	t.Cleanup(func() { newSeedanceClient = previousFactory })

	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelBaseUrl:    "https://zhenze-huhehaote.cmecloud.cn/api/v3",
		ApiKey:            "api-key",
		UpstreamModelName: ModelName,
	}}
	adaptor.Init(info)

	response, err := adaptor.DoRequest(nil, info, bytes.NewBufferString(`{"model":"doubao-seedance-2.0"}`))

	require.ErrorIs(t, err, client.createErr)
	assert.Nil(t, response)
}

func TestTaskAdaptorBuildsSeedanceVideoPayload(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("task_request", relaycommon.TaskSubmitReq{
		Model:   ModelName,
		Prompt:  "a lighthouse in a storm",
		Images:  []string{"https://example.com/input.png"},
		Seconds: "8",
		Metadata: map[string]interface{}{
			"ratio":          "16:9",
			"generate_audio": true,
			"content": []interface{}{
				map[string]interface{}{"type": "video_url", "video_url": map[string]interface{}{"url": "https://example.com/input.mp4"}},
				map[string]interface{}{"type": "audio_url", "audio_url": map[string]interface{}{"url": "https://example.com/input.mp3"}},
			},
		},
	})

	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		IsModelMapped:     true,
		UpstreamModelName: ModelName,
	}}
	reader, err := adaptor.BuildRequestBody(context, info)

	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload requestPayload
	require.NoError(t, common.Unmarshal(body, &payload))
	assert.Equal(t, ModelName, payload.Model)
	assert.Equal(t, "16:9", payload.Ratio)
	require.NotNil(t, payload.GenerateAudio)
	assert.True(t, bool(*payload.GenerateAudio))
	require.NotNil(t, payload.Duration)
	assert.Equal(t, dto.IntValue(8), *payload.Duration)
	require.Len(t, payload.Content, 4)
	assert.Equal(t, "image_url", payload.Content[0].Type)
	assert.Equal(t, "https://example.com/input.png", payload.Content[0].ImageURL.URL)
	assert.Equal(t, "video_url", payload.Content[1].Type)
	assert.Equal(t, "audio_url", payload.Content[2].Type)
	assert.Equal(t, "text", payload.Content[3].Type)
	assert.Equal(t, "a lighthouse in a storm", payload.Content[3].Text)
}

func TestTaskAdaptorFetchesTaskThroughSeedanceClient(t *testing.T) {
	client := &fakeSeedanceClient{task: map[string]any{
		"id":      "upstream-task",
		"status":  "succeeded",
		"content": map[string]any{"video_url": "https://video.example/result.mp4"},
	}}
	previousFactory := newSeedanceClient
	newSeedanceClient = func(baseURL, apiKey, model string) (seedanceClient, error) {
		assert.Equal(t, "https://zhenze-huhehaote.cmecloud.cn/api/v3", baseURL)
		assert.Equal(t, "api-key", apiKey)
		assert.Equal(t, "doubao-seedance-2.0", model)
		return client, nil
	}
	t.Cleanup(func() { newSeedanceClient = previousFactory })

	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		UpstreamModelName: "doubao-seedance-2.0",
	}})

	response, err := adaptor.FetchTask("https://zhenze-huhehaote.cmecloud.cn/api/v3", "api-key", map[string]any{
		"task_id": "upstream-task",
		"model":   "doubao-seedance-2.0",
	}, "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	var result responseTask
	require.NoError(t, common.Unmarshal(body, &result))
	assert.Equal(t, "succeeded", result.Status)
	assert.Equal(t, "https://video.example/result.mp4", result.Content.VideoURL)
}

func TestTaskAdaptorParsesSeedanceTaskResult(t *testing.T) {
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResult([]byte(`{"status":"succeeded","content":{"video_url":"https://video.example/result.mp4"}}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, result.Status)
	assert.Equal(t, "100%", result.Progress)
	assert.Equal(t, "https://video.example/result.mp4", result.Url)
}
