package controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/relay/channel/task/doubao"
	geminitask "github.com/QuantumNous/the-one/relay/channel/task/gemini"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/QuantumNous/the-one/relay/helper"
	openai "github.com/QuantumNous/the-one/relaykit/dto"
	"github.com/QuantumNous/the-one/relaykit/types"
	"github.com/QuantumNous/the-one/service"
	"github.com/QuantumNous/the-one/setting/system_setting"

	"github.com/gin-gonic/gin"
)

const (
	chatVideoBridgeContextKey            = "chat_video_bridge"
	chatVideoBridgeResponseSuppressedKey = "chat_video_bridge_response_suppressed"
)

var ErrChatVideoBridgeUnsupportedRequest = errors.New("chat video bridge only supports a final user text message")

type chatVideoBridgeRequest struct {
	Model  string
	Stream bool
}

func IsChatVideoBridgeModel(settings system_setting.ChatVideoBridgeSetting, model string) bool {
	model = strings.TrimSpace(model)
	if !settings.Enabled || !slices.Contains(settings.Models, model) {
		return false
	}

	// Administrators can enter their own video model IDs, including aliases
	// mapped by a channel. Keep rejecting known image-only Seedance models
	// because a chat request cannot provide their required image input.
	return !slices.Contains(doubao.ModelList, model) ||
		slices.Contains(doubao.TextToVideoModelList, model)
}

// BuildChatVideoTaskRequest converts the small, intentionally supported subset
// of an OpenAI chat request into the task request consumed by video adaptors.
func BuildChatVideoTaskRequest(request *openai.GeneralOpenAIRequest) (*relaycommon.TaskSubmitReq, error) {
	if request == nil ||
		len(request.Tools) > 0 ||
		request.ResponseFormat != nil ||
		hasChatVideoBridgeJSONValue(request.Functions) ||
		hasChatVideoBridgeJSONValue(request.FunctionCall) ||
		hasChatVideoBridgeJSONValue(request.Audio) ||
		hasChatVideoBridgeJSONValue(request.Modalities) ||
		request.Input != nil ||
		request.Prompt != nil {
		return nil, ErrChatVideoBridgeUnsupportedRequest
	}

	var prompt string
	var images []string
	for _, message := range request.Messages {
		if hasChatVideoBridgeJSONValue(message.ToolCalls) || message.ToolCallId != "" {
			return nil, ErrChatVideoBridgeUnsupportedRequest
		}
		if message.Role != "user" {
			continue
		}
		switch content := message.Content.(type) {
		case string:
			prompt = content
			images = nil
		default:
			parts := message.ParseContent()
			if len(parts) == 0 {
				return nil, ErrChatVideoBridgeUnsupportedRequest
			}
			var textParts []string
			var imageURL string
			for _, part := range parts {
				switch part.Type {
				case openai.ContentTypeText:
					if strings.TrimSpace(part.Text) != "" {
						textParts = append(textParts, strings.TrimSpace(part.Text))
					}
				case openai.ContentTypeImageURL:
					image := part.GetImageMedia()
					if image == nil || strings.TrimSpace(image.Url) == "" || imageURL != "" {
						return nil, ErrChatVideoBridgeUnsupportedRequest
					}
					imageURL = strings.TrimSpace(image.Url)
				default:
					return nil, ErrChatVideoBridgeUnsupportedRequest
				}
			}
			if len(textParts) != 1 {
				return nil, ErrChatVideoBridgeUnsupportedRequest
			}
			if imageURL != "" && (!slices.Contains(geminitask.ImageToVideoModelList, request.Model) || geminitask.ParseImageInput(imageURL) == nil) {
				return nil, ErrChatVideoBridgeUnsupportedRequest
			}
			prompt = textParts[0]
			if imageURL == "" {
				images = nil
			} else {
				images = []string{imageURL}
			}
		}
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, ErrChatVideoBridgeUnsupportedRequest
	}

	return &relaycommon.TaskSubmitReq{
		Model:  request.Model,
		Prompt: prompt,
		Images: images,
	}, nil
}

