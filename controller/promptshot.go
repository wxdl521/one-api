package controller

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/constant"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/relaykit/types"
	"github.com/QuantumNous/the-one/service"
	"github.com/QuantumNous/the-one/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const promptShotMaxImageBytes = 20 << 20

var (
	errPromptShotInvalidImage           = errors.New("promptshot image is invalid")
	errPromptShotInvalidQuality         = errors.New("promptshot quality is invalid")
	errPromptShotInvalidRequest         = errors.New("promptshot request is invalid")
	errPromptShotImageNotBase64         = errors.New("promptshot image response is not base64")
	errPromptShotInvalidReverseResponse = errors.New("promptshot reverse response is invalid")
)

type promptShotGenerateRequest struct {
	Prompt      string   `json:"prompt"`
	RefImageB64 string   `json:"ref_image_b64"`
	RefMIME     string   `json:"ref_mime"`
	Temperature *float64 `json:"temperature"`
}

type promptShotCleanRequest struct {
	ImageB64 string `json:"image_b64"`
	MIME     string `json:"mime"`
	Quality  string `json:"quality"`
}

type promptShotReverseRequest struct {
	ImageB64 string `json:"image_b64"`
	MIME     string `json:"mime"`
}

type promptShotRelayRequest struct {
	Path         string
	ContentType  string
	Body         []byte
	ResponseKind string
}

type promptShotSafeResponse struct {
	Status int
	Body   []byte
}

const promptShotContextKey = service.PromptShotCompatContextKey

const promptShotResponseKindContextKey = "promptshot_response_kind"

// PromptShotAuthValidate intentionally returns product labels rather than
// token, model, provider, or upstream information.
func PromptShotAuthValidate(c *gin.Context) {
	c.JSON(200, gin.H{
		"account_label": "已验证",
		"plan_label":    "当前套餐",
		"quota_label":   "额度按实际使用结算",
		"price_note":    "价格以服务端配置为准",
		"privacy_note":  "图片仅用于本次处理",
		"model_note":    "模型由服务端自动选择",
	})
}

func PromptShotPrepareReverse() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request promptShotReverseRequest
		if err := common.UnmarshalBodyReusable(c, &request); err != nil {
			promptShotRelayError(c, system_setting.PromptShotOperationReverse, err)
			return
		}
		promptShotPrepare(c, system_setting.PromptShotOperationReverse, request)
	}
}

func PromptShotPrepareGenerate() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request promptShotGenerateRequest
		if err := common.UnmarshalBodyReusable(c, &request); err != nil {
			promptShotRelayError(c, system_setting.PromptShotOperationGenerate, err)
			return
		}
		operation := system_setting.PromptShotOperationGenerate
		if strings.TrimSpace(request.RefImageB64) != "" {
			operation = system_setting.PromptShotOperationEdit
		}
		promptShotPrepare(c, operation, request)
	}
}

func PromptShotPrepareClean() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request promptShotCleanRequest
		if err := common.UnmarshalBodyReusable(c, &request); err != nil {
			promptShotRelayError(c, system_setting.PromptShotOperationClean, err)
			return
		}
		if request.Quality != "medium" {
			promptShotRelayError(c, system_setting.PromptShotOperationClean, errPromptShotInvalidQuality)
			return
		}
		promptShotPrepare(c, system_setting.PromptShotOperationClean, request)
	}
}

func promptShotPrepare(c *gin.Context, operation system_setting.PromptShotOperation, payload any) {
	selection, err := selectPromptShotModel(c, operation)
	if err != nil {
		promptShotRelayError(c, operation, err)
		return
	}
	request, err := buildPromptShotRelayRequest(operation, selection, payload)
	if err != nil {
		promptShotRelayError(c, operation, err)
		return
	}
	if err := promptShotReplaceRequestBody(c, request); err != nil {
		promptShotRelayError(c, operation, err)
		return
	}
	c.Set(promptShotContextKey, true)
	c.Set(promptShotResponseKindContextKey, request.ResponseKind)
	c.Request = c.Request.WithContext(service.WithPromptShotRequestContext(c.Request.Context()))
	common.SetContextKey(c, constant.ContextKeyTokenSpecificChannelId, strconv.Itoa(selection.ChannelID))
}

