package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchAgentPlanUsageNormalizesOfficialAFPWindows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "GetAFPUsage", r.URL.Query().Get("Action"))
		assert.Equal(t, "2024-01-01", r.URL.Query().Get("Version"))
		assert.Regexp(t, `^HMAC-SHA256 Credential=agent-plan-access-key/\d{8}/cn-north-1/ark/request, SignedHeaders=content-type;host;x-content-sha256;x-date, Signature=[a-f0-9]{64}$`, r.Header.Get("Authorization"))
		assert.NotEmpty(t, r.Header.Get("X-Date"))
		assert.NotEmpty(t, r.Header.Get("X-Content-Sha256"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{}`, string(body))

		_, _ = w.Write([]byte(`{
			"Result": {
				"AFPFiveHour": {"Quota": 50000, "Used": 0.046, "ResetTime": 1778806800000},
				"AFPWeekly": {"Quota": 175000, "Used": 106.749, "ResetTime": 1779062400000},
				"AFPMonthly": {"Quota": 500000, "Used": 500001, "ResetTime": 1780531200000}
			}
		}`))
	}))
	defer server.Close()

	usage, err := FetchAgentPlanUsage(context.Background(), server.Client(), server.URL, "agent-plan-access-key|agent-plan-secret-key")

	require.NoError(t, err)
	assert.Equal(t, 49999.954, usage.FiveHour.Remaining)
	assert.Equal(t, int64(1778806800000), usage.FiveHour.ResetAt)
	assert.Equal(t, 174893.251, usage.Weekly.Remaining)
	assert.Equal(t, int64(1779062400000), usage.Weekly.ResetAt)
	assert.Zero(t, usage.Monthly.Remaining)
	assert.Equal(t, int64(1780531200000), usage.Monthly.ResetAt)
}

func TestAgentPlanUsageCacheReturnsRecentSnapshotWhenRefreshFails(t *testing.T) {
	now := time.Date(2026, 7, 23, 9, 0, 0, 0, time.UTC)
	fetchCalls := 0
	cache := NewAgentPlanUsageCache(time.Minute, func() time.Time { return now }, func(context.Context, *http.Client, string, string) (AgentPlanUsage, error) {
		fetchCalls++
		if fetchCalls == 1 {
			return AgentPlanUsage{FiveHour: AgentPlanUsageWindow{Remaining: 42}}, nil
		}
		return AgentPlanUsage{}, errors.New("upstream unavailable")
	})

	usage, updatedAt, stale, err := cache.Fetch(context.Background(), 7, http.DefaultClient, "https://example.com", "key")
	require.NoError(t, err)
	assert.False(t, stale)
	assert.Equal(t, now.Unix(), updatedAt)
	assert.Equal(t, 42.0, usage.FiveHour.Remaining)
	assert.Equal(t, 1, fetchCalls)

	now = now.Add(30 * time.Second)
	usage, updatedAt, stale, err = cache.Fetch(context.Background(), 7, http.DefaultClient, "https://example.com", "key")
	require.NoError(t, err)
	assert.False(t, stale)
	assert.Equal(t, time.Date(2026, 7, 23, 9, 0, 0, 0, time.UTC).Unix(), updatedAt)
	assert.Equal(t, 42.0, usage.FiveHour.Remaining)
	assert.Equal(t, 1, fetchCalls)

	now = now.Add(time.Minute + time.Second)
	usage, updatedAt, stale, err = cache.Fetch(context.Background(), 7, http.DefaultClient, "https://example.com", "key")
	require.NoError(t, err)
	assert.True(t, stale)
	assert.Equal(t, time.Date(2026, 7, 23, 9, 0, 0, 0, time.UTC).Unix(), updatedAt)
	assert.Equal(t, 42.0, usage.FiveHour.Remaining)
}