func hasChatVideoBridgeJSONValue(raw []byte) bool {
	value := strings.TrimSpace(string(raw))
	return value != "" && value != "null"
}

// PrepareChatVideoBridge recognizes opted-in Seedance chat calls before the
// normal chat distributor runs. Eligible requests are replaced with the
// canonical task payload so existing authorization and channel selection run
// against the video endpoint semantics.
func PrepareChatVideoBridge() gin.HandlerFunc {
	return func(c *gin.Context) {
		settings := system_setting.GetChatVideoBridgeSetting()
		if !settings.Enabled {
			return
		}

		request := &openai.GeneralOpenAIRequest{}
		if err := common.UnmarshalBodyReusable(c, request); err != nil {
			return
		}
		if !IsChatVideoBridgeModel(settings, request.Model) {
			return
		}

		taskRequest, err := BuildChatVideoTaskRequest(request)
		if err != nil {
			respondChatVideoBridgeError(c, http.StatusBadRequest, err.Error())
			c.Abort()
			return
		}
		body, err := common.Marshal(taskRequest)
		if err != nil {
			respondChatVideoBridgeError(c, http.StatusInternalServerError, "Failed to prepare video task")
			c.Abort()
			return
		}

		if storage, err := common.GetBodyStorage(c); err == nil {
			_ = storage.Close()
		}
		c.Set(common.KeyBodyStorage, nil)
		c.Set(common.KeyRequestBody, body)
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Request.ContentLength = int64(len(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Request.URL.Path = "/v1/video/generations"
		c.Request.URL.RawPath = ""
		c.Request.RequestURI = c.Request.URL.RequestURI()
		c.Set(chatVideoBridgeContextKey, chatVideoBridgeRequest{
			Model:  request.Model,
			Stream: request.Stream != nil && *request.Stream,
		})
	}
}

func RelayChatCompletions(c *gin.Context) {
	value, ok := c.Get(chatVideoBridgeContextKey)
	if !ok {
		Relay(c, types.RelayFormatOpenAI)
		return
	}
	state, ok := value.(chatVideoBridgeRequest)
	if !ok {
		respondChatVideoBridgeError(c, http.StatusInternalServerError, "Failed to prepare video task")
		return
	}
	relayChatVideo(c, state)
}

func relayChatVideo(c *gin.Context, state chatVideoBridgeRequest) {
	c.Set(chatVideoBridgeResponseSuppressedKey, true)
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		respondChatVideoBridgeError(c, http.StatusInternalServerError, "Failed to prepare video task")
		return
	}
	task, taskErr := SubmitTask(c, relayInfo)
	if taskErr != nil {
		respondChatVideoBridgeError(c, taskErr.StatusCode, taskErr.Message)
		return
	}
	if task == nil {
		respondChatVideoBridgeError(c, http.StatusInternalServerError, "Failed to persist video task")
		return
	}

	settings := system_setting.GetChatVideoBridgeSetting()
	ticket, err := service.IssueChatVideoTaskTicket(task.TaskID, task.UserId, time.Duration(settings.TaskPageTTLSeconds)*time.Second)
	if err != nil {
		respondChatVideoBridgeError(c, http.StatusInternalServerError, "Failed to create video task link")
		return
	}

	if state.Stream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
		c.Writer.Flush()
	}

	task = waitForChatVideoTask(c, task, time.Duration(settings.MaxWaitSeconds)*time.Second, state.Stream)
	if task.Status == model.TaskStatusFailure {
		respondChatVideoBridgeError(c, http.StatusBadGateway, "Video generation failed")
		return
	}

	pageURL := chatVideoTaskPageURL(task.TaskID, ticket)
	content := fmt.Sprintf("Video is still generating. [View task progress](%s)", pageURL)
	if task.Status == model.TaskStatusSuccess {
		contentURL := chatVideoTaskContentURL(task.TaskID, ticket)
		content = fmt.Sprintf("Video is ready. [Play or download video](%s)\n\n[View task details](%s)", contentURL, pageURL)
	}
	writeChatVideoCompletion(c, state, content)
}