func PromptShotResponseAdapter() gin.HandlerFunc {
	return func(c *gin.Context) {
		originalWriter := c.Writer
		bufferedWriter := &promptShotResponseWriter{ResponseWriter: originalWriter}
		c.Writer = bufferedWriter
		c.Next()
		c.Writer = originalWriter
		promptShotFlushResponse(c, originalWriter, bufferedWriter, c.GetString(promptShotResponseKindContextKey))
	}
}

func PromptShotRelay(c *gin.Context) {
	switch c.Request.URL.Path {
	case "/v1/chat/completions":
		Relay(c, types.RelayFormatOpenAI)
	case "/v1/responses":
		Relay(c, types.RelayFormatOpenAIResponses)
	default:
		Relay(c, types.RelayFormatOpenAIImage)
	}
}

func selectPromptShotModel(c *gin.Context, operation system_setting.PromptShotOperation) (*service.PromptShotModelSelection, error) {
	if _, selectedByClient := common.GetContextKey(c, constant.ContextKeyTokenSpecificChannelId); selectedByClient {
		return nil, service.ErrPromptShotGroupUnavailable
	}
	modelLimitsValue, _ := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
	modelLimits, _ := modelLimitsValue.(map[string]bool)
	selection, err := service.SelectPromptShotModel(
		service.PromptShotSelectionRequest{
			Group:                   common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
			TokenModelLimitsEnabled: common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled),
			TokenModelLimits:        modelLimits,
		},
		operation,
		system_setting.GetPromptShotSetting(),
		promptShotChannelCapabilityResolver{},
	)
	if err != nil {
		return nil, err
	}
	return selection, nil
}

type promptShotChannelCapabilityResolver struct{}

func (promptShotChannelCapabilityResolver) IsAvailable(group, modelName, requestPath string, channelID int) (bool, error) {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return false, err
	}
	if channel == nil || channel.Status != common.ChannelStatusEnabled || !model.IsChannelEnabledForGroupModel(group, modelName, channelID) {
		return false, nil
	}
	if channel.Type != constant.ChannelTypeAdvancedCustom {
		return true, nil
	}
	advancedCustom := channel.GetOtherSettings().AdvancedCustom
	return advancedCustom != nil && advancedCustom.SupportsPathForModel(requestPath, modelName), nil
}

func promptShotReplaceRequestBody(c *gin.Context, request *promptShotRelayRequest) error {
	if request == nil || len(request.Body) == 0 || request.Path == "" || request.ContentType == "" {
		return errPromptShotInvalidRequest
	}
	common.CleanupBodyStorage(c)
	storage, err := common.CreateBodyStorage(request.Body)
	if err != nil {
		return err
	}
	c.Set(common.KeyBodyStorage, storage)
	c.Request.Body = io.NopCloser(storage)
	c.Request.ContentLength = int64(len(request.Body))
	c.Request.Header.Set("Content-Type", request.ContentType)
	c.Request.URL.Path = request.Path
	c.Request.URL.RawPath = ""
	c.Request.RequestURI = request.Path
	return nil
}

func promptShotRelayError(c *gin.Context, operation system_setting.PromptShotOperation, err error) {
	status := http.StatusBadRequest
	message := "请求参数无效"
	if errors.Is(err, service.ErrPromptShotNoConfiguredModel) || errors.Is(err, service.ErrPromptShotNoAvailableCapability) || errors.Is(err, service.ErrPromptShotCapabilityCheckFailed) || errors.Is(err, service.ErrPromptShotCapabilityResolverUnavailable) {
		status = http.StatusUnprocessableEntity
		message = "当前 Token 无可用此功能"
	}
	if errors.Is(err, service.ErrPromptShotTokenModelForbidden) || errors.Is(err, service.ErrPromptShotGroupUnavailable) {
		status = http.StatusForbidden
		message = "当前 Token 无权使用此功能"
	}
	if operation.Canonical() == system_setting.PromptShotOperationEdit && (status == http.StatusUnprocessableEntity || status == http.StatusForbidden) {
		status = http.StatusUnprocessableEntity
		message = "当前 Token 无可用参考图编辑能力"
	}
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"message": message}})
}

