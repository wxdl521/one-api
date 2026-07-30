package controller

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type agentConnectCreateResponse struct {
	RequestID string `json:"request_id"`
}

type agentConnectPairingCreateResponse struct {
	RequestID           string `json:"request_id"`
	AuthorizationPath   string `json:"authorization_path"`
	ExchangePath        string `json:"exchange_path"`
	PollIntervalSeconds int    `json:"poll_interval_seconds"`
}

type agentConnectPairingExchangeResponse struct {
	Pending bool   `json:"pending"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
	Skill   struct {
		Name    string `json:"name"`
		Source  string `json:"source"`
		Version string `json:"version"`
	} `json:"skill"`
}

type agentConnectAuthorizeResponse struct {
	CallbackURL string `json:"callback_url"`
}

type agentConnectExchangeResponse struct {
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
	APIPath string `json:"api_path"`
	MCPPath string `json:"mcp_path"`
}

func TestAgentConnectHTTPFlowOnlyReturnsKeyFromExchange(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AgentConnectRequest{}, &model.Token{}))
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "agent-connect-visible-model",
		ChannelId: 1,
		Enabled:   true,
	}).Error)

	verifier := strings.Repeat("v", 43)
	verifierHash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(verifierHash[:])
	createContext, createRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/agent-connect/requests", map[string]string{
		"client_kind":           "myagents",
		"redirect_uri":          "http://127.0.0.1:43127/callback",
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
		"state":                 strings.Repeat("s", 32),
	}, 0)
	CreateAgentConnectRequest(createContext)
	createResponse := decodeAPIResponse(t, createRecorder)
	require.True(t, createResponse.Success)
	var created agentConnectCreateResponse
	require.NoError(t, common.Unmarshal(createResponse.Data, &created))
	require.NotEmpty(t, created.RequestID)
	assert.NotContains(t, createRecorder.Body.String(), "api_key")

	optionsRecorder := httptest.NewRecorder()
	optionsContext, _ := gin.CreateTestContext(optionsRecorder)
	optionsContext.Request = httptest.NewRequest(http.MethodGet, "/api/agent-connect/requests/"+created.RequestID, nil)
	optionsContext.Params = gin.Params{{Key: "request_id", Value: created.RequestID}}
	optionsContext.Set("id", 42)
	optionsContext.Set("group", "default")
	GetAgentConnectRequestOptions(optionsContext)
	optionsResponse := decodeAPIResponse(t, optionsRecorder)
	require.True(t, optionsResponse.Success)
	assert.Contains(t, optionsRecorder.Body.String(), "agent-connect-visible-model")
	assert.NotContains(t, optionsRecorder.Body.String(), "api_key")

	authorizeContext, authorizeRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/agent-connect/requests/"+created.RequestID+"/authorize", map[string]string{
		"group": "default",
		"model": "agent-connect-visible-model",
	}, 42)
	authorizeContext.Params = gin.Params{{Key: "request_id", Value: created.RequestID}}
	authorizeContext.Set("group", "default")
	AuthorizeAgentConnectRequest(authorizeContext)
	authorizeResponse := decodeAPIResponse(t, authorizeRecorder)
	require.True(t, authorizeResponse.Success)
	var authorized agentConnectAuthorizeResponse
	require.NoError(t, common.Unmarshal(authorizeResponse.Data, &authorized))
	callbackURL, err := url.Parse(authorized.CallbackURL)
	require.NoError(t, err)
	authorizationCode := callbackURL.Query().Get("code")
	require.NotEmpty(t, authorizationCode)
	assert.Equal(t, strings.Repeat("s", 32), callbackURL.Query().Get("state"))
	assert.NotContains(t, authorizeRecorder.Body.String(), "api_key")

	exchangeContext, exchangeRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/agent-connect/exchange", map[string]string{
		"request_id":         created.RequestID,
		"authorization_code": authorizationCode,
		"code_verifier":      verifier,
	}, 0)
	ExchangeAgentConnectRequest(exchangeContext)
	exchangeResponse := decodeAPIResponse(t, exchangeRecorder)
	require.True(t, exchangeResponse.Success)
	var exchanged agentConnectExchangeResponse
	require.NoError(t, common.Unmarshal(exchangeResponse.Data, &exchanged))
	assert.NotEmpty(t, exchanged.APIKey)
	assert.Equal(t, "agent-connect-visible-model", exchanged.Model)
	assert.Equal(t, "/v1", exchanged.APIPath)
	assert.Equal(t, "/mcp", exchanged.MCPPath)
}

func TestAgentConnectAuthorizeRejectsModelOutsideSelectedGroup(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AgentConnectRequest{}, &model.Token{}))
	require.NoError(t, db.Create(&model.Ability{
		Group:     "vip",
		Model:     "agent-connect-vip-only-model",
		ChannelId: 1,
		Enabled:   true,
	}).Error)

	verifier := strings.Repeat("v", 43)
	verifierHash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(verifierHash[:])
	createContext, createRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/agent-connect/requests", map[string]string{
		"client_kind":           "myagents",
		"redirect_uri":          "http://127.0.0.1:43127/callback",
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
		"state":                 strings.Repeat("s", 32),
	}, 0)
	CreateAgentConnectRequest(createContext)
	createResponse := decodeAPIResponse(t, createRecorder)
	require.True(t, createResponse.Success)
	var created agentConnectCreateResponse
	require.NoError(t, common.Unmarshal(createResponse.Data, &created))

	authorizeContext, authorizeRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/agent-connect/requests/"+created.RequestID+"/authorize", map[string]string{
		"group": "default",
		"model": "agent-connect-vip-only-model",
	}, 42)
	authorizeContext.Params = gin.Params{{Key: "request_id", Value: created.RequestID}}
	authorizeContext.Set("group", "default")
	AuthorizeAgentConnectRequest(authorizeContext)

	authorizeResponse := decodeAPIResponse(t, authorizeRecorder)
	assert.False(t, authorizeResponse.Success)
	assert.NotContains(t, authorizeRecorder.Body.String(), "api_key")
}

func TestAgentConnectPairingHTTPFlowReturnsKeyOnlyAfterBrowserApproval(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AgentConnectRequest{}, &model.Token{}))
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "agent-connect-pairing-model",
		ChannelId: 1,
		Enabled:   true,
	}).Error)

	verifier := strings.Repeat("v", 43)
	verifierHash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(verifierHash[:])
	createContext, createRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/agent-connect/pairings", map[string]string{
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
	}, 0)
	CreateAgentConnectPairing(createContext)
	createResponse := decodeAPIResponse(t, createRecorder)
	require.True(t, createResponse.Success)
	var created agentConnectPairingCreateResponse
	require.NoError(t, common.Unmarshal(createResponse.Data, &created))
	require.NotEmpty(t, created.RequestID)
	assert.Equal(t, "/agent-connect?request_id="+url.QueryEscape(created.RequestID), created.AuthorizationPath)
	assert.Equal(t, "/api/agent-connect/pairings/"+url.PathEscape(created.RequestID)+"/exchange", created.ExchangePath)
	assert.Equal(t, 2, created.PollIntervalSeconds)
	assert.NotContains(t, createRecorder.Body.String(), "api_key")

	pendingContext, pendingRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/agent-connect/pairings/"+created.RequestID+"/exchange", map[string]string{
		"code_verifier": verifier,
	}, 0)
	pendingContext.Params = gin.Params{{Key: "request_id", Value: created.RequestID}}
	ExchangeAgentConnectPairing(pendingContext)
	pendingResponse := decodeAPIResponse(t, pendingRecorder)
	require.True(t, pendingResponse.Success)
	var pending agentConnectPairingExchangeResponse
	require.NoError(t, common.Unmarshal(pendingResponse.Data, &pending))
	assert.True(t, pending.Pending)
	assert.Empty(t, pending.APIKey)
	assert.NotContains(t, pendingRecorder.Body.String(), "api_key")

	authorizeContext, authorizeRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/agent-connect/requests/"+created.RequestID+"/authorize", map[string]string{
		"group": "default",
		"model": "agent-connect-pairing-model",
	}, 42)
	authorizeContext.Params = gin.Params{{Key: "request_id", Value: created.RequestID}}
	authorizeContext.Set("group", "default")
	AuthorizeAgentConnectRequest(authorizeContext)
	authorizeResponse := decodeAPIResponse(t, authorizeRecorder)
	require.True(t, authorizeResponse.Success)
	assert.Contains(t, authorizeRecorder.Body.String(), "completed")
	assert.NotContains(t, authorizeRecorder.Body.String(), "callback_url")
	assert.NotContains(t, authorizeRecorder.Body.String(), "api_key")

	exchangeContext, exchangeRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/agent-connect/pairings/"+created.RequestID+"/exchange", map[string]string{
		"code_verifier": verifier,
	}, 0)
	exchangeContext.Params = gin.Params{{Key: "request_id", Value: created.RequestID}}
	ExchangeAgentConnectPairing(exchangeContext)
	exchangeResponse := decodeAPIResponse(t, exchangeRecorder)
	require.True(t, exchangeResponse.Success)
	var exchanged agentConnectPairingExchangeResponse
	require.NoError(t, common.Unmarshal(exchangeResponse.Data, &exchanged))
	assert.False(t, exchanged.Pending)
	assert.NotEmpty(t, exchanged.APIKey)
	assert.Equal(t, "agent-connect-pairing-model", exchanged.Model)
	assert.Equal(t, "the-one-gateway", exchanged.Skill.Name)
	assert.Equal(t, "https://the-one.bolierxiang.cn/skills/myagents/the-one-gateway.zip", exchanged.Skill.Source)
	assert.Equal(t, "1.1.0", exchanged.Skill.Version)
}