func waitForChatVideoTask(c *gin.Context, task *model.Task, maxWait time.Duration, stream bool) *model.Task {
	if maxWait <= 0 {
		return task
	}
	deadline := time.NewTimer(maxWait)
	defer deadline.Stop()
	pollTicker := time.NewTicker(2 * time.Second)
	defer pollTicker.Stop()
	var heartbeat <-chan time.Time
	var heartbeatTicker *time.Ticker
	if stream {
		heartbeatTicker = time.NewTicker(15 * time.Second)
		defer heartbeatTicker.Stop()
		heartbeat = heartbeatTicker.C
	}

	for {
		select {
		case <-c.Request.Context().Done():
			return task
		case <-deadline.C:
			return task
		case <-pollTicker.C:
			updatedTask, exists, err := model.GetByTaskId(task.UserId, task.TaskID)
			if err == nil && exists && updatedTask != nil {
				task = updatedTask
				if task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure {
					return task
				}
			}
		case <-heartbeat:
			_, _ = fmt.Fprint(c.Writer, ": keepalive\n\n")
			c.Writer.Flush()
		}
	}
}

func writeChatVideoCompletion(c *gin.Context, state chatVideoBridgeRequest, content string) {
	createdAt := time.Now().Unix()
	id := helper.GetResponseID(c)
	if state.Stream {
		chunk := openai.ChatCompletionsStreamResponse{
			Id:      id,
			Object:  "chat.completion.chunk",
			Created: createdAt,
			Model:   state.Model,
			Choices: []openai.ChatCompletionsStreamResponseChoice{{
				Index: 0,
				Delta: openai.ChatCompletionsStreamResponseChoiceDelta{
					Role:    "assistant",
					Content: common.GetPointer(content),
				},
			}},
		}
		writeChatVideoSSE(c, chunk)
		writeChatVideoSSE(c, helper.GenerateStopResponse(id, createdAt, state.Model, "stop"))
		_, _ = fmt.Fprint(c.Writer, "data: [DONE]\n\n")
		c.Writer.Flush()
		return
	}

	c.JSON(http.StatusOK, openai.OpenAITextResponse{
		Id:      id,
		Object:  "chat.completion",
		Created: createdAt,
		Model:   state.Model,
		Choices: []openai.OpenAITextResponseChoice{{
			Index:        0,
			Message:      openai.Message{Role: "assistant", Content: content},
			FinishReason: "stop",
		}},
	})
}

func writeChatVideoSSE(c *gin.Context, payload any) {
	data, err := common.Marshal(payload)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	c.Writer.Flush()
}

func respondChatVideoBridgeError(c *gin.Context, statusCode int, message string) {
	if statusCode < http.StatusBadRequest {
		statusCode = http.StatusBadGateway
	}
	if c.GetBool(chatVideoBridgeResponseSuppressedKey) && c.Writer.Written() {
		writeChatVideoSSE(c, gin.H{"error": gin.H{"message": message, "type": "server_error"}})
		_, _ = fmt.Fprint(c.Writer, "data: [DONE]\n\n")
		c.Writer.Flush()
		return
	}
	c.JSON(statusCode, gin.H{"error": gin.H{"message": message, "type": "invalid_request_error"}})
}

func chatVideoTaskBaseURL() string {
	return strings.TrimRight(system_setting.ServerAddress, "/")
}

func chatVideoTaskPageURL(taskID string, ticket string) string {
	return chatVideoTaskBaseURL() + "/chat-video/tasks/" + url.PathEscape(taskID) + "?ticket=" + url.QueryEscape(ticket)
}

func chatVideoTaskContentURL(taskID string, ticket string) string {
	return chatVideoTaskBaseURL() + "/api/chat-video/tasks/" + url.PathEscape(taskID) + "/content?ticket=" + url.QueryEscape(ticket)
}
