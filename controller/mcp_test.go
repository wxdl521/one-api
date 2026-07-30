package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/middleware"
	"github.com/QuantumNous/the-one/model"
	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mcpBearerTransport struct {
	Authorization string
}

func (transport mcpBearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clonedRequest := request.Clone(request.Context())
	clonedRequest.Header.Set("Authorization", transport.Authorization)
	return http.DefaultTransport.RoundTrip(clonedRequest)
}

func TestMCPExposesOnlyReadOnlyConnectionToolsWithBearerAuth(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}, &model.AgentConnectRequest{}))
	user := &model.User{
		Username: "mcp-user",
		Password: "unused-password-hash",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, db.Create(user).Error)
	token := &model.Token{
		UserId:             user.Id,
		Name:               "MyAgents connection",
		Key:                "mcpTestSecretKey",
		Status:             common.TokenStatusEnabled,
		CreatedTime:        1,
		AccessedTime:       1,
		ExpiredTime:        -1,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: true,
		ModelLimits:        "mcp-test-model",
		Group:              "default",
	}
	require.NoError(t, db.Create(token).Error)
	tokenID := token.Id
	require.NoError(t, db.Create(&model.AgentConnectRequest{
		RequestHash:   "mcp-connection-token",
		ClientKind:    "myagents",
		RedirectURI:   "http://127.0.0.1:43127/callback",
		State:         "mcp-connection-state",
		CodeChallenge: "mcp-connection-challenge",
		UserId:        user.Id,
		Group:         "default",
		Model:         "mcp-test-model",
		Status:        "consumed",
		TokenId:       &tokenID,
		ExpiresAt:     time.Now().Add(time.Minute),
	}).Error)

	router := gin.New()
	router.Any("/mcp", middleware.MCPBearerAuth(), middleware.TokenAuth(), ServeMCP)
	httpServer := httptest.NewServer(router)
	defer httpServer.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:             httpServer.URL + "/mcp",
		HTTPClient:           &http.Client{Transport: mcpBearerTransport{Authorization: "Bearer " + token.Key}},
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}, nil)
	require.NoError(t, err)
	defer session.Close()

	tools, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, tools.Tools, 4)
	toolNames := make(map[string]struct{}, len(tools.Tools))
	for _, tool := range tools.Tools {
		toolNames[tool.Name] = struct{}{}
		require.NotNil(t, tool.Annotations)
		assert.True(t, tool.Annotations.ReadOnlyHint)
		assert.True(t, tool.Annotations.IdempotentHint)
		require.NotNil(t, tool.Annotations.DestructiveHint)
		assert.False(t, *tool.Annotations.DestructiveHint)
	}
	assert.Equal(t, map[string]struct{}{
		"the_one_connection_status": {},
		"the_one_list_models":       {},
		"the_one_usage":             {},
		"the_one_reconnect":         {},
	}, toolNames)

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "the_one_list_models"})
	require.NoError(t, err)
	serializedResult, err := common.Marshal(result)
	require.NoError(t, err)
	assert.Contains(t, string(serializedResult), "mcp-test-model")
	assert.NotContains(t, string(serializedResult), token.Key)
}

func TestMCPRejectsMissingOrExpiredBearerTokens(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	user := &model.User{
		Username: "expired-mcp-user",
		Password: "unused-password-hash",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, db.Create(user).Error)
	expiredToken := &model.Token{
		UserId:       user.Id,
		Name:         "expired mcp token",
		Key:          "expiredMcpTestKey",
		Status:       common.TokenStatusEnabled,
		CreatedTime:  1,
		AccessedTime: 1,
		ExpiredTime:  1,
		Group:        "default",
	}
	require.NoError(t, db.Create(expiredToken).Error)

	router := gin.New()
	router.Any("/mcp", middleware.MCPBearerAuth(), middleware.TokenAuth(), ServeMCP)
	for _, authorization := range []string{"", "Bearer " + expiredToken.Key} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
		request.Header.Set("Content-Type", "application/json")
		if authorization != "" {
			request.Header.Set("Authorization", authorization)
		}

		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	}
}

func TestMCPRejectsBearerKeysThatWereNotIssuedForAgentConnect(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Token{}, &model.AgentConnectRequest{}))
	user := &model.User{
		Username: "ordinary-mcp-user",
		Password: "unused-password-hash",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, db.Create(user).Error)
	ordinaryToken := &model.Token{
		UserId:         user.Id,
		Name:           "ordinary key",
		Key:            "ordinaryMcpTestKey",
		Status:         common.TokenStatusEnabled,
		CreatedTime:    1,
		AccessedTime:   1,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		Group:          "default",
	}
	require.NoError(t, db.Create(ordinaryToken).Error)

	router := gin.New()
	router.Any("/mcp", middleware.MCPBearerAuth(), middleware.TokenAuth(), ServeMCP)
	httpServer := httptest.NewServer(router)
	defer httpServer.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:             httpServer.URL + "/mcp",
		HTTPClient:           &http.Client{Transport: mcpBearerTransport{Authorization: "Bearer " + ordinaryToken.Key}},
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}, nil)
	assert.Error(t, err)
	assert.Nil(t, session)
}
