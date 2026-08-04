package controller

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/service"
	"github.com/QuantumNous/the-one/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildPromptShotRelayRequestUsesImageGenerationsWithoutReferenceImage(t *testing.T) {
	request, err := buildPromptShotRelayRequest(
		system_setting.PromptShotOperationGenerate,
		&service.PromptShotModelSelection{Model: "image-generate", RequestPath: "/v1/images/generations"},
		promptShotGenerateRequest{Prompt: "a calm mountain lake"},
	)

	require.NoError(t, err)
	assert.Equal(t, "/v1/images/generations", request.Path)
	assert.Equal(t, "application/json", request.ContentType)
	assert.Equal(t, "image", request.ResponseKind)
	assert.Equal(t, "image-generate", gjson.GetBytes(request.Body, "model").String())
	assert.Equal(t, "a calm mountain lake", gjson.GetBytes(request.Body, "prompt").String())
	assert.False(t, gjson.GetBytes(request.Body, "image").Exists())
}

func TestBuildPromptShotRelayRequestUsesMultipartEditsForReferenceImage(t *testing.T) {
	request, err := buildPromptShotRelayRequest(
		system_setting.PromptShotOperationEdit,
		&service.PromptShotModelSelection{Model: "image-edit", RequestPath: "/v1/images/edits"},
		promptShotGenerateRequest{
			Prompt:      "preserve the composition",
			RefImageB64: "YWJj",
			RefMIME:     "image/png",
		},
	)

	require.NoError(t, err)
	assert.Equal(t, "/v1/images/edits", request.Path)
	assert.Equal(t, "image", request.ResponseKind)
	mediaType, params, err := mime.ParseMediaType(request.ContentType)
	require.NoError(t, err)
	assert.Equal(t, "multipart/form-data", mediaType)

	form, err := multipart.NewReader(bytes.NewReader(request.Body), params["boundary"]).ReadForm(1024 * 1024)
	require.NoError(t, err)
	assert.Equal(t, "image-edit", form.Value["model"][0])
	assert.Equal(t, "preserve the composition", form.Value["prompt"][0])
	file, err := form.File["image"][0].Open()
	require.NoError(t, err)
	defer file.Close()
	image, err := io.ReadAll(file)
	require.NoError(t, err)
	assert.Equal(t, []byte("abc"), image)
	assert.Equal(t, "image/png", form.File["image"][0].Header.Get("Content-Type"))
}

func TestBuildPromptShotRelayRequestRejectsCleanQualityOutsideMedium(t *testing.T) {
	_, err := buildPromptShotRelayRequest(
		system_setting.PromptShotOperationClean,
		&service.PromptShotModelSelection{Model: "image-edit", RequestPath: "/v1/images/edits"},
		promptShotCleanRequest{ImageB64: "YWJj", MIME: "image/png", Quality: "high"},
	)

	require.ErrorIs(t, err, errPromptShotInvalidQuality)
}

func TestBuildPromptShotRelayRequestBuildsMultipartEditsForCleanImage(t *testing.T) {
	request, err := buildPromptShotRelayRequest(
		system_setting.PromptShotOperationClean,
		&service.PromptShotModelSelection{Model: "image-edit", RequestPath: "/v1/images/edits"},
		promptShotCleanRequest{ImageB64: "YWJj", MIME: "image/png", Quality: "medium"},
	)

	require.NoError(t, err)
	mediaType, params, err := mime.ParseMediaType(request.ContentType)
	require.NoError(t, err)
	assert.Equal(t, "multipart/form-data", mediaType)
	form, err := multipart.NewReader(bytes.NewReader(request.Body), params["boundary"]).ReadForm(1024 * 1024)
	require.NoError(t, err)
	assert.Equal(t, "image-edit", form.Value["model"][0])
	assert.Equal(t, promptShotCleanInstruction, form.Value["prompt"][0])
	require.Len(t, form.File["image"], 1)
}

func TestBuildPromptShotRelayRequestBuildsNonStreamingVisionChat(t *testing.T) {
	request, err := buildPromptShotRelayRequest(
		system_setting.PromptShotOperationReverse,
		&service.PromptShotModelSelection{Model: "vision-model", RequestPath: "/v1/chat/completions"},
		promptShotReverseRequest{ImageB64: "YWJj", MIME: "image/jpeg"},
	)

	require.NoError(t, err)
	assert.Equal(t, "/v1/chat/completions", request.Path)
	assert.Equal(t, "reverse", request.ResponseKind)
	assert.False(t, gjson.GetBytes(request.Body, "stream").Bool())
	assert.Equal(t, "vision-model", gjson.GetBytes(request.Body, "model").String())
	assert.Equal(t, "data:image/jpeg;base64,YWJj", gjson.GetBytes(request.Body, "messages.0.content.1.image_url.url").String())
}

