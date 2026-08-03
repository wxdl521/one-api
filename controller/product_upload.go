package controller

import (
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/the-one/common"
	"github.com/gin-gonic/gin"
)

const maxProductImageSize = 5 << 20

func AdminUploadProductImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		common.ApiErrorMsg(c, "product image is required")
		return
	}
	if file.Size <= 0 || file.Size > maxProductImageSize {
		common.ApiErrorMsg(c, "invalid product image size")
		return
	}
	extension := strings.ToLower(filepath.Ext(file.Filename))
	switch extension {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
	default:
		common.ApiErrorMsg(c, "unsupported product image format")
		return
	}
	nameBytes := make([]byte, 16)
	if _, err := rand.Read(nameBytes); err != nil {
		common.ApiError(c, err)
		return
	}
	directory := filepath.Join("data", "product-images")
	if err := c.SaveUploadedFile(file, filepath.Join(directory, hex.EncodeToString(nameBytes)+extension)); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"url": "/product-images/" + hex.EncodeToString(nameBytes) + extension})
}
