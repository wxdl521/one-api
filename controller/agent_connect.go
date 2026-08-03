package controller

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/service"
	"github.com/QuantumNous/the-one/setting/system_setting"
	"github.com/gin-gonic/gin"
)

const (
	agentConnectSkillName           = "the-one-gateway"
	agentConnectSkillVersion        = "1.1.0"
	agentConnectMyAgentsSkillSource = "https://the-one.bolierxiang.cn/skills/myagents/the-one-gateway.zip"
	agentConnectHermesSkillSource   = "https://the-one.bolierxiang.cn/skills/hermes/the-one-gateway/SKILL.md"
	agentConnectPollIntervalSeconds = 3
)

type createAgentConnectRequest struct {
	ClientKind          string `json:"client_kind"`
	RedirectURI         string `json:"redirect_uri"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	State               string `json:"state"`
}

type authorizeAgentConnectRequest struct {
	Group string `json:"group"`
	Model string `json:"model"`
}

type exchangeAgentConnectRequest struct {
	RequestID         string `json:"request_id"`
	AuthorizationCode string `json:"authorization_code"`
	Code              string `json:"code"`
	CodeVerifier      string `json:"code_verifier"`
}

type createAgentConnectPairingRequest struct {
	ClientKind          string `json:"client_kind"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
}

type exchangeAgentConnectPairingRequest struct {
	CodeVerifier string `json:"code_verifier"`
}

func CreateAgentConnectRequest(c *gin.Context) {
	var input createAgentConnectRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAgentConnectError(c, model.ErrAgentConnectInvalid)
		return
	}
	requestID, request, err := model.CreateAgentConnectRequest(model.AgentConnectRequestCreate{
		ClientKind:          input.ClientKind,
		RedirectURI:         input.RedirectURI,
		CodeChallenge:       input.CodeChallenge,
		CodeChallengeMethod: input.CodeChallengeMethod,
		State:               input.State,
	})
	if err != nil {
		writeAgentConnectError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"request_id": requestID,
		"expires_at": request.ExpiresAt.Unix(),
	})
}

// CreateAgentConnectPairing creates an in-browser MyAgents Skill pairing.
// Unlike the CLI flow, this request has no loopback callback or state value.
func CreateAgentConnectPairing(c *gin.Context) {
	var input createAgentConnectPairingRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAgentConnectError(c, model.ErrAgentConnectInvalid)
		return
	}
	clientKind := input.ClientKind
	if clientKind == "" {
		clientKind = "myagents-skill"
	}
	if !model.IsAgentConnectPairingClient(clientKind) {
		writeAgentConnectError(c, model.ErrAgentConnectInvalid)
		return
	}
	requestID, request, err := model.CreateAgentConnectRequest(model.AgentConnectRequestCreate{
		ClientKind:          clientKind,
		CodeChallenge:       input.CodeChallenge,
		CodeChallengeMethod: input.CodeChallengeMethod,
	})
	if err != nil {
		writeAgentConnectError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"request_id":            requestID,
		"authorization_path":    "/agent-connect?request_id=" + url.QueryEscape(requestID),
		"authorization_url":     agentConnectOrigin() + "/agent-connect?request_id=" + url.QueryEscape(requestID),
		"exchange_path":         "/api/agent-connect/pairings/" + url.PathEscape(requestID) + "/exchange",
		"expires_at":            request.ExpiresAt.Unix(),
		"poll_interval_seconds": 2,
		"poll_interval_ms":      agentConnectPollIntervalSeconds * 1000,
		"expires_in":            agentConnectExpiresIn(request.ExpiresAt),
	})
}

func GetAgentConnectRequestOptions(c *gin.Context) {
	request, err := model.GetAgentConnectRequest(c.Param("request_id"))
	if err != nil {
		writeAgentConnectError(c, err)
		return
	}
	if !requireAgentConnectFreshLogin(c, request) {
		return
	}
	common.ApiSuccess(c, gin.H{
		"groups": service.GetAgentConnectGroupOptions(c.GetString("group")),
	})
}

func AuthorizeAgentConnectRequest(c *gin.Context) {
	var input authorizeAgentConnectRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAgentConnectError(c, model.ErrAgentConnectInvalid)
		return
	}
	requestID := c.Param("request_id")
	request, err := model.GetAgentConnectRequest(requestID)
	if err != nil {
		writeAgentConnectError(c, err)
		return
	}
	if !requireAgentConnectFreshLogin(c, request) {
		return
	}
	if !service.IsAgentConnectSelectionAllowed(c.GetString("group"), input.Group, input.Model) {
		writeAgentConnectError(c, model.ErrAgentConnectInvalid)
		return
	}
	authorizationCode, err := model.AuthorizeAgentConnectRequest(requestID, c.GetInt("id"), input.Group, input.Model)
	if err != nil {
		writeAgentConnectError(c, err)
		return
	}
	if request.PairingMode {
		common.ApiSuccess(c, gin.H{
			"completed": true,
			"message":   "Connection approved. Return to MyAgents.",
		})
		return
	}
	callbackURL, err := url.Parse(request.RedirectURI)
	if err != nil {
		common.SysError("agent connect request had an invalid validated redirect URI: " + err.Error())
		writeAgentConnectError(c, model.ErrAgentConnectInvalid)
		return
	}
	query := callbackURL.Query()
	query.Set("code", authorizationCode)
	query.Set("state", request.State)
	callbackURL.RawQuery = query.Encode()
	common.ApiSuccess(c, gin.H{"callback_url": callbackURL.String()})
}

