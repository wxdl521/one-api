package controller

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type agentMCPContextKey struct{}

var agentMCPHTTPHandler = mcp.NewStreamableHTTPHandler(
	func(request *http.Request) *mcp.Server {
		tokenID, ok := request.Context().Value(agentMCPContextKey{}).(int)
		if !ok || tokenID <= 0 {
			return nil
		}
		return newAgentMCPServer(tokenID)
	},
	&mcp.StreamableHTTPOptions{
		Stateless:           true,
		JSONResponse:        true,
		MaxRequestBodyBytes: 1 << 20,
	},
)

type agentMCPConnectionStatus struct {
	Status    string `json:"status"`
	Group     string `json:"group"`
	Model     string `json:"model"`
	ExpiresAt int64  `json:"expires_at"`
	Unlimited bool   `json:"unlimited_quota"`
	TokenName string `json:"token_name"`
}

type agentMCPModels struct {
	Models []string `json:"models"`
}

type agentMCPUsage struct {
	UnlimitedQuota bool  `json:"unlimited_quota"`
	UsedQuota      int   `json:"used_quota"`
	RemainingQuota int   `json:"remaining_quota"`
	ExpiresAt      int64 `json:"expires_at"`
}

type agentMCPReconnect struct {
	Command string `json:"command"`
	Note    string `json:"note"`
}

// ServeMCP is mounted behind MCPBearerAuth and TokenAuth. It deliberately
// keeps the bearer credential outside every MCP response and tool payload.
func ServeMCP(c *gin.Context) {
	tokenID := c.GetInt("token_id")
	if tokenID <= 0 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	isAgentConnectToken, err := model.IsAgentConnectToken(tokenID)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if !isAgentConnectToken {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), agentMCPContextKey{}, tokenID))
	agentMCPHTTPHandler.ServeHTTP(c.Writer, c.Request)
}

func newAgentMCPServer(tokenID int) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "the-one-gateway",
		Version: agentConnectSkillVersion,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "the_one_connection_status",
		Description: "Read the current The One connection status without exposing credentials.",
		Annotations: agentMCPReadOnlyAnnotations(),
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, agentMCPConnectionStatus, error) {
		token, err := getAgentMCPToken(tokenID)
		if err != nil {
			return nil, agentMCPConnectionStatus{}, err
		}
		return nil, agentMCPConnectionStatus{
			Status:    "connected",
			Group:     token.Group,
			Model:     token.ModelLimits,
			ExpiresAt: token.ExpiredTime,
			Unlimited: token.UnlimitedQuota,
			TokenName: token.Name,
		}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "the_one_list_models",
		Description: "List the model pinned to this The One connection.",
		Annotations: agentMCPReadOnlyAnnotations(),
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, agentMCPModels, error) {
		token, err := getAgentMCPToken(tokenID)
		if err != nil {
			return nil, agentMCPModels{}, err
		}
		return nil, agentMCPModels{Models: token.GetModelLimits()}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "the_one_usage",
		Description: "Read usage and expiry information for this The One connection.",
		Annotations: agentMCPReadOnlyAnnotations(),
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, agentMCPUsage, error) {
		token, err := getAgentMCPToken(tokenID)
		if err != nil {
			return nil, agentMCPUsage{}, err
		}
		return nil, agentMCPUsage{
			UnlimitedQuota: token.UnlimitedQuota,
			UsedQuota:      token.UsedQuota,
			RemainingQuota: token.RemainQuota,
			ExpiresAt:      token.ExpiredTime,
		}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "the_one_reconnect",
		Description: "Show the no-secret command for reconnecting The One in MyAgents.",
		Annotations: agentMCPReadOnlyAnnotations(),
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, agentMCPReconnect, error) {
		return nil, agentMCPReconnect{
			Command: "the-one-connect myagents --base-url " + agentMCPBaseURL(),
			Note:    "This command opens a browser. Complete login and two-factor authentication yourself; never paste an API key.",
		}, nil
	})

	return server
}

func agentMCPReadOnlyAnnotations() *mcp.ToolAnnotations {
	falseValue := false
	return &mcp.ToolAnnotations{
		ReadOnlyHint:    true,
		IdempotentHint:  true,
		DestructiveHint: &falseValue,
		OpenWorldHint:   &falseValue,
	}
}

func getAgentMCPToken(tokenID int) (*model.Token, error) {
	token, err := model.GetTokenById(tokenID)
	if err != nil {
		return nil, errors.New("The One connection is no longer available")
	}
	return token, nil
}

func agentMCPBaseURL() string {
	baseURL := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	if strings.HasPrefix(baseURL, "https://") || strings.HasPrefix(baseURL, "http://") {
		return baseURL
	}
	return "<your-the-one-origin>"
}
