package siliconflow

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/QuantumNous/the-one/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertImageRequestBoundsBatchSize guards the billing invariant that the
// Extra-passthrough batch_size (which bypasses the standard n validation in
// relay/helper) cannot exceed dto.MaxImageN, and that explicit zero values in
// passthrough fields are preserved.
func TestConvertImageRequestBoundsBatchSize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newImageRequest := func(t *testing.T, body string) dto.ImageRequest {
		var req dto.ImageRequest
		require.NoError(t, common.Unmarshal([]byte(body), &req))
		return req
	}

	adaptor := &Adaptor{}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	t.Run("batch_size above bound is rejected", func(t *testing.T) {
		req := newImageRequest(t, fmt.Sprintf(`{"model":"m","prompt":"p","batch_size":%d}`, dto.MaxImageN+1))
		_, err := adaptor.ConvertImageRequest(c, nil, req)
		require.Error(t, err)
	})

	t.Run("batch_size at bound is accepted", func(t *testing.T) {
		req := newImageRequest(t, fmt.Sprintf(`{"model":"m","prompt":"p","batch_size":%d}`, dto.MaxImageN))
		converted, err := adaptor.ConvertImageRequest(c, nil, req)
		require.NoError(t, err)
		sfReq, ok := converted.(*SFImageRequest)
		require.True(t, ok)
		assert.Equal(t, uint(dto.MaxImageN), sfReq.BatchSize)
	})

	t.Run("standard n fallback is used when batch_size absent", func(t *testing.T) {
		req := newImageRequest(t, `{"model":"m","prompt":"p","n":2}`)
		converted, err := adaptor.ConvertImageRequest(c, nil, req)
		require.NoError(t, err)
		sfReq, ok := converted.(*SFImageRequest)
		require.True(t, ok)
		assert.Equal(t, uint(2), sfReq.BatchSize)
	})

	t.Run("explicit zero guidance_scale is preserved", func(t *testing.T) {
		req := newImageRequest(t, `{"model":"m","prompt":"p","guidance_scale":0}`)
		converted, err := adaptor.ConvertImageRequest(c, nil, req)
		require.NoError(t, err)
		sfReq, ok := converted.(*SFImageRequest)
		require.True(t, ok)
		require.NotNil(t, sfReq.GuidanceScale)
		assert.Equal(t, 0.0, *sfReq.GuidanceScale)
	})
}
