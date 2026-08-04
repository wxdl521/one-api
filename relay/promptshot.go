package relay

import "github.com/gin-gonic/gin"

// PromptShot requests contain base64 source images. Even debug logs must not
// serialize those request payloads.
func shouldRedactPromptShotPayload(c *gin.Context) bool {
	return c != nil && c.GetBool("promptshot_compat")
}
