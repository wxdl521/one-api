package advancedcustom

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/the-one/common"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	relayconstant "github.com/QuantumNous/the-one/relay/constant"
	"github.com/QuantumNous/the-one/relaykit/dto"
	"github.com/QuantumNous/the-one/relaykit/relayconvert"
	"github.com/QuantumNous/the-one/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoMAQwenImageConvertsOpenAIImageGenerationRequest(t *testing.T) {
	adaptor := &Adaptor{}
	info := momaQwenImageRelayInfo()
	c := advancedCustomGinContext("/v1/images/generations")
	watermark := false

	converted, err := adaptor.ConvertImageRequest(c, info, dto.ImageRequest{
		Model:     momaQwenImageModel,
		Prompt:    "a red kite above a lake",
		N:         lo.ToPtr(uint(2)),
		Size:      "1024x1536",
		Watermark: &watermark,
	})
	require.NoError(t, err)

	request, ok := converted.(*momaQwenImageRequest)
	require.True(t, ok)
	assert.Equal(t, momaQwenImageModel, request.Model)
	require.Len(t, request.Input.Messages, 1)
	assert.Equal(t, "user", request.Input.Messages[0].Role)
	require.Len(t, request.Input.Messages[0].Content, 1)
	assert.Equal(t, "a red kite above a lake", request.Input.Messages[0].Content[0].Text)
	require.NotNil(t, request.Parameters.N)
	assert.Equal(t, uint(2), *request.Parameters.N)
	assert.Equal(t, "1024*1536", request.Parameters.Size)
	require.NotNil(t, request.Parameters.Watermark)
	assert.False(t, *request.Parameters.Watermark)

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(
		t,
		"https://fallback.example/v1/aigc/multimodal-generation/generation",
		requestURL,
	)
}

func TestMoMAQwenImageRejectsUnsupportedModelAndOptions(t *testing.T) {
	tests := []struct {
		name    string
		request dto.ImageRequest
		want    string
	}{
		{
			name: "model",
			request: dto.ImageRequest{
				Model:  "qwen/qwen-image-2.0",
				Prompt: "a cat",
			},
			want: "only supports model",
		},
		{
			name: "quality",
			request: dto.ImageRequest{
				Model:   momaQwenImageModel,
				Prompt:  "a cat",
				Quality: "high",
			},
			want: "quality is not supported",
		},
		{
			name: "extra field",
			request: dto.ImageRequest{
				Model:  momaQwenImageModel,
				Prompt: "a cat",
				Extra: map[string]json.RawMessage{
					"prompt_extend": json.RawMessage("true"),
				},
			},
			want: "extra field prompt_extend is not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adaptor := &Adaptor{}
			_, err := adaptor.ConvertImageRequest(advancedCustomGinContext("/v1/images/generations"), momaQwenImageRelayInfo(), tt.request)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestMoMAQwenImageConvertsSuccessfulResponseToOpenAIImageResponse(t *testing.T) {
	adaptor := &Adaptor{}
	info := momaQwenImageRelayInfo()
	info.StartTime = time.Unix(1_700_000_000, 0)
	recorder := httptest.NewRecorder()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)

	body := []byte(`{
		"output": {
			"choices": [
				{"message":{"content":[{"image":"https://images.example/one.png"}]}},
				{"message":{"content":[{"image":"base64-image"}]}}
			]
		},
		"usage": {"image_count": 2}
	}`)
	usage, theOneError := adaptor.DoResponse(c, &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}, info)
	require.Nil(t, theOneError)
	require.NotNil(t, usage)

	var response dto.ImageResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, info.StartTime.Unix(), response.Created)
	require.Len(t, response.Data, 2)
	assert.Equal(t, "https://images.example/one.png", response.Data[0].Url)
	assert.Equal(t, "base64-image", response.Data[1].B64Json)
}

func TestMoMAQwenImageRejectsUnsuccessfulResponse(t *testing.T) {
	adaptor := &Adaptor{}
	info := momaQwenImageRelayInfo()
	c := advancedCustomGinContext("/v1/images/generations")

	_, theOneError := adaptor.DoResponse(c, &http.Response{
		StatusCode: http.StatusBadRequest,
		Body: io.NopCloser(bytes.NewReader([]byte(`{
			"code": "InvalidParameter",
			"message": "size is invalid"
		}`))),
	}, info)
	require.NotNil(t, theOneError)
	assert.Equal(t, types.ErrorCodeBadResponse, theOneError.GetErrorCode())
	assert.Contains(t, theOneError.Error(), "size is invalid")
}

func momaQwenImageRelayInfo() *relaycommon.RelayInfo {
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/images/generations",
				UpstreamPath: "/v1/aigc/multimodal-generation/generation",
				Converter:    relayconvert.ConverterOpenAIImageToMoMAQwenImage,
				Models:       []string{momaQwenImageModel},
			},
		},
	})
	info.RelayFormat = types.RelayFormatOpenAIImage
	info.RelayMode = relayconstant.RelayModeImagesGenerations
	info.RequestURLPath = "/v1/images/generations"
	info.OriginModelName = momaQwenImageModel
	info.UpstreamModelName = momaQwenImageModel
	return info
}
