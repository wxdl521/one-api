package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPromptShotRequestContextMarksErrorsForRedaction(t *testing.T) {
	assert.False(t, IsPromptShotRequestContext(context.Background()))
	assert.True(t, IsPromptShotRequestContext(WithPromptShotRequestContext(context.Background())))
}
