package router

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillRouterServesSafeVersionedMarkdown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetSkillRouter(engine)

	testCases := []struct {
		path         string
		expectedName string
	}{
		{path: "/skills/myagents/SKILL.md", expectedName: "name: the-one-myagents-pairing"},
		{path: "/skills/myagents/the-one-gateway/SKILL.md", expectedName: "name: the-one-gateway"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, testCase.path, nil)
			engine.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code)
			assert.True(t, strings.HasPrefix(recorder.Header().Get("Content-Type"), "text/markdown"))
			assert.NotEmpty(t, recorder.Header().Get("X-The-One-Skill-Version"))
			assert.Contains(t, recorder.Body.String(), testCase.expectedName)
		})
	}
}

func TestSkillRouterServesInstallableGatewayArchive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetSkillRouter(engine)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/skills/myagents/the-one-gateway.zip", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/zip"))
	assert.NotEmpty(t, recorder.Header().Get("X-The-One-Skill-Version"))

	archive, err := zip.NewReader(bytes.NewReader(recorder.Body.Bytes()), int64(recorder.Body.Len()))
	require.NoError(t, err)
	require.Len(t, archive.File, 1)
	assert.Equal(t, "the-one-gateway/SKILL.md", archive.File[0].Name)
}

func TestOnboardingSkillDoesNotInstructExecutableOrCredentialDisclosure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetSkillRouter(engine)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/skills/myagents/SKILL.md", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	content := strings.ToLower(recorder.Body.String())
	for _, forbidden := range []string{"invoke-webrequest", "curl", ".exe", "the-one-connect", "authorization: bearer"} {
		assert.NotContains(t, content, forbidden)
	}
	assert.Contains(t, content, "never inspect, copy, print, log, or disclose")
	assert.Contains(t, content, "do not change the current default provider")
}

func TestHermesSkillsArePublicAndPreserveTheDefaultModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetSkillRouter(engine)

	for path, expectedName := range map[string]string{
		"/skills/hermes/SKILL.md":                 "name: the-one-hermes-pairing",
		"/skills/hermes/the-one-gateway/SKILL.md": "name: the-one-gateway",
	} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.True(t, strings.HasPrefix(recorder.Header().Get("Content-Type"), "text/markdown"))
		assert.Contains(t, recorder.Body.String(), expectedName)
	}

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/skills/hermes/SKILL.md", nil))
	content := strings.ToLower(recorder.Body.String())
	assert.Contains(t, content, "do not change the current default model")
	assert.Contains(t, content, "the_one_api_key")
	assert.Contains(t, content, "agent-controlled browser")
	assert.Contains(t, content, "authorization_url")
	assert.Contains(t, content, "data.pending")
	assert.Contains(t, content, "retry-after")
	assert.Contains(t, content, "system browser")
	for _, forbidden := range []string{"invoke-webrequest", "curl", ".exe", "the-one-connect", "authorization: bearer"} {
		assert.NotContains(t, content, forbidden)
	}
}
