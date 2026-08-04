package controller

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/middleware"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/relaykit/types"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
)

const (
	miniAppTextTestRequestMaxBytes = 64 << 10
	miniAppTextTestWaitTimeout     = 20 * time.Second
)

var miniAppTextTestRelay = executeMiniAppTextTestRelay

// miniAppTextTestRelayResponseWriter keeps only the relay status and headers.
// The Mini Program BFF never returns upstream output, so retaining it in a
// ResponseRecorder would allow an unbounded response body to accumulate in
// memory without serving a caller.
type miniAppTextTestRelayResponseWriter struct {
	header     http.Header
	statusCode int
}

func newMiniAppTextTestRelayResponseWriter() *miniAppTextTestRelayResponseWriter {
	return &miniAppTextTestRelayResponseWriter{header: make(http.Header)}
}

func (w *miniAppTextTestRelayResponseWriter) Header() http.Header {
	return w.header
}

func (w *miniAppTextTestRelayResponseWriter) WriteHeader(statusCode int) {
	if w.statusCode == 0 {
		w.statusCode = statusCode
	}
}

func (w *miniAppTextTestRelayResponseWriter) Write(data []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return len(data), nil
}

func (w *miniAppTextTestRelayResponseWriter) Flush() {}

func (w *miniAppTextTestRelayResponseWriter) StatusCode() int {
	if w.statusCode == 0 {
		return http.StatusOK
	}
	return w.statusCode
}

func (w *miniAppTextTestRelayResponseWriter) BufferedBytes() int { return 0 }

func MiniAppListTextTestModels(c *gin.Context) {
	models, err := service.ListMiniTextTestModels(c.GetInt("id"))
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"models": models})
}

func MiniAppStartTextTest(c *gin.Context) {
	fields, ok := decodeMiniAppRequestWithMaxBytes(c, miniAppTextTestRequestMaxBytes, "client_request_id", "model", "input")
	if !ok {
		return
	}
	requestID, ok := miniAppRequiredString(c, fields, "client_request_id", 128)
	if !ok {
		return
	}
	modelName, ok := miniAppRequiredString(c, fields, "model", 255)
	if !ok {
		return
	}
	inputRaw, ok := fields["input"]
	if !ok {
		writeMiniAppInvalidRequest(c)
		return
	}
	var input string
	if err := common.Unmarshal(inputRaw, &input); err != nil {
		writeMiniAppInvalidRequest(c)
		return
	}

	request := service.MiniTextTestRequest{
		ClientRequestID: requestID,
		Model:           modelName,
		Input:           input,
	}
	status, created, err := service.StartMiniTextTest(c.GetInt("id"), request)
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	if !created {
		common.ApiSuccess(c, status)
		return
	}

	completion := miniAppTextTestRelay(c, request)
	status, err = service.CompleteMiniTextTest(c.GetInt("id"), request.ClientRequestID, completion)
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	if completion.State != model.MiniTextTestAttemptStateSucceeded {
		writeMiniAppTextTestError(c, completion.ErrorCode)
		return
	}
	common.ApiSuccess(c, status)
}

