package aiccseedance

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/the-one/common"
	maasseedance "maas_seedance_sdk_1.0.0_go"
)

type seedanceClient interface {
	CreateVideoGenerationTask(data map[string]any) (string, error)
	QueryVideoGenerationTask(taskID string) (map[string]any, error)
}

var newSeedanceClient = newOfficialSeedanceClient

func newOfficialSeedanceClient(baseURL, apiKey, model string) (client seedanceClient, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("initialize AICC Seedance client: %v", recovered)
		}
	}()

	// Keep the explicit lookup as a fail-fast configuration check. The official
	// client performs this lookup again internally and writes the resolved
	// endpoint into the encrypted task request. Passing the endpoint here would
	// make it look up an endpoint as though it were a virtual model name.
	if _, err = resolveSeedanceModel(baseURL, apiKey, model); err != nil {
		return nil, err
	}
	return maasseedance.NewMaasSeedanceClient(baseURL, apiKey, model, false)
}

func resolveSeedanceModel(baseURL, apiKey, model string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || strings.TrimSpace(apiKey) == "" || strings.TrimSpace(model) == "" {
		return "", fmt.Errorf("AICC Seedance base URL, API key, and model are required")
	}

	body, err := common.Marshal(map[string]string{"model": model})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/mapping/query", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("AICC Seedance mapping query failed with status %d", resp.StatusCode)
	}

	var result struct {
		Endpoint string `json:"endpoint"`
	}
	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Endpoint) == "" {
		return "", fmt.Errorf("AICC Seedance mapping query returned an empty endpoint")
	}
	return result.Endpoint, nil
}
