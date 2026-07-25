package relay

import (
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/logger"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/QuantumNous/the-one/relay/helper"
	"github.com/QuantumNous/the-one/relaykit/dto"
	"github.com/QuantumNous/the-one/relaykit/types"
	"github.com/QuantumNous/the-one/service"

	"github.com/gin-gonic/gin"
)

func EmbeddingHelper(c *gin.Context, info *relaycommon.RelayInfo) (theOneError *types.TheOneError) {
	info.InitChannelMeta(c)

	embeddingReq, ok := info.Request.(*dto.EmbeddingRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected *dto.EmbeddingRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(embeddingReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to EmbeddingRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	convertedRequest, err := adaptor.ConvertEmbeddingRequest(c, info, *request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return theOneErrorFromParamOverride(err)
		}
	}

	logger.LogDebug(c, "converted embedding request body: %s", jsonData)
	body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	jsonData = nil
	info.UpstreamRequestBodySize = size
	var requestBody io.Reader = body
	statusCodeMappingStr := c.GetString("status_code_mapping")
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		if httpResp.StatusCode != http.StatusOK {
			theOneError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			// reset status code 重置状态码
			service.ResetStatusCode(theOneError, statusCodeMappingStr)
			return theOneError
		}
	}

	usage, theOneError := adaptor.DoResponse(c, httpResp, info)
	if theOneError != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(theOneError, statusCodeMappingStr)
		return theOneError
	}
	service.PostTextConsumeQuota(c, info, usage.(*dto.Usage), nil)
	return nil
}
