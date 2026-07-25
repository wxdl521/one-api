package controller

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/the-one/middleware"
	"github.com/QuantumNous/the-one/model"
	relaycommon "github.com/QuantumNous/the-one/relay/common"
	"github.com/QuantumNous/the-one/relaykit/types"

	"github.com/gin-gonic/gin"
)

func Playground(c *gin.Context) {
	var theOneError *types.TheOneError

	defer func() {
		if theOneError != nil {
			c.JSON(theOneError.StatusCode, gin.H{
				"error": theOneError.ToOpenAIError(),
			})
		}
	}()

	useAccessToken := c.GetBool("use_access_token")
	if useAccessToken {
		theOneError = types.NewError(errors.New("暂不支持使用 access token"), types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAI, nil, nil)
	if err != nil {
		theOneError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		return
	}

	userId := c.GetInt("id")

	// Write user context to ensure acceptUnsetRatio is available
	userCache, err := model.GetUserCache(userId)
	if err != nil {
		theOneError = types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		return
	}
	userCache.WriteContext(c)

	tempToken := &model.Token{
		UserId: userId,
		Name:   fmt.Sprintf("playground-%s", relayInfo.UsingGroup),
		Group:  relayInfo.UsingGroup,
	}
	_ = middleware.SetupContextForToken(c, tempToken)

	Relay(c, types.RelayFormatOpenAI)
}
