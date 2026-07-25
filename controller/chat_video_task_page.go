package controller

import (
	"net/http"

	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/service"

	"github.com/gin-gonic/gin"
)

const chatVideoPublicContentContextKey = "chat_video_public_content"

type chatVideoTaskStatusResponse struct {
	TaskID     string `json:"task_id"`
	Status     string `json:"status"`
	Progress   string `json:"progress"`
	FailReason string `json:"fail_reason,omitempty"`
	ContentURL string `json:"content_url,omitempty"`
}

func GetChatVideoTaskStatus(c *gin.Context) {
	task, ticket := getChatVideoTicketTask(c)
	if task == nil {
		return
	}

	response := chatVideoTaskStatusResponse{
		TaskID:   task.TaskID,
		Status:   string(task.Status),
		Progress: task.Progress,
	}
	if task.Status == model.TaskStatusFailure {
		response.FailReason = sanitizeChatVideoTaskFailure(task.FailReason)
	}
	if task.Status == model.TaskStatusSuccess {
		response.ContentURL = chatVideoTaskContentURL(task.TaskID, ticket)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": response})
}

func ChatVideoTaskContent(c *gin.Context) {
	task, _ := getChatVideoTicketTask(c)
	if task == nil {
		return
	}

	// VideoProxy performs the established SSRF-protected upstream fetch. It
	// resolves task ownership again using this verified ticket identity.
	c.Set("id", task.UserId)
	c.Set(chatVideoPublicContentContextKey, true)
	VideoProxy(c)
}

func getChatVideoTicketTask(c *gin.Context) (*model.Task, string) {
	taskID := c.Param("task_id")
	ticket := c.Query("ticket")
	claims, err := service.VerifyChatVideoTaskTicket(ticket, taskID)
	if err != nil {
		chatVideoTaskNotFound(c)
		return nil, ""
	}
	task, exists, err := model.GetByTaskId(claims.UserID, taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to load task"})
		return nil, ""
	}
	if !exists || task == nil {
		chatVideoTaskNotFound(c)
		return nil, ""
	}
	return task, ticket
}

func chatVideoTaskNotFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Task not found"})
}

func sanitizeChatVideoTaskFailure(_ string) string {
	// Task failures can contain an upstream URL, provider error payload, or a
	// channel-specific diagnostic. The public task page must never expose
	// those implementation details, even when the link holder is the owner.
	return "Video generation failed"
}