func TestNormalizePromptShotImageResponseRequiresBase64InsteadOfURL(t *testing.T) {
	response, err := normalizePromptShotResponse("image", []byte(`{"data":[{"b64_json":"YWJj"}],"usage":{"total_tokens":3}}`))

	require.NoError(t, err)
	assert.Equal(t, "YWJj", gjson.GetBytes(response, "image_b64").String())
	assert.Equal(t, "image/png", gjson.GetBytes(response, "mime").String())
	assert.True(t, gjson.GetBytes(response, "usage").Exists())

	_, err = normalizePromptShotResponse("image", []byte(`{"data":[{"url":"https://provider.example/image.png"}]}`))
	require.ErrorIs(t, err, errPromptShotImageNotBase64)
}

func TestNormalizePromptShotImageResponseDetectsReturnedImageMIME(t *testing.T) {
	jpeg := base64.StdEncoding.EncodeToString([]byte{0xff, 0xd8, 0xff, 0x00})
	response, err := normalizePromptShotResponse("image", []byte(`{"data":[{"b64_json":"`+jpeg+`"}]}`))

	require.NoError(t, err)
	assert.Equal(t, "image/jpeg", gjson.GetBytes(response, "mime").String())
}

func TestNormalizePromptShotReverseResponseRequiresStructuredOutput(t *testing.T) {
	response, err := normalizePromptShotResponse("reverse", []byte(`{"choices":[{"message":{"content":"{\"zh\":\"中文\",\"en\":\"English\",\"json\":{\"subject\":\"cat\",\"composition\":\"center\",\"style\":\"photo\",\"lighting\":\"soft\",\"colors\":\"blue\",\"materials\":\"fur\",\"mood\":\"calm\"}}"}}]}`))

	require.NoError(t, err)
	assert.Equal(t, "中文", gjson.GetBytes(response, "zh").String())
	assert.Equal(t, "English", gjson.GetBytes(response, "en").String())
	assert.Equal(t, "cat", gjson.GetBytes(response, "json.subject").String())

	_, err = normalizePromptShotResponse("reverse", []byte(`{"choices":[{"message":{"content":"not-json"}}]}`))
	require.ErrorIs(t, err, errPromptShotInvalidReverseResponse)
}

func TestBuildPromptShotRelayRequestDoesNotShareStateAcrossConcurrentVariants(t *testing.T) {
	const variants = 4
	results := make(chan string, variants)
	errors := make(chan error, variants)
	var waitGroup sync.WaitGroup

	for index := 0; index < variants; index++ {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			prompt := "variant-" + string(rune('a'+index))
			request, err := buildPromptShotRelayRequest(
				system_setting.PromptShotOperationEdit,
				&service.PromptShotModelSelection{Model: "image-edit", RequestPath: "/v1/images/edits"},
				promptShotGenerateRequest{Prompt: prompt, RefImageB64: "YWJj", RefMIME: "image/png"},
			)
			if err != nil {
				errors <- err
				return
			}
			mediaType, params, err := mime.ParseMediaType(request.ContentType)
			if err != nil || mediaType != "multipart/form-data" {
				errors <- err
				return
			}
			form, err := multipart.NewReader(bytes.NewReader(request.Body), params["boundary"]).ReadForm(1024 * 1024)
			if err != nil {
				errors <- err
				return
			}
			results <- form.Value["prompt"][0]
		}(index)
	}
	waitGroup.Wait()
	close(results)
	close(errors)

	for err := range errors {
		require.NoError(t, err)
	}
	got := make(map[string]bool, variants)
	for result := range results {
		got[result] = true
	}
	require.Equal(t, map[string]bool{
		"variant-a": true,
		"variant-b": true,
		"variant-c": true,
		"variant-d": true,
	}, got)
}

func TestBuildPromptShotRelayRequestDoesNotAcceptDataURLOrUnsupportedMIME(t *testing.T) {
	_, err := buildPromptShotRelayRequest(
		system_setting.PromptShotOperationEdit,
		&service.PromptShotModelSelection{Model: "image-edit", RequestPath: "/v1/images/edits"},
		promptShotGenerateRequest{Prompt: "test", RefImageB64: "data:image/png;base64,YWJj", RefMIME: "image/png"},
	)
	require.ErrorIs(t, err, errPromptShotInvalidImage)

	_, err = buildPromptShotRelayRequest(
		system_setting.PromptShotOperationReverse,
		&service.PromptShotModelSelection{Model: "vision", RequestPath: "/v1/chat/completions"},
		promptShotReverseRequest{ImageB64: "YWJj", MIME: "image/svg+xml"},
	)
	require.ErrorIs(t, err, errPromptShotInvalidImage)
}

func TestNormalizePromptShotResponseDoesNotExposeUpstreamBodyOnFailure(t *testing.T) {
	response, err := promptShotSafeErrorResponse(429, []byte(`{"error":{"message":"provider-key-should-not-appear"}}`))

	require.NoError(t, err)
	assert.Equal(t, 429, response.Status)
	assert.NotContains(t, strings.ToLower(string(response.Body)), "provider")
	assert.NotContains(t, string(response.Body), "provider-key-should-not-appear")

	var decoded map[string]any
	require.NoError(t, common.Unmarshal(response.Body, &decoded))
	assert.NotEmpty(t, gjson.GetBytes(response.Body, "error.message").String())
}