type promptShotResponseWriter struct {
	gin.ResponseWriter
	body   bytes.Buffer
	status int
}

func (w *promptShotResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *promptShotResponseWriter) WriteHeaderNow() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
}

func (w *promptShotResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}

func (w *promptShotResponseWriter) WriteString(value string) (int, error) {
	return w.Write([]byte(value))
}

func (w *promptShotResponseWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *promptShotResponseWriter) Size() int {
	return w.body.Len()
}

func (w *promptShotResponseWriter) Written() bool {
	return w.status != 0
}

func (w *promptShotResponseWriter) Flush() {}

func (w *promptShotResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.ResponseWriter.Hijack()
}

func (w *promptShotResponseWriter) CloseNotify() <-chan bool {
	return w.ResponseWriter.CloseNotify()
}

func (w *promptShotResponseWriter) Pusher() http.Pusher {
	return w.ResponseWriter.Pusher()
}

func promptShotFlushResponse(c *gin.Context, original gin.ResponseWriter, buffered *promptShotResponseWriter, kind string) {
	status := buffered.Status()
	response := buffered.body.Bytes()
	if status >= http.StatusBadRequest {
		safe, err := promptShotSafeErrorResponse(status, response)
		if err == nil {
			status = safe.Status
			response = safe.Body
		}
	} else {
		normalized, err := normalizePromptShotResponse(kind, response)
		if err != nil {
			safe, safeErr := promptShotSafeErrorResponse(http.StatusBadGateway, nil)
			if safeErr == nil {
				status = safe.Status
				response = safe.Body
			}
		} else {
			response = normalized
		}
	}
	original.Header().Del("Content-Length")
	original.Header().Set("Content-Type", "application/json; charset=utf-8")
	original.WriteHeader(status)
	_, _ = original.Write(response)
}

func buildPromptShotRelayRequest(
	operation system_setting.PromptShotOperation,
	selection *service.PromptShotModelSelection,
	payload any,
) (*promptShotRelayRequest, error) {
	if selection == nil || selection.Model == "" || selection.RequestPath == "" {
		return nil, errPromptShotInvalidRequest
	}

	switch operation {
	case system_setting.PromptShotOperationReverse:
		request, ok := payload.(promptShotReverseRequest)
		if !ok {
			return nil, errPromptShotInvalidRequest
		}
		image, mimeType, err := promptShotDecodeImage(request.ImageB64, request.MIME)
		if err != nil {
			return nil, err
		}
		_ = image
		if !operation.SupportsRequestPath(selection.RequestPath) {
			return nil, errPromptShotInvalidRequest
		}
		imageURL := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(image)
		if selection.RequestPath == "/v1/responses" {
			body, err := common.Marshal(map[string]any{
				"model":  selection.Model,
				"stream": false,
				"input": []any{map[string]any{
					"role": "user",
					"content": []any{
						map[string]any{"type": "input_text", "text": promptShotReverseInstruction},
						map[string]any{"type": "input_image", "image_url": imageURL},
					},
				}},
			})
			if err != nil {
				return nil, err
			}
			return &promptShotRelayRequest{Path: selection.RequestPath, ContentType: gin.MIMEJSON, Body: body, ResponseKind: "reverse"}, nil
		}
		body, err := common.Marshal(map[string]any{
			"model":  selection.Model,
			"stream": false,
			"messages": []any{map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": promptShotReverseInstruction},
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": imageURL}},
				},
			}},
		})
		if err != nil {
			return nil, err
		}
		return &promptShotRelayRequest{Path: selection.RequestPath, ContentType: gin.MIMEJSON, Body: body, ResponseKind: "reverse"}, nil

	case system_setting.PromptShotOperationGenerate:
		request, ok := payload.(promptShotGenerateRequest)
		if !ok || strings.TrimSpace(request.Prompt) == "" || selection.RequestPath != "/v1/images/generations" {
			return nil, errPromptShotInvalidRequest
		}
		body, err := common.Marshal(map[string]any{
			"model":           selection.Model,
			"prompt":          request.Prompt,
			"n":               1,
			"response_format": "b64_json",
		})
		if err != nil {
			return nil, err
		}
		return &promptShotRelayRequest{Path: selection.RequestPath, ContentType: gin.MIMEJSON, Body: body, ResponseKind: "image"}, nil

	case system_setting.PromptShotOperationEdit:
		request, ok := payload.(promptShotGenerateRequest)
		if !ok {
			return nil, errPromptShotInvalidRequest
		}
		return buildPromptShotImageEditRequest(selection, request.Prompt, request.RefImageB64, request.RefMIME)

	case system_setting.PromptShotOperationClean:
		request, ok := payload.(promptShotCleanRequest)
		if !ok || request.Quality != "medium" {
			return nil, errPromptShotInvalidQuality
		}
		return buildPromptShotImageEditRequest(selection, promptShotCleanInstruction, request.ImageB64, request.MIME)
	default:
		return nil, errPromptShotInvalidRequest
	}
}

