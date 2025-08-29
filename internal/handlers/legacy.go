package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	gormmodels "github.com/dhanavadh/fastfill-backend/internal/models/gorm"
	"github.com/dhanavadh/fastfill-backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type LegacyHandler struct {
	templateService *services.TemplateService
}

func NewLegacyHandler(templateService *services.TemplateService) *LegacyHandler {
	return &LegacyHandler{
		templateService: templateService,
	}
}

type FormTemplate struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	SvgFiles    []string `json:"svgFiles"`
	PreviewUrl  string   `json:"previewUrl,omitempty"`
}

type CreateFromFormSVGRequest struct {
	DisplayName  string `json:"displayName" binding:"required"`
	Description  string `json:"description"`
	Category     string `json:"category"`
	FormCategory string `json:"formCategory" binding:"required"`
	SvgFileName  string `json:"svgFileName" binding:"required"`
}

func (h *LegacyHandler) GetFormTemplates(c *gin.Context) {
	formSVGDir := "./static/templates/form_svg"

	if _, err := os.Stat(formSVGDir); os.IsNotExist(err) {
		c.JSON(http.StatusOK, gin.H{"templates": []FormTemplate{}})
		return
	}

	entries, err := os.ReadDir(formSVGDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read form templates"})
		return
	}

	var templates []FormTemplate
	apiBaseURL := os.Getenv("API_BASE_URL")

	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != ".DS_Store" {
			categoryPath := filepath.Join(formSVGDir, entry.Name())

			svgFiles, err := os.ReadDir(categoryPath)
			if err != nil {
				continue
			}

			var svgFileNames []string
			var previewUrl string

			for _, svgFile := range svgFiles {
				if strings.HasSuffix(strings.ToLower(svgFile.Name()), ".svg") {
					svgFileNames = append(svgFileNames, svgFile.Name())
					if previewUrl == "" {
						previewUrl = fmt.Sprintf("%s/static/templates/form_svg/%s/%s",
							apiBaseURL, entry.Name(), svgFile.Name())
					}
				}
			}

			if len(svgFileNames) > 0 {
				templates = append(templates, FormTemplate{
					Name:        entry.Name(),
					DisplayName: entry.Name(),
					SvgFiles:    svgFileNames,
					PreviewUrl:  previewUrl,
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

func (h *LegacyHandler) CreateTemplateFromFormSVG(c *gin.Context) {
	var req CreateFromFormSVGRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data", "details": err.Error()})
		return
	}

	svgPath := filepath.Join("./static/templates/form_svg", req.FormCategory, req.SvgFileName)
	if _, err := os.Stat(svgPath); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Selected SVG file does not exist"})
		return
	}

	template := &gormmodels.Template{
		ID:          uuid.New().String(),
		DisplayName: req.DisplayName,
		Description: req.Description,
		Category:    req.Category,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.templateService.Create(template); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create template"})
		return
	}

	apiBaseURL := os.Getenv("API_BASE_URL")
	template.SVGBackground = fmt.Sprintf("%s/static/templates/form_svg/%s/%s",
		apiBaseURL, req.FormCategory, req.SvgFileName)

	if err := h.templateService.Update(template); err != nil {
		fmt.Printf("Warning: Failed to update template SVG background: %v\n", err)
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":      template.ID,
		"message": "Template created successfully",
		"template": gin.H{
			"id":            template.ID,
			"displayName":   template.DisplayName,
			"description":   template.Description,
			"category":      template.Category,
			"svgBackground": template.SVGBackground,
			"createdAt":     template.CreatedAt,
		},
	})
}
