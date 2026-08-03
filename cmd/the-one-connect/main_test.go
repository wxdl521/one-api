package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalOriginRequiresHTTPSOutsideLoopback(t *testing.T) {
	origin, err := canonicalOrigin("HTTPS://Gateway.Example.com/")
	require.NoError(t, err)
	assert.Equal(t, "https://gateway.example.com", origin)

	_, err = canonicalOrigin("http://gateway.example.com")
	assert.Error(t, err)
	_, err = canonicalOrigin("https://gateway.example.com/path")
	assert.Error(t, err)
	origin, err = canonicalOrigin("http://127.0.0.1:3000")
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:3000", origin)
}

func TestCallbackCodeRejectsStateMismatch(t *testing.T) {
	values := url.Values{"code": {"authorization-code"}, "state": {"unexpected"}}
	_, err := callbackCode(values, "expected")
	assert.Error(t, err)

	code, err := callbackCode(url.Values{"code": {"authorization-code"}, "state": {"expected"}}, "expected")
	require.NoError(t, err)
	assert.Equal(t, "authorization-code", code)
}

func TestConfigureMyAgentsUsesStableIDsAndNeverWritesTheDefaultProvider(t *testing.T) {
	var mutex sync.Mutex
	requests := make([]struct {
		path string
		body map[string]any
	}, 0)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var body map[string]any
		require.NoError(t, common.DecodeJson(request.Body, &body))
		mutex.Lock()
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: request.URL.Path, body: body})
		mutex.Unlock()
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	manifest := agentConnectManifest{
		APIKey:  "secret-that-must-never-be-printed",
		Model:   "gpt-4.1-mini",
		APIPath: "/v1",
		MCPPath: "/mcp",
		Skill: agentConnectSkillManifest{
			Name:    "the-one-gateway",
			Version: "1.0.0",
			Source:  "https://the-one.bolierxiang.cn/skills/myagents/the-one-gateway.zip",
		},
	}
	var output strings.Builder
	err := configureMyAgents(context.Background(), server.Client(), server.URL+"/api/admin", "https://gateway.example.com", manifest, &output)
	require.NoError(t, err)
	assert.NotContains(t, output.String(), manifest.APIKey)

	require.Len(t, requests, 6)
	assert.Equal(t, "/api/admin/model/add", requests[0].path)
	provider, ok := requests[0].body["provider"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, stableConnectionID("https://gateway.example.com"), provider["id"])
	assert.Equal(t, "https://gateway.example.com/v1", provider["baseUrl"])
	assert.NotContains(t, requests[0].body, "default")
	assert.NotContains(t, requests[0].body, "setDefault")

	assert.Equal(t, "/api/admin/model/set-key", requests[1].path)
	assert.Equal(t, manifest.APIKey, requests[1].body["apiKey"])
	assert.Equal(t, "/api/admin/mcp/add", requests[2].path)
	mcpServer, ok := requests[2].body["server"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "http", mcpServer["type"])
	assert.Equal(t, "https://gateway.example.com/mcp", mcpServer["url"])
	headers, ok := mcpServer["headers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Bearer "+manifest.APIKey, headers["Authorization"])
	assert.Equal(t, "/api/admin/mcp/enable", requests[3].path)
	assert.Equal(t, "/api/admin/skill/add", requests[4].path)
	assert.Equal(t, manifest.Skill.Source, requests[4].body["url"])
	assert.Equal(t, skillScope, requests[4].body["scope"])
	assert.Equal(t, manifest.Skill.Name, requests[4].body["skill"])
	assert.Equal(t, "/api/admin/mcp/test", requests[5].path)
}
