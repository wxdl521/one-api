package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptShotPreflightRejectsOversizedContentLengthBeforeBodyRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	called := false
	engine.POST("/promptshot", PromptShotPreflight(), func(c *gin.Context) {
		called = true
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/promptshot", bytes.NewReader([]byte("{}")))
	request.ContentLength = PromptShotMaxRequestBytes + 1
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	assert.False(t, called)
}

func TestPromptShotPreflightLimitsChunkedBodyBeforeTokenParsing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = originalRedisEnabled })
	engine := gin.New()
	engine.POST("/promptshot", PromptShotPreflight(), PromptShotBodyTokenAuth(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/promptshot", io.NopCloser(bytes.NewReader(make([]byte, PromptShotMaxRequestBytes+1))))
	request.ContentLength = -1
	request.Header.Set("Content-Type", gin.MIMEJSON)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
}

func TestPromptShotPreflightRateLimitsConcurrentBodiesBeforeHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = originalRedisEnabled })
	engine := gin.New()
	engine.POST("/promptshot", promptShotPreflightWithRateLimit(1, 60, "PSPF-CONCURRENT-TEST"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	var waitGroup sync.WaitGroup
	statuses := make(chan int, 4)
	for index := 0; index < 4; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			request := httptest.NewRequest(http.MethodPost, "/promptshot", bytes.NewBufferString(`{}`))
			request.RemoteAddr = "192.0.2.77:12345"
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			statuses <- recorder.Code
		}()
	}
	waitGroup.Wait()
	close(statuses)

	successes := 0
	limited := 0
	for status := range statuses {
		if status == http.StatusNoContent {
			successes++
		}
		if status == http.StatusTooManyRequests {
			limited++
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 3, limited)
}

func TestPromptShotPreflightFallsBackToMemoryWhenRedisIsNotInitialized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalRedisEnabled := common.RedisEnabled
	originalRedisClient := common.RDB
	common.RedisEnabled = true
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRedisClient
	})
	engine := gin.New()
	engine.POST("/promptshot", promptShotPreflightWithRateLimit(1, 60, "PSPF-NIL-REDIS-TEST"), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/promptshot", bytes.NewBufferString(`{}`)))

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestPromptShotBodyTokenAuthMovesCredentialToAuthorizationAndPreservesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/validate", PromptShotBodyTokenAuth(), func(c *gin.Context) {
		var body promptShotBodyTokenRequest
		require.NoError(t, common.UnmarshalBodyReusable(c, &body))
		assert.Equal(t, "Bearer example-token", c.GetHeader("Authorization"))
		assert.Equal(t, "example-token", body.Token)
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBufferString(`{"token":"example-token"}`))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "example-token")
}
