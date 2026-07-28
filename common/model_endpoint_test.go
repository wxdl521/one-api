package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKnownAzureModelsUseTheirSpecializedEndpoints(t *testing.T) {
	assert.True(t, IsOpenAIResponseOnlyModel("gpt-5.4-pro"))
	assert.True(t, IsImageGenerationModel("gpt-image-2"))
}
