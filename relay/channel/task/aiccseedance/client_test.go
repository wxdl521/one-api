package aiccseedance

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/the-one/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSeedanceModelUsesBearerAuthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodPost, request.Method)
		assert.Equal(t, "/mapping/query", request.URL.Path)
		assert.Equal(t, "Bearer api-key", request.Header.Get("Authorization"))

		body, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		var payload map[string]string
		require.NoError(t, common.Unmarshal(body, &payload))
		assert.Equal(t, ModelName, payload["model"])

		writer.Header().Set("Content-Type", "application/json")
		_, err = writer.Write([]byte(`{"endpoint":"ep-seedance-2-0"}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	endpoint, err := resolveSeedanceModel(server.URL, "api-key", ModelName)

	require.NoError(t, err)
	assert.Equal(t, "ep-seedance-2-0", endpoint)
}

func TestResolveSeedanceModelRejectsMappingFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, err := writer.Write([]byte(`{"message":"invalid key"}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	_, err := resolveSeedanceModel(server.URL, "api-key", ModelName)

	require.Error(t, err)
}
