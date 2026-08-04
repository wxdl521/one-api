package relay

import (
	"github.com/QuantumNous/the-one/service"
	"github.com/gin-gonic/gin"
)

// PromptShot requests contain base64 source images. Even debug logs must not
// serialize those request payloads.
func shouldRedactPromptShotPayload(c *gin.Context) bool {
	return service.IsPromptShotCompatibleRequest(c)
}
