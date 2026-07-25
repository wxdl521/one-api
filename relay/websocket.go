package relay

import (
	"fmt"

	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/QuantumNous/the-one/relaykit/dto"
	"github.com/QuantumNous/the-one/relaykit/types"
	"github.com/QuantumNous/the-one/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func WssHelper(c *gin.Context, info *relaycommon.RelayInfo) (theOneError *types.TheOneError) {
	info.InitChannelMeta(c)

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
	//var requestBody io.Reader
	//firstWssRequest, _ := c.Get("first_wss_request")
	//requestBody = bytes.NewBuffer(firstWssRequest.([]byte))

	statusCodeMappingStr := c.GetString("status_code_mapping")
	resp, err := adaptor.DoRequest(c, info, nil)
	if err != nil {
		return types.NewError(err, types.ErrorCodeDoRequestFailed)
	}

	if resp != nil {
		info.TargetWs = resp.(*websocket.Conn)
		defer info.TargetWs.Close()
	}

	usage, theOneError := adaptor.DoResponse(c, nil, info)
	if theOneError != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(theOneError, statusCodeMappingStr)
		return theOneError
	}
	service.PostWssConsumeQuota(c, info, info.UpstreamModelName, usage.(*dto.RealtimeUsage), "")
	return nil
}
