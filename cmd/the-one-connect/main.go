package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/QuantumNous/the-one/common"
)

const (
	callbackPath = "/callback"
	skillScope   = "user"
)

type agentConnectSkillManifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"`
}

type agentConnectManifest struct {
	APIKey    string                    `json:"api_key"`
	ExpiresAt int64                     `json:"expires_at"`
	Model     string                    `json:"model"`
	APIPath   string                    `json:"api_path"`
	MCPPath   string                    `json:"mcp_path"`
	Skill     agentConnectSkillManifest `json:"skill"`
}

type gatewayEnvelope[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type createRequestResponse struct {
	RequestID string `json:"request_id"`
}

func main() {
	contextWithSignals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(contextWithSignals, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Connection failed:", safeErrorMessage(err))
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) == 0 || arguments[0] != "myagents" {
		return errors.New("usage: the-one-connect myagents --base-url <origin>")
	}
	flags := flag.NewFlagSet("myagents", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	baseURL := flags.String("base-url", "", "The One origin")
	if err := flags.Parse(arguments[1:]); err != nil {
		return errors.New("usage: the-one-connect myagents --base-url <origin>")
	}
	origin, err := canonicalOrigin(*baseURL)
	if err != nil {
		return err
	}
	return connectMyAgents(ctx, origin, output)
}

func connectMyAgents(ctx context.Context, origin string, output io.Writer) error {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return errors.New("could not start the local authorization callback")
	}
	defer listener.Close()

	state, err := randomURLValue()
	if err != nil {
		return errors.New("could not create authorization state")
	}
	verifier, err := randomURLValue()
	if err != nil {
		return errors.New("could not create PKCE verifier")
	}
	verifierHash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(verifierHash[:])
	callbackURL := "http://" + listener.Addr().String() + callbackPath
	httpClient := &http.Client{Timeout: 20 * time.Second}

	requestID, err := createGatewayRequest(ctx, httpClient, origin, callbackURL, challenge, state)
	if err != nil {
		return err
	}
	authorizationURL := origin + "/agent-connect?request_id=" + url.QueryEscape(requestID)
	fmt.Fprintln(output, "Opening the secure The One confirmation page in your browser...")
	if err := openBrowser(authorizationURL); err != nil {
		return errors.New("could not open a browser; open the connection URL from the The One dashboard and try again")
	}

	code, err := waitForCallback(ctx, listener, state)
	if err != nil {
		return err
	}
	manifest, err := exchangeGatewayRequest(ctx, httpClient, origin, requestID, code, verifier)
	if err != nil {
		return err
	}
	if manifest.APIKey == "" || manifest.Model == "" || manifest.APIPath == "" || manifest.MCPPath == "" || manifest.Skill.Name == "" || manifest.Skill.Source == "" {
		return errors.New("The One returned an incomplete connection manifest")
	}

	adminURL, err := myAgentsAdminURL()
	if err != nil {
		return err
	}
	if err := configureMyAgents(ctx, httpClient, adminURL, origin, manifest, output); err != nil {
		return err
	}
	if err := verifyGatewayModels(ctx, httpClient, origin, manifest); err != nil {
		return err
	}
	fmt.Fprintln(output, "The One is configured in MyAgents. Your existing default provider was not changed.")
	return nil
}

func canonicalOrigin(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("base URL must be an HTTPS origin")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("base URL must not include a path")
	}
	scheme := strings.ToLower(parsed.Scheme)
	hostname := strings.ToLower(parsed.Hostname())
	ip := net.ParseIP(hostname)
	if scheme != "https" && (scheme != "http" || ip == nil || !ip.IsLoopback()) {
		return "", errors.New("base URL must use HTTPS outside loopback development")
	}
	if hostname == "" {
		return "", errors.New("base URL must include a hostname")
	}
	if port := parsed.Port(); port != "" {
		portNumber, portErr := strconv.Atoi(port)
		if portErr != nil || portNumber < 1 || portNumber > 65535 {
			return "", errors.New("base URL has an invalid port")
		}
	}
	host := hostname
	if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	if port := parsed.Port(); port != "" {
		host += ":" + port
	}
	return scheme + "://" + host, nil
}

func callbackCode(values url.Values, expectedState string) (string, error) {
	code := values.Get("code")
	state := values.Get("state")
	if code == "" || len(code) > 512 || state == "" ||
		subtle.ConstantTimeCompare([]byte(state), []byte(expectedState)) != 1 {
		return "", errors.New("the local callback could not be verified")
	}
	return code, nil
}

func waitForCallback(ctx context.Context, listener net.Listener, expectedState string) (string, error) {
	result := make(chan string, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != callbackPath {
			http.NotFound(response, request)
			return
		}
		code, err := callbackCode(request.URL.Query(), expectedState)
		if err != nil {
			http.Error(response, "The local callback could not be verified. Return to The One and try again.", http.StatusBadRequest)
			return
		}
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(response, "<html><body><p>Connection confirmed. You can return to MyAgents.</p></body></html>")
		select {
		case result <- code:
		default:
		}
	})}
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.Serve(listener)
	}()
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()

	select {
	case code := <-result:
		return code, nil
	case <-ctx.Done():
		return "", errors.New("connection was canceled")
	case err := <-serveErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return "", errors.New("the local authorization callback stopped unexpectedly")
		}
		return "", errors.New("the local authorization callback stopped")
	}
}