func MiniAppTextTestStatus(c *gin.Context) {
	requestID := strings.TrimSpace(c.Param("request_id"))
	status, err := service.GetMiniTextTestStatus(c.GetInt("id"), requestID)
	if err != nil {
		writeMiniAppError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}

func miniAppTextTestRelayContext(c *gin.Context) (context.Context, context.CancelFunc) {
	// A Mini Program can cancel the client request while its process is sent to
	// the background. The accepted test must still run the normal relay billing
	// and settlement lifecycle, so its server context deliberately does not
	// inherit client cancellation. Keep the sensitive-payload marker on this
	// detached, bounded context so internal relay diagnostics stay redacted.
	return context.WithTimeout(
		common.WithSensitiveRelayPayloadLogging(context.WithoutCancel(c.Request.Context())),
		miniAppTextTestWaitTimeout,
	)
}

func miniAppTextTestContextCompletion(requestID string, err error) (service.MiniTextTestCompletion, bool) {
	if errors.Is(err, context.DeadlineExceeded) {
		return service.MiniTextTestCompletion{
			State:           model.MiniTextTestAttemptStateTimedOut,
			ChargeReference: requestID,
			ErrorCode:       "MINIAPP_TEXT_TEST_TIMEOUT",
		}, true
	}
	if errors.Is(err, context.Canceled) {
		return service.MiniTextTestCompletion{
			State:           model.MiniTextTestAttemptStateFailed,
			ChargeReference: requestID,
			ErrorCode:       "MINIAPP_TEXT_TEST_UNAVAILABLE",
		}, true
	}
	return service.MiniTextTestCompletion{}, false
}

func executeMiniAppTextTestRelay(c *gin.Context, request service.MiniTextTestRequest) service.MiniTextTestCompletion {
	requestID := c.GetString(common.RequestIdKey)
	if requestID == "" {
		requestID = common.NewRequestId()
	}
	relayRequest, err := common.Marshal(gin.H{
		"model": request.Model,
		"messages": []gin.H{{
			"role":    "user",
			"content": request.Input,
		}},
		"stream":                false,
		"max_completion_tokens": 512,
	})
	if err != nil {
		return service.MiniTextTestCompletion{
			State:           model.MiniTextTestAttemptStateFailed,
			ChargeReference: requestID,
			ErrorCode:       "MINIAPP_TEXT_TEST_UNAVAILABLE",
		}
	}

	relayWriter := newMiniAppTextTestRelayResponseWriter()
	relayContext, _ := gin.CreateTestContext(relayWriter)
	for key, value := range c.Keys {
		relayContext.Set(key, value)
	}
	// Never reuse a BFF request body's storage for the internal relay. The
	// synthetic body below is the complete and fixed upstream contract.
	relayContext.Set(common.KeyBodyStorage, nil)
	relayContext.Set(common.KeyRequestBody, nil)
	defer common.CleanupBodyStorage(relayContext)
	relayContext.Set(common.RequestIdKey, requestID)
	relayRequestContext, cancel := miniAppTextTestRelayContext(c)
	defer cancel()
	relayContext.Request = c.Request.Clone(relayRequestContext)
	relayContext.Request.Method = http.MethodPost
	relayContext.Request.URL.Path = "/pg/chat/completions"
	relayContext.Request.URL.RawPath = ""
	relayContext.Request.RequestURI = relayContext.Request.URL.RequestURI()
	relayContext.Request.Header = make(http.Header)
	relayContext.Request.Header.Set("Content-Type", "application/json")
	relayContext.Request.Header.Set("Accept", "application/json")
	relayContext.Request.Body = io.NopCloser(bytes.NewReader(relayRequest))
	relayContext.Request.ContentLength = int64(len(relayRequest))
	if completion, terminal := miniAppTextTestContextCompletion(requestID, relayRequestContext.Err()); terminal {
		return completion
	}

	// This is the existing relay entry sequence: channel distribution occurs
	// before pricing and Relay performs the normal estimate/pre-consume/
	// settle/refund/consume-log lifecycle. /pg marks the internally generated
	// request as tokenless without introducing an alternate billing client.
	middleware.Distribute()(relayContext)
	if !relayContext.IsAborted() {
		Relay(relayContext, types.RelayFormatOpenAI)
	}

	if completion, terminal := miniAppTextTestContextCompletion(requestID, relayRequestContext.Err()); terminal {
		return completion
	}
	if relayWriter.StatusCode() < http.StatusOK || relayWriter.StatusCode() >= http.StatusMultipleChoices {
		errorCode := "MINIAPP_TEXT_TEST_UNAVAILABLE"
		if relayWriter.StatusCode() == http.StatusBadRequest || relayWriter.StatusCode() == http.StatusForbidden || relayWriter.StatusCode() == http.StatusTooManyRequests {
			errorCode = "MINIAPP_TEXT_TEST_REJECTED"
		}
		return service.MiniTextTestCompletion{
			State:           model.MiniTextTestAttemptStateFailed,
			ChargeReference: requestID,
			ErrorCode:       errorCode,
		}
	}

	completion := service.MiniTextTestCompletion{
		State:           model.MiniTextTestAttemptStateSucceeded,
		ChargeReference: requestID,
	}
	if consumeLog, logErr := model.GetConsumeLogByRequestID(c.GetInt("id"), requestID); logErr == nil {
		completion.ChargedQuota = consumeLog.Quota
	}
	return completion
}

func writeMiniAppTextTestError(c *gin.Context, code string) {
	status := http.StatusBadGateway
	if code == "MINIAPP_TEXT_TEST_REJECTED" {
		status = http.StatusForbidden
	} else if code == "MINIAPP_TEXT_TEST_TIMEOUT" {
		status = http.StatusGatewayTimeout
	}
	c.JSON(status, gin.H{
		"success": false,
		"code":    code,
		"message": http.StatusText(status),
	})
}