func buildPromptShotImageEditRequest(selection *service.PromptShotModelSelection, prompt, imageB64, imageMIME string) (*promptShotRelayRequest, error) {
	if selection == nil || selection.Model == "" || selection.RequestPath != "/v1/images/edits" || strings.TrimSpace(prompt) == "" {
		return nil, errPromptShotInvalidRequest
	}
	image, mimeType, err := promptShotDecodeImage(imageB64, imageMIME)
	if err != nil {
		return nil, err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range map[string]string{
		"model":           selection.Model,
		"prompt":          prompt,
		"n":               "1",
		"response_format": "b64_json",
	} {
		if err := writer.WriteField(key, value); err != nil {
			return nil, err
		}
	}
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="image"; filename="reference%s"`, promptShotImageExtension(mimeType))},
		"Content-Type":        {mimeType},
	})
	if err != nil {
		return nil, err
	}
	if _, err = part.Write(image); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	return &promptShotRelayRequest{Path: selection.RequestPath, ContentType: writer.FormDataContentType(), Body: body.Bytes(), ResponseKind: "image"}, nil
}

func promptShotDecodeImage(imageB64, mimeType string) ([]byte, string, error) {
	imageB64 = strings.TrimSpace(imageB64)
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if imageB64 == "" || strings.HasPrefix(imageB64, "data:") || !promptShotSupportedMIME(mimeType) {
		return nil, "", errPromptShotInvalidImage
	}
	decoded, err := base64.StdEncoding.DecodeString(imageB64)
	if err != nil || len(decoded) == 0 || len(decoded) > promptShotMaxImageBytes {
		return nil, "", errPromptShotInvalidImage
	}
	return decoded, mimeType, nil
}

func promptShotSupportedMIME(mimeType string) bool {
	switch mimeType {
	case "image/png", "image/jpeg", "image/webp":
		return true
	default:
		return false
	}
}

func promptShotImageExtension(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}

func normalizePromptShotResponse(kind string, body []byte) ([]byte, error) {
	switch kind {
	case "image":
		return normalizePromptShotImageResponse(body)
	case "reverse":
		return normalizePromptShotReverseResponse(body)
	default:
		return nil, errPromptShotInvalidRequest
	}
}

func normalizePromptShotImageResponse(body []byte) ([]byte, error) {
	b64 := strings.TrimSpace(gjson.GetBytes(body, "data.0.b64_json").String())
	if b64 == "" || strings.HasPrefix(b64, "data:") {
		return nil, errPromptShotImageNotBase64
	}
	image, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, errPromptShotImageNotBase64
	}
	response := map[string]any{
		"image_b64": b64,
		"mime":      promptShotResponseMIME(image),
	}
	if usage := gjson.GetBytes(body, "usage"); usage.Exists() {
		var value any
		if err := common.Unmarshal([]byte(usage.Raw), &value); err == nil {
			response["usage"] = value
		}
	}
	return common.Marshal(response)
}

func promptShotResponseMIME(image []byte) string {
	switch {
	case len(image) >= 8 && bytes.Equal(image[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}):
		return "image/png"
	case len(image) >= 3 && image[0] == 0xff && image[1] == 0xd8 && image[2] == 0xff:
		return "image/jpeg"
	case len(image) >= 12 && bytes.Equal(image[:4], []byte("RIFF")) && bytes.Equal(image[8:12], []byte("WEBP")):
		return "image/webp"
	default:
		return "image/png"
	}
}

func normalizePromptShotReverseResponse(body []byte) ([]byte, error) {
	content := strings.TrimSpace(gjson.GetBytes(body, "choices.0.message.content").String())
	if content == "" {
		content = strings.TrimSpace(gjson.GetBytes(body, "output.0.content.0.text").String())
	}
	if content == "" {
		return nil, errPromptShotInvalidReverseResponse
	}
	var result struct {
		ZH   string         `json:"zh"`
		EN   string         `json:"en"`
		JSON map[string]any `json:"json"`
	}
	if err := common.Unmarshal([]byte(content), &result); err != nil || strings.TrimSpace(result.ZH) == "" || strings.TrimSpace(result.EN) == "" || !promptShotReverseJSONComplete(result.JSON) {
		return nil, errPromptShotInvalidReverseResponse
	}
	response := map[string]any{"zh": result.ZH, "en": result.EN, "json": result.JSON}
	if usage := gjson.GetBytes(body, "usage"); usage.Exists() {
		var value any
		if err := common.Unmarshal([]byte(usage.Raw), &value); err == nil {
			response["usage"] = value
		}
	}
	return common.Marshal(response)
}

func promptShotReverseJSONComplete(value map[string]any) bool {
	for _, key := range []string{"subject", "composition", "style", "lighting", "colors", "materials", "mood"} {
		text, ok := value[key].(string)
		if !ok || strings.TrimSpace(text) == "" {
			return false
		}
	}
	return true
}

func promptShotSafeErrorResponse(status int, _ []byte) (*promptShotSafeResponse, error) {
	message := "请求处理失败，请稍后重试"
	switch status {
	case http.StatusBadRequest:
		message = "请求参数无效"
	case http.StatusUnauthorized:
		message = "认证失败"
	case http.StatusForbidden:
		message = "当前 Token 无权使用此功能"
	case http.StatusUnprocessableEntity:
		message = "当前 Token 无可用参考图编辑能力"
	case http.StatusTooManyRequests:
		message = "请求过于频繁，请稍后重试"
	default:
		if status < http.StatusBadRequest || status >= http.StatusInternalServerError {
			status = http.StatusBadGateway
			message = "上游服务暂时不可用，请稍后重试"
		}
	}
	body, err := common.Marshal(map[string]any{"error": map[string]string{"message": message}})
	if err != nil {
		return nil, fmt.Errorf("encode promptshot safe error: %w", err)
	}
	return &promptShotSafeResponse{Status: status, Body: body}, nil
}

func promptShotRelayLogMessage(c *gin.Context, err error) string {
	if c != nil && c.GetBool(promptShotContextKey) {
		return "promptshot relay request failed"
	}
	if err == nil {
		return ""
	}
	return common.LocalLogPreview(err.Error())
}

const promptShotReverseInstruction = "Analyze this image and return JSON only with zh, en, and json. json must contain subject, composition, style, lighting, colors, materials, and mood."

const promptShotCleanInstruction = "Remove text, watermarks, logos, and artifacts while preserving the image composition."
