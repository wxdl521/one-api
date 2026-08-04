package sora

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/the-one/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEstimateBillingClampsSeconds guards the billing invariant that the
// "seconds" OtherRatio (a quota multiplier) stays within
// [1, relaycommon.MaxTaskDurationSeconds] and resolves duration with the same
// field priority as the request validator, even if validation was bypassed.
func TestEstimateBillingClampsSeconds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		req         relaycommon.TaskSubmitReq
		wantSeconds float64
	}{
		{
			name:        "oversized seconds string is clamped",
			req:         relaycommon.TaskSubmitReq{Seconds: "9999999999"},
			wantSeconds: float64(relaycommon.MaxTaskDurationSeconds),
		},
		{
			name:        "oversized duration is clamped",
			req:         relaycommon.TaskSubmitReq{Duration: 9999999999},
			wantSeconds: float64(relaycommon.MaxTaskDurationSeconds),
		},
		{
			name:        "seconds string takes priority over duration like the validator",
			req:         relaycommon.TaskSubmitReq{Seconds: "8", Duration: 12},
			wantSeconds: 8,
		},
		{
			name:        "absent duration falls back to default",
			req:         relaycommon.TaskSubmitReq{},
			wantSeconds: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("task_request", tt.req)

			adaptor := &TaskAdaptor{}
			ratios := adaptor.EstimateBilling(c, &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}})

			require.NotNil(t, ratios)
			assert.Equal(t, tt.wantSeconds, ratios["seconds"])
		})
	}
}
