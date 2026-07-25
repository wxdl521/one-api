package relay

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/QuantumNous/the-one/common"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/QuantumNous/the-one/relay/helper"
	"github.com/QuantumNous/the-one/relaykit/dto"
	"github.com/QuantumNous/the-one/relaykit/types"
	"github.com/QuantumNous/the-one/service"

	"github.com/gin-gonic/gin"
)

func AudioHelper(c *gin.Context, info *relaycommon.RelayInfo) (theOneError *types.TheOneError) {
	info.InitChannelMeta(c)

	audioReq, ok := info.Request.(*dto.AudioRequest)
	if !ok {
		return types.NewError(errors.New("invalid request type"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(audioReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to AudioRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
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

	ioReader, err := adaptor.ConvertAudioRequest(c, info, *request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	resp, err := adaptor.DoRequest(c, info, ioReader)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	statusCodeMappingStr := c.GetString("status_code_mapping")

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
	if usage.(*dto.Usage).CompletionTokenDetails.AudioTokens > 0 || usage.(*dto.Usage).PromptTokensDetails.AudioTokens > 0 {
		service.PostAudioConsumeQuota(c, info, usage.(*dto.Usage), "")
	} else {
		service.PostTextConsumeQuota(c, info, usage.(*dto.Usage), nil)
	}

	return nil
}
