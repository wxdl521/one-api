package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/the-one/common"
)

const (
	agentPlanUsageAction  = "GetAFPUsage"
	agentPlanUsageVersion = "2024-01-01"
	agentPlanUsageRegion  = "cn-north-1"
	agentPlanUsageService = "ark"
)

type AgentPlanUsageWindow struct {
	Remaining float64 `json:"remaining"`
	ResetAt   int64   `json:"reset_at"`
}

type AgentPlanUsage struct {
	FiveHour AgentPlanUsageWindow `json:"five_hour"`
	Weekly   AgentPlanUsageWindow `json:"weekly"`
	Monthly  AgentPlanUsageWindow `json:"monthly"`
}

type AgentPlanUsageFetcher func(context.Context, *http.Client, string, string) (AgentPlanUsage, error)

type agentPlanUsageCacheEntry struct {
	Usage     AgentPlanUsage
	UpdatedAt time.Time
}

type AgentPlanUsageCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	fetch   AgentPlanUsageFetcher
	entries map[int]agentPlanUsageCacheEntry
}

func NewAgentPlanUsageCache(ttl time.Duration, now func() time.Time, fetch AgentPlanUsageFetcher) *AgentPlanUsageCache {
	return &AgentPlanUsageCache{
		ttl:     ttl,
		now:     now,
		fetch:   fetch,
		entries: make(map[int]agentPlanUsageCacheEntry),
	}
}