func createGatewayRequest(ctx context.Context, client *http.Client, origin string, redirectURI string, challenge string, state string) (string, error) {
	var response gatewayEnvelope[createRequestResponse]
	err := postJSON(ctx, client, origin+"/api/agent-connect/requests", map[string]string{
		"client_kind":           "myagents",
		"redirect_uri":          redirectURI,
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
		"state":                 state,
	}, &response)
	if err != nil || !response.Success || response.Data.RequestID == "" {
		return "", errors.New("The One could not start the connection request")
	}
	return response.Data.RequestID, nil
}

func exchangeGatewayRequest(ctx context.Context, client *http.Client, origin string, requestID string, code string, verifier string) (agentConnectManifest, error) {
	var response gatewayEnvelope[agentConnectManifest]
	err := postJSON(ctx, client, origin+"/api/agent-connect/exchange", map[string]string{
		"request_id":         requestID,
		"authorization_code": code,
		"code_verifier":      verifier,
	}, &response)
	if err != nil || !response.Success {
		return agentConnectManifest{}, errors.New("The One could not complete the connection request")
	}
	return response.Data, nil
}

func configureMyAgents(ctx context.Context, client *http.Client, adminURL string, origin string, manifest agentConnectManifest, output io.Writer) error {
	providerID := stableConnectionID(origin)
	mcpID := providerID + "-mcp"
	modelBaseURL := origin + manifest.APIPath
	mcpURL := origin + manifest.MCPPath

	if err := callMyAgentsAdmin(ctx, client, adminURL, "/model/add", map[string]any{
		"provider": map[string]any{
			"id":             providerID,
			"name":           "The One",
			"baseUrl":        modelBaseURL,
			"models":         []string{manifest.Model},
			"primaryModel":   manifest.Model,
			"authType":       "api_key",
			"protocol":       "openai",
			"upstreamFormat": "chat_completions",
		},
		"dryRun": false,
	}); err != nil {
		return err
	}
	if err := callMyAgentsAdmin(ctx, client, adminURL, "/model/set-key", map[string]any{
		"id":     providerID,
		"apiKey": manifest.APIKey,
	}); err != nil {
		return err
	}
	if err := callMyAgentsAdmin(ctx, client, adminURL, "/mcp/add", map[string]any{
		"server": map[string]any{
			"id":          mcpID,
			"name":        "The One Gateway",
			"type":        "http",
			"url":         mcpURL,
			"description": "Read-only The One connection tools",
			"headers": map[string]string{
				"Authorization": "Bearer " + manifest.APIKey,
			},
		},
		"dryRun": false,
	}); err != nil {
		return err
	}
	if err := callMyAgentsAdmin(ctx, client, adminURL, "/mcp/enable", map[string]any{
		"id":    mcpID,
		"scope": "global",
	}); err != nil {
		return err
	}
	if err := callMyAgentsAdmin(ctx, client, adminURL, "/skill/add", map[string]any{
		"url":   manifest.Skill.Source,
		"scope": skillScope,
		"skill": manifest.Skill.Name,
	}); err != nil {
		return err
	}
	if err := callMyAgentsAdmin(ctx, client, adminURL, "/mcp/test", map[string]any{"id": mcpID}); err != nil {
		return err
	}
	fmt.Fprintln(output, "MyAgents provider, read-only MCP, and user Skill are configured.")
	return nil
}

func callMyAgentsAdmin(ctx context.Context, client *http.Client, adminURL string, path string, payload any) error {
	var response struct {
		Success *bool `json:"success"`
	}
	if err := postJSON(ctx, client, strings.TrimRight(adminURL, "/")+path, payload, &response); err != nil {
		return errors.New("MyAgents configuration request failed")
	}
	if response.Success != nil && !*response.Success {
		return errors.New("MyAgents rejected a configuration request")
	}
	return nil
}

func verifyGatewayModels(ctx context.Context, client *http.Client, origin string, manifest agentConnectManifest) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+manifest.APIPath+"/models", nil)
	if err != nil {
		return errors.New("could not verify the The One model connection")
	}
	request.Header.Set("Authorization", "Bearer "+manifest.APIKey)
	response, err := client.Do(request)
	if err != nil {
		return errors.New("could not verify the The One model connection")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return errors.New("The One model connection verification failed")
	}
	return nil
}

func postJSON(ctx context.Context, client *http.Client, endpoint string, payload any, destination any) error {
	body, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
	}
	return common.DecodeJson(response.Body, destination)
}

func myAgentsAdminURL() (string, error) {
	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("could not find the MyAgents configuration directory")
	}
	portFile := filepath.Join(homeDirectory, ".myagents", "sidecar.port")
	content, err := os.ReadFile(portFile)
	if err != nil {
		return "", errors.New("MyAgents is not running; start it and run this command again")
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil || port < 1 || port > 65535 {
		return "", errors.New("MyAgents reported an invalid local management port")
	}
	return "http://127.0.0.1:" + strconv.Itoa(port) + "/api/admin", nil
}

func stableConnectionID(origin string) string {
	digest := sha256.Sum256([]byte(origin))
	return "the-one-" + hex.EncodeToString(digest[:])[:12]
}

func randomURLValue() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func openBrowser(target string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
	case "darwin":
		return exec.Command("open", target).Start()
	default:
		return errors.New("automatic browser opening is supported on Windows and macOS")
	}
}

func safeErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if strings.Contains(strings.ToLower(message), "api key") || strings.Contains(strings.ToLower(message), "apikey") {
		return "a local configuration request failed"
	}
	return message
}
