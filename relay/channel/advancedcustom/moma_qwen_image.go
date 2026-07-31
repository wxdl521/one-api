package advancedcustom

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/the-one/common"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/QuantumNous/the-one/relaykit/dto"
	"github.com/QuantumNous/the-one/relaykit/types"
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
)

const momaQwenImageModel = "qwen/qwen-image-2.0-pro"

type momaQwenImageRequest struct {
	Model      string                  `json:"model"`
	Input      momaQwenImageInput      `json:"input"`
	Parameters momaQwenImageParameters `json:"parameters,omitempty"`
}

type momaQwenImageInput struct {
	Messages []momaQwenImageMessage `json:"messages"`
}

type momaQwenImageMessage struct {
	Role    string                 `json:"role"`
	Content []momaQwenImageContent `json:"content"`
}

type momaQwenImageContent struct {
	Text string `json:"text,omitempty"`
}

type momaQwenImageParameters struct {
	N         *uint  `json:"n,omitempty"`
	Size      string `json:"size,omitempty"`
	Watermark *bool  `json:"watermark,omitempty"`
}

type momaQwenImageResponse struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Output  struct {
		Code    string `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
		Results []struct {
			URL      string `json:"url,omitempty"`
			B64Image string `json:"b64_image,omitempty"`
		} `json:"results,omitempty"`
		Choices []struct {
			Message struct {
				Content []struct {
					Image    string `json:"image,omitempty"`
					ImageURL string `json:"image_url,omitempty"`
					B64Image string `json:"b64_image,omitempty"`
					Text     string `json:"text,omitempty"`
				} `json:"content,omitempty"`
			} `json:"message,omitempty"`
		} `json:"choices,omitempty"`
	} `json:"output,omitempty"`
}

func convertOpenAIImageToMoMAQwenImage(info *relaycommon.RelayInfo, request dto.ImageRequest) (*momaQwenImageRequest, error) {
	if info == nil || info.OriginModelName != momaQwenImageModel || info.UpstreamModelName != momaQwenImageModel || request.Model != momaQwenImageModel {
		model := request.Model
		if model == "" && info != nil {
			model = info.OriginModelName
		}
		return nil, fmt.Errorf("MoMA Qwen Image converter only supports model %s, got %s", momaQwenImageModel, model)
	}
	if request.Quality != "" {
		return nil, errors.New("quality is not supported by MoMA Qwen Image")
	}
	if request.ResponseFormat != "" {
		return nil, errors.New("response_format is not supported by MoMA Qwen Image")
	}
	for _, field := range []struct {
		name  string
		value []byte
	}{
		{name: "style", value: request.Style},
		{name: "user", value: request.User},
		{name: "extra_fields", value: request.ExtraFields},
		{name: "background", value: request.Background},
		{name: "moderation", value: request.Moderation},
		{name: "output_format", value: request.OutputFormat},
		{name: "output_compression", value: request.OutputCompression},
		{name: "partial_images", value: request.PartialImages},
		{name: "images", value: request.Images},
		{name: "mask", value: request.Mask},
		{name: "input_fidelity", value: request.InputFidelity},
		{name: "watermark_enabled", value: request.WatermarkEnabled},
		{name: "user_id", value: request.UserId},
		{name: "image", value: request.Image},
	} {
		if len(field.value) > 0 && string(field.value) != "null" {
			return nil, fmt.Errorf("%s is not supported by MoMA Qwen Image", field.name)
		}
	}
	if request.Stream != nil && *request.Stream {
		return nil, errors.New("stream is not supported by MoMA Qwen Image")
	}
	for name := range request.Extra {
		return nil, fmt.Errorf("extra field %s is not supported by MoMA Qwen Image", name)
	}

	n := uint(1)
	if request.N != nil {
		if *request.N == 0 || *request.N > dto.MaxImageN {
			return nil, fmt.Errorf("n must be an integer between 1 and %d", dto.MaxImageN)
		}
		n = *request.N
	}

	return &momaQwenImageRequest{
		Model: info.UpstreamModelName,
		Input: momaQwenImageInput{
			Messages: []momaQwenImageMessage{
				{
					Role: "user",
					Content: []momaQwenImageContent{
						{Text: request.Prompt},
					},
				},
			},
		},
		Parameters: momaQwenImageParameters{
			N:         &n,
			Size:      strings.ReplaceAll(request.Size, "x", "*"),
			Watermark: request.Watermark,
		},
	}, nil
}

func doMoMAQwenImageResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.TheOneError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)

	var upstream momaQwenImageResponse
	if err := common.Unmarshal(responseBody, &upstream); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if upstream.Message != "" || upstream.Code != "" || upstream.Output.Message != "" || upstream.Output.Code != "" {
		message := upstream.Message
		if message == "" {
			message = upstream.Output.Message
		}
		if message == "" {
			message = "MoMA Qwen Image request failed"
		}
		return nil, types.NewOpenAIError(errors.New(message), types.ErrorCodeBadResponse, resp.StatusCode)
	}

	imageResponse := dto.ImageResponse{
		Created:  info.StartTime.Unix(),
		Metadata: responseBody,
	}
	for _, result := range upstream.Output.Results {
		imageResponse.Data = append(imageResponse.Data, dto.ImageData{
			Url:     result.URL,
			B64Json: result.B64Image,
		})
	}
	for _, choice := range upstream.Output.Choices {
		var image dto.ImageData
		for _, content := range choice.Message.Content {
			if content.Text != "" {
				image.RevisedPrompt = content.Text
			}
			value := content.Image
			if value == "" {
				value = content.ImageURL
			}
			if value == "" {
				value = content.B64Image
			}
			if value == "" {
				continue
			}
			if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
				image.Url = value
			} else {
				image.B64Json = value
			}
		}
		if image.Url != "" || image.B64Json != "" {
			imageResponse.Data = append(imageResponse.Data, image)
		}
	}
	if len(imageResponse.Data) == 0 {
		return nil, types.NewOpenAIError(errors.New("MoMA Qwen Image response does not contain generated images"), types.ErrorCodeEmptyResponse, http.StatusBadGateway)
	}

	jsonResponse, err := common.Marshal(imageResponse)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	service.IOCopyBytesGracefully(c, resp, jsonResponse)
	return &dto.Usage{}, nil
}
