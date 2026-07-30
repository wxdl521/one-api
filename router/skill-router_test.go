package router

import (
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
