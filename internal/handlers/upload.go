package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dhanavadh/fastfill-backend/internal/services"

	"github.com/gin-gonic/gin"
)

type UploadHandler struct {
	uploadService   *services.UploadService
	templateService *services.TemplateService
}

func NewUploadHandler(uploadService *services.UploadService, templateService *services.TemplateService) *UploadHandler {
	return &UploadHandler{
		uploadService:   uploadService,
		templateService: templateService,
	}
}

func (h *UploadHandler) UploadSVG(c *gin.Context) {
	templateID := c.Param("templateId")

	file, header, err := c.Request.FormFile("svg")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	if header.Header.Get("Content-Type") != "image/svg+xml" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File must be an SVG"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	svgFile, err := h.uploadService.UploadSVG(ctx, templateID, file, header)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload file"})
		return
	}

	// Generate URL for frontend to use  
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	fileURL := fmt.Sprintf("%s://%s/api/files/svg/%s", scheme, c.Request.Host, templateID)

	template, err := h.templateService.GetByID(templateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update template"})
		return
	}

	if template != nil {
		template.SVGBackground = fileURL
		if err := h.templateService.Update(template); err != nil {
			fmt.Printf("Warning: Failed to update template SVG background: %v\n", err)
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message":      "File uploaded successfully",
		"filename":     svgFile.Filename,
		"originalName": svgFile.OriginalName,
		"size":         svgFile.FileSize,
		"url":          fileURL,
		"gcsPath":      svgFile.GCSPath,
	})
}

func (h *UploadHandler) GetSVG(c *gin.Context) {
	templateID := c.Param("id")

	signedURL, err := h.uploadService.GetSVGFileURL(templateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch SVG file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": signedURL})
}

func (h *UploadHandler) ServeSVG(c *gin.Context) {
	templateID := c.Param("id")

	svgFile, err := h.uploadService.GetSVGFile(templateID)
	if err != nil || svgFile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SVG file not found"})
		return
	}

	signedURL, err := h.uploadService.GetSVGFileURL(templateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get file"})
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, signedURL)
}

func (h *UploadHandler) GetSVGContent(templateID, svgID string) ([]byte, error) {
	return h.uploadService.GetSVGContent(templateID, svgID)
}

func (h *UploadHandler) ServeLegacySVG(c *gin.Context) {
	templateID := c.Param("templateId")
	filename := c.Param("filename")
	
	// Extract SVG ID from filename (remove .svg extension)
	svgID := strings.TrimSuffix(filename, ".svg")
	
	// Get SVG content
	content, err := h.uploadService.GetSVGContent(templateID, svgID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SVG file not found"})
		return
	}
	
	// Serve the SVG content directly
	c.Header("Content-Type", "image/svg+xml")
	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, "image/svg+xml", content)
}
