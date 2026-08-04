package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMiniAppAllowedModelSetDefaultsToDenyAndUsesExactNames(t *testing.T) {
	defaultDeny := miniAppAllowedModelSet("")
	assert.Empty(t, defaultDeny)

	allowed := miniAppAllowedModelSet(" gpt-mini, gpt-4.1-mini ,gpt-mini ")
	assert.Contains(t, allowed, "gpt-mini")
	assert.Contains(t, allowed, "gpt-4.1-mini")
	assert.NotContains(t, allowed, "gpt")
	assert.NotContains(t, allowed, "gpt-4.1")
}