func (c *AgentPlanUsageCache) Fetch(ctx context.Context, channelID int, client *http.Client, endpoint, apiKey string) (usage AgentPlanUsage, updatedAt int64, stale bool, err error) {
	c.mu.Lock()
	now := c.now()
	if entry, ok := c.entries[channelID]; ok && now.Sub(entry.UpdatedAt) < c.ttl {
		c.mu.Unlock()
		return entry.Usage, entry.UpdatedAt.Unix(), false, nil
	}
	c.mu.Unlock()

	usage, err = c.fetch(ctx, client, endpoint, apiKey)
	if err == nil {
		updatedAtTime := c.now()
		c.mu.Lock()
		c.entries[channelID] = agentPlanUsageCacheEntry{Usage: usage, UpdatedAt: updatedAtTime}
		c.mu.Unlock()
		return usage, updatedAtTime.Unix(), false, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.entries[channelID]; ok {
		return entry.Usage, entry.UpdatedAt.Unix(), true, nil
	}
	return AgentPlanUsage{}, 0, false, err
}

type agentPlanUsageResponse struct {
	Result struct {
		AFPFiveHour agentPlanUsageUpstreamWindow `json:"AFPFiveHour"`
		AFPWeekly   agentPlanUsageUpstreamWindow `json:"AFPWeekly"`
		AFPMonthly  agentPlanUsageUpstreamWindow `json:"AFPMonthly"`
	} `json:"Result"`
}

type agentPlanUsageUpstreamWindow struct {
	Quota     float64 `json:"Quota"`
	Used      float64 `json:"Used"`
	ResetTime int64   `json:"ResetTime"`
}

func FetchAgentPlanUsage(ctx context.Context, client *http.Client, endpoint, apiKey string) (AgentPlanUsage, error) {
	if client == nil {
		return AgentPlanUsage{}, fmt.Errorf("agent plan usage client is required")
	}
	if strings.TrimSpace(apiKey) == "" {
		return AgentPlanUsage{}, fmt.Errorf("agent plan API key is required")
	}

	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return AgentPlanUsage{}, fmt.Errorf("parse agent plan usage endpoint: %w", err)
	}
	query := requestURL.Query()
	query.Set("Action", agentPlanUsageAction)
	query.Set("Version", agentPlanUsageVersion)
	requestURL.RawQuery = query.Encode()

	body := []byte("{}")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), bytes.NewReader(body))
	if err != nil {
		return AgentPlanUsage{}, fmt.Errorf("create agent plan usage request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if err := signAgentPlanUsageRequest(req, body, apiKey, time.Now().UTC()); err != nil {
		return AgentPlanUsage{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return AgentPlanUsage{}, fmt.Errorf("request agent plan usage: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return AgentPlanUsage{}, fmt.Errorf("agent plan usage upstream returned status %d", resp.StatusCode)
	}

	payload := agentPlanUsageResponse{}
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return AgentPlanUsage{}, fmt.Errorf("decode agent plan usage response: %w", err)
	}

	fiveHour, err := normalizeAgentPlanUsageWindow(payload.Result.AFPFiveHour)
	if err != nil {
		return AgentPlanUsage{}, fmt.Errorf("normalize five-hour agent plan usage: %w", err)
	}
	weekly, err := normalizeAgentPlanUsageWindow(payload.Result.AFPWeekly)
	if err != nil {
		return AgentPlanUsage{}, fmt.Errorf("normalize weekly agent plan usage: %w", err)
	}
	monthly, err := normalizeAgentPlanUsageWindow(payload.Result.AFPMonthly)
	if err != nil {
		return AgentPlanUsage{}, fmt.Errorf("normalize monthly agent plan usage: %w", err)
	}

	return AgentPlanUsage{FiveHour: fiveHour, Weekly: weekly, Monthly: monthly}, nil
}

func signAgentPlanUsageRequest(req *http.Request, body []byte, credentials string, now time.Time) error {
	credentialParts := strings.Split(strings.TrimSpace(credentials), "|")
	if len(credentialParts) != 2 || strings.TrimSpace(credentialParts[0]) == "" || strings.TrimSpace(credentialParts[1]) == "" {
		return fmt.Errorf("agent plan credentials must use the AccessKey|SecretKey format")
	}
	accessKey := strings.TrimSpace(credentialParts[0])
	secretKey := strings.TrimSpace(credentialParts[1])
	payloadHash := sha256.Sum256(body)
	hexPayloadHash := hex.EncodeToString(payloadHash[:])
	xDate := now.Format("20060102T150405Z")
	shortDate := now.Format("20060102")
	host := req.URL.Host

	req.Host = host
	req.Header.Set("X-Date", xDate)
	req.Header.Set("X-Content-Sha256", hexPayloadHash)

	query := req.URL.Query()
	queryKeys := make([]string, 0, len(query))
	for key := range query {
		queryKeys = append(queryKeys, key)
	}
	sort.Strings(queryKeys)
	queryParts := make([]string, 0, len(query))
	for _, key := range queryKeys {
		values := query[key]
		sort.Strings(values)
		for _, value := range values {
			queryParts = append(queryParts, url.QueryEscape(key)+"="+url.QueryEscape(value))
		}
	}

	headersToSign := map[string]string{
		"content-type":     req.Header.Get("Content-Type"),
		"host":             host,
		"x-content-sha256": hexPayloadHash,
		"x-date":           xDate,
	}
	headerNames := make([]string, 0, len(headersToSign))
	for name := range headersToSign {
		headerNames = append(headerNames, name)
	}
	sort.Strings(headerNames)
	canonicalHeaders := strings.Builder{}
	for _, name := range headerNames {
		canonicalHeaders.WriteString(name)
		canonicalHeaders.WriteString(":")
		canonicalHeaders.WriteString(strings.TrimSpace(headersToSign[name]))
		canonicalHeaders.WriteString("\n")
	}
	signedHeaders := strings.Join(headerNames, ";")
	canonicalPath := req.URL.EscapedPath()
	if canonicalPath == "" {
		canonicalPath = "/"
	}
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalPath,
		strings.Join(queryParts, "&"),
		canonicalHeaders.String(),
		signedHeaders,
		hexPayloadHash,
	}, "\n")
	hashedCanonicalRequest := sha256.Sum256([]byte(canonicalRequest))
	credentialScope := strings.Join([]string{shortDate, agentPlanUsageRegion, agentPlanUsageService, "request"}, "/")
	stringToSign := strings.Join([]string{
		"HMAC-SHA256",
		xDate,
		credentialScope,
		hex.EncodeToString(hashedCanonicalRequest[:]),
	}, "\n")

	kDate := agentPlanUsageHMAC([]byte(secretKey), shortDate)
	kRegion := agentPlanUsageHMAC(kDate, agentPlanUsageRegion)
	kService := agentPlanUsageHMAC(kRegion, agentPlanUsageService)
	kSigning := agentPlanUsageHMAC(kService, "request")
	signature := hex.EncodeToString(agentPlanUsageHMAC(kSigning, stringToSign))
	req.Header.Set("Authorization", fmt.Sprintf("HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", accessKey, credentialScope, signedHeaders, signature))
	return nil
}

func agentPlanUsageHMAC(key []byte, data string) []byte {
	hash := hmac.New(sha256.New, key)
	_, _ = hash.Write([]byte(data))
	return hash.Sum(nil)
}

func normalizeAgentPlanUsageWindow(window agentPlanUsageUpstreamWindow) (AgentPlanUsageWindow, error) {
	if math.IsNaN(window.Quota) || math.IsInf(window.Quota, 0) || math.IsNaN(window.Used) || math.IsInf(window.Used, 0) {
		return AgentPlanUsageWindow{}, fmt.Errorf("quota and used values must be finite")
	}
	return AgentPlanUsageWindow{
		Remaining: math.Max(0, window.Quota-window.Used),
		ResetAt:   window.ResetTime,
	}, nil
}
