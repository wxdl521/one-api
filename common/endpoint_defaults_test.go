package common

import (
	"testing"

	"github.com/QuantumNous/the-one/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageEditEndpointDefault(t *testing.T) {
	endpoint, ok := GetDefaultEndpointInfo(constant.EndpointTypeImageEdit)
	require.True(t, ok)
	assert.Equal(t, "/v1/images/edits", endpoint.Path)
	assert.Equal(t, "POST", endpoint.Method)
}
