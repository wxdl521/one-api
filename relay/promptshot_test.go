package relay

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestShouldRedactPromptShotPayload(t *testing.T) {
	context, _ := gin.CreateTestContext(nil)
	assert.False(t, shouldRedactPromptShotPayload(context))

	context.Set("promptshot_compat", true)
	assert.True(t, shouldRedactPromptShotPayload(context))
}
