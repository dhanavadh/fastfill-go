package handlers

import (
	"net/http"

	"github.com/dhanavadh/fastfill-backend/internal/services"
	"github.com/gin-gonic/gin"
)

type OCRHandler struct {
	ocrService *services.OCRService
}

func NewOCRHandler(ocrService *services.OCRService) *OCRHandler {
	return &OCRHandler{
		ocrService: ocrService,
	}
}

func (h *OCRHandler) ProcessImage(c *gin.Context) {
	// Get uploaded file
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No image file provided"})
		return
	}
	defer file.Close()

	// Validate file type
	if !isImageFile(header.Header.Get("Content-Type")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File must be an image"})
		return
	}

	// Process the image
	result, err := h.ocrService.ProcessThaiID(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OCR processing failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func isImageFile(contentType string) bool {
	allowedTypes := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/gif",
		"image/bmp",
		"image/webp",
	}

	for _, allowedType := range allowedTypes {
		if contentType == allowedType {
			return true
		}
	}
	return false
}