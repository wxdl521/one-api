package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/constant"
	"github.com/QuantumNous/the-one/model"
	"github.com/QuantumNous/the-one/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pricingResponse struct {
	Success       bool                        `json:"success"`
	ChannelGroups []model.PricingChannelGroup `json:"channel_groups"`
}

func TestGetPricingReturnsVisibleChannelGroups(t *testing.T) {
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
	})

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&[]model.Channel{
		{Id: 1, Type: constant.ChannelTypeOpenAI, Key: "key-1", Status: common.ChannelStatusEnabled, Name: "Volcengine"},
		{Id: 2, Type: constant.ChannelTypeOpenAI, Key: "key-2", Status: common.ChannelStatusEnabled, Name: "Mobile"},
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "shared-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "volcengine-model", ChannelId: 1, Enabled: true},
		{Group: "vip", Model: "mobile-model", ChannelId: 2, Enabled: true},
	}).Error)
	model.InvalidatePricingCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)

	GetPricing(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload pricingResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	assert.Equal(t, []model.PricingChannelGroup{
		{Name: "Volcengine", Models: []string{"shared-model", "volcengine-model"}},
	}, payload.ChannelGroups)
}