func CancelAgentConnectRequest(c *gin.Context) {
	if err := model.CancelAgentConnectRequest(c.Param("request_id"), c.GetInt("id")); err != nil {
		writeAgentConnectError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func ExchangeAgentConnectRequest(c *gin.Context) {
	var input exchangeAgentConnectRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAgentConnectError(c, model.ErrAgentConnectInvalid)
		return
	}
	if input.Code != "" {
		if input.AuthorizationCode != "" && input.AuthorizationCode != input.Code {
			writeAgentConnectError(c, model.ErrAgentConnectInvalid)
			return
		}
		input.AuthorizationCode = input.Code
	}
	token, err := model.ExchangeAgentConnectRequest(input.RequestID, input.AuthorizationCode, input.CodeVerifier)
	if err != nil {
		writeAgentConnectError(c, err)
		return
	}
	writeAgentConnectManifest(c, token, agentConnectMyAgentsSkillSource)
}

func ExchangeAgentConnectPairing(c *gin.Context) {
	var input exchangeAgentConnectPairingRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAgentConnectError(c, model.ErrAgentConnectInvalid)
		return
	}
	request, err := model.GetAgentConnectRequest(c.Param("request_id"))
	if err != nil {
		writeAgentConnectError(c, err)
		return
	}
	token, clientKind, err := model.ExchangeAgentConnectPairingManifest(c.Param("request_id"), input.CodeVerifier)
	if errors.Is(err, model.ErrAgentConnectNotAuthorized) {
		common.ApiSuccess(c, gin.H{
			"pending":          true,
			"status":           "waiting_user",
			"poll_interval_ms": agentConnectPollIntervalSeconds * 1000,
			"expires_in":       agentConnectExpiresIn(request.ExpiresAt),
		})
		return
	}
	if err != nil {
		writeAgentConnectError(c, err)
		return
	}
	writeAgentConnectManifest(c, token, agentConnectSkillSourceForClient(clientKind))
}

func writeAgentConnectManifest(c *gin.Context, token *model.Token, skillSource string) {
	baseURL := agentConnectOrigin()
	common.ApiSuccess(c, gin.H{
		"api_key":    token.Key,
		"expires_at": token.ExpiredTime,
		"model":      token.ModelLimits,
		"api_path":   "/v1",
		"mcp_path":   "/mcp",
		"skill": gin.H{
			"name":    agentConnectSkillName,
			"version": agentConnectSkillVersion,
			"source":  skillSource,
		},
		"manifest": gin.H{
			"api_key":       token.Key,
			"api_key_env":   "THE_ONE_API_KEY",
			"provider_name": "the-one-bolierxiang-cn",
			"base_url":      baseURL + "/v1",
			"api_mode":      "chat_completions",
			"model":         token.ModelLimits,
			"group":         token.Group,
			"scopes":        []string{"chat", "mcp:readonly"},
			"mcp": gin.H{
				"name": "the-one-gateway-bolierxiang-cn",
				"url":  baseURL + "/mcp",
				"tools_include": []string{
					"the_one_connection_status",
					"the_one_list_models",
					"the_one_usage",
					"the_one_reconnect",
				},
			},
			"skill_url": skillSource,
		},
	})
}

func agentConnectOrigin() string {
	return strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
}

func agentConnectExpiresIn(expiresAt time.Time) int64 {
	remaining := int64(time.Until(expiresAt).Seconds())
	if remaining < 1 {
		return 1
	}
	return remaining
}

func agentConnectSkillSourceForClient(clientKind string) string {
	if clientKind == "hermes-skill" {
		return agentConnectHermesSkillSource
	}
	return agentConnectMyAgentsSkillSource
}

func writeAgentConnectError(c *gin.Context, err error) {
	code := "invalid_request"
	message := "The connection request is invalid."
	switch {
	case errors.Is(err, model.ErrAgentConnectExpired):
		code = "expired"
		message = "The connection request has expired. Start the connection again."
	case errors.Is(err, model.ErrAgentConnectCanceled):
		code = "denied"
		message = "The connection request was canceled."
	case errors.Is(err, model.ErrAgentConnectConsumed):
		code = "revoked"
		message = "The connection request has already been used."
	case errors.Is(err, model.ErrAgentConnectTokenLimit):
		code = "token_limit"
		message = "Your account has reached its API key limit."
	case errors.Is(err, model.ErrAgentConnectInvalidVerifier):
		code = "invalid_verifier"
		message = "The local connection could not be verified."
	case errors.Is(err, model.ErrAgentConnectReauthenticationRequired):
		code = "reauthentication_required"
		message = "Sign in again to continue the connection."
	case errors.Is(err, model.ErrAgentConnectNotAuthorized):
		code = "waiting_user"
		message = "Confirm the connection in the browser before continuing."
	case errors.Is(err, model.ErrAgentConnectInvalid):
		code = "invalid_request"
	default:
		common.SysError("agent connect request failed: " + err.Error())
		code = "internal_error"
		message = "Unable to complete the connection request."
	}
	c.JSON(200, gin.H{"success": false, "message": message, "error": gin.H{"code": code}})
}
