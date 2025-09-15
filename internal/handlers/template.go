package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	gormmodels "github.com/dhanavadh/fastfill-backend/internal/models/gorm"
	"github.com/dhanavadh/fastfill-backend/internal/services"
	"github.com/dhanavadh/fastfill-backend/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TemplateHandler struct {
	templateService *services.TemplateService
	config          *config.Config
}

func NewTemplateHandler(templateService *services.TemplateService, cfg *config.Config) *TemplateHandler {
	return &TemplateHandler{
		templateService: templateService,
		config:          cfg,
	}
}

type TemplateResponse struct {
	ID            string             `json:"id"`
	DisplayName   string             `json:"displayName"`
	Description   string             `json:"description"`
	Category      string             `json:"category"`
	PreviewImage  string             `json:"previewImage"`
	SVGBackground string             `json:"svgBackground"`
	DataInterface string             `json:"dataInterface"`
	Fields        []FieldResponse    `json:"fields"`
	SVGFiles      []SVGFileResponse  `json:"svgFiles,omitempty"`
}

type FieldResponse struct {
	Name               string            `json:"name"`
	Type               string            `json:"type"`
	Required           bool              `json:"required"`
	DataKey            string            `json:"dataKey"`
	IsAddressComponent bool              `json:"isAddressComponent"`
	PageIndex          int               `json:"pageIndex"`
	Options            []string          `json:"options,omitempty"`
	Position           *PositionResponse `json:"position,omitempty"`
}

type SVGFileResponse struct {
	ID           uint   `json:"id"`
	Filename     string `json:"filename"`
	OriginalName string `json:"originalName"`
	PageIndex    int    `json:"pageIndex"`
	FileURL      string `json:"fileUrl"`
}

type PositionResponse struct {
	Top    float64 `json:"top"`
	Left   float64 `json:"left"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type CreateTemplateRequest struct {
	DisplayName   string         `json:"displayName" binding:"required"`
	Description   string         `json:"description"`
	Category      string         `json:"category"`
	PreviewImage  string         `json:"previewImage"`
	SVGBackground string         `json:"svgBackground"`
	DataInterface string         `json:"dataInterface"`
	Fields        []FieldRequest `json:"fields"`
}

type FieldRequest struct {
	Name               string           `json:"name" binding:"required"`
	Type               string           `json:"type" binding:"required"`
	Required           bool             `json:"required"`
	DataKey            string           `json:"dataKey" binding:"required"`
	IsAddressComponent bool             `json:"isAddressComponent"`
	PageIndex          int              `json:"pageIndex"`
	Options            []string         `json:"options,omitempty"`
	Position           *PositionRequest `json:"position"`
}

type PositionRequest struct {
	Top    float64 `json:"top"`
	Left   float64 `json:"left"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func (h *TemplateHandler) GetAll(c *gin.Context) {
	templates, err := h.templateService.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch templates"})
		return
	}

	response := make([]TemplateResponse, len(templates))
	for i, t := range templates {
		response[i] = h.toTemplateResponse(t, c)
	}

	c.JSON(http.StatusOK, response)
}

func (h *TemplateHandler) GetByID(c *gin.Context) {
	templateID := c.Param("id")

	template, err := h.templateService.GetByID(templateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch template"})
		return
	}

	if template == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	c.JSON(http.StatusOK, h.toTemplateResponse(*template, c))
}

func (h *TemplateHandler) Create(c *gin.Context) {
	var req CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON", "details": err.Error()})
		return
	}

	template := &gormmodels.Template{
		ID:            uuid.New().String(),
		DisplayName:   req.DisplayName,
		Description:   req.Description,
		Category:      req.Category,
		PreviewImage:  req.PreviewImage,
		SVGBackground: req.SVGBackground,
		DataInterface: req.DataInterface,
		Fields:        h.toGormFields(req.Fields),
	}

	if template.DataInterface == "" {
		template.DataInterface = template.DisplayName + "FormData"
	}

	if err := h.templateService.Create(template); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create template"})
		return
	}

	c.JSON(http.StatusCreated, h.toTemplateResponse(*template, c))
}

func (h *TemplateHandler) Update(c *gin.Context) {
	templateID := c.Param("id")

	var req CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	template := &gormmodels.Template{
		ID:            templateID,
		DisplayName:   req.DisplayName,
		Description:   req.Description,
		Category:      req.Category,
		PreviewImage:  req.PreviewImage,
		SVGBackground: req.SVGBackground,
		DataInterface: req.DataInterface,
		Fields:        h.toGormFields(req.Fields),
		UpdatedAt:     time.Now(),
	}

	existing, err := h.templateService.GetByID(templateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if existing == nil {
		if err := h.templateService.Create(template); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create template"})
			return
		}
	} else {
		if err := h.templateService.Update(template); err != nil {
			fmt.Printf("Template update error: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update template", "details": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, h.toTemplateResponse(*template, c))
}

func (h *TemplateHandler) Delete(c *gin.Context) {
	templateID := c.Param("id")

	if err := h.templateService.Delete(templateID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Template deleted successfully"})
}

func (h *TemplateHandler) getBaseURL(c *gin.Context) string {
	// Priority: 1. API_BASE_URL config, 2. Request host, 3. localhost fallback
	if h.config.Server.BaseURL != "" {
		return h.config.Server.BaseURL
	}
	
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	
	host := c.Request.Host
	if host == "" {
		host = "localhost:8080" // Final fallback
	}
	
	return fmt.Sprintf("%s://%s", scheme, host)
}

func (h *TemplateHandler) toTemplateResponse(t gormmodels.Template, c *gin.Context) TemplateResponse {
	fields := make([]FieldResponse, len(t.Fields))
	for i, f := range t.Fields {
		var options []string
		if f.Options != "" {
			if err := json.Unmarshal([]byte(f.Options), &options); err != nil {
				// If unmarshal fails, treat as empty options
				options = nil
			}
		}
		
		fields[i] = FieldResponse{
			Name:               f.Name,
			Type:               f.Type,
			Required:           f.Required,
			DataKey:            f.DataKey,
			IsAddressComponent: f.IsAddressComponent,
			PageIndex:          f.PageIndex,
			Options:            options,
			Position: &PositionResponse{
				Top:    float64(f.PositionTop),
				Left:   float64(f.PositionLeft),
				Width:  float64(f.PositionWidth),
				Height: float64(f.PositionHeight),
			},
		}
	}

	svgFiles := make([]SVGFileResponse, len(t.SVGFiles))
	baseURL := h.getBaseURL(c)
	for i, svf := range t.SVGFiles {
		fileURL := fmt.Sprintf("%s/api/files/svg/%s/page/%d", baseURL, t.ID, svf.PageIndex)
		
		svgFiles[i] = SVGFileResponse{
			ID:           svf.ID,
			Filename:     svf.Filename,
			OriginalName: svf.OriginalName,
			PageIndex:    svf.PageIndex,
			FileURL:      fileURL,
		}
	}

	return TemplateResponse{
		ID:            t.ID,
		DisplayName:   t.DisplayName,
		Description:   t.Description,
		Category:      t.Category,
		PreviewImage:  t.PreviewImage,
		SVGBackground: t.SVGBackground,
		DataInterface: t.DataInterface,
		Fields:        fields,
		SVGFiles:      svgFiles,
	}
}

func (h *TemplateHandler) toGormFields(fields []FieldRequest) []gormmodels.Field {
	gormFields := make([]gormmodels.Field, len(fields))
	for i, f := range fields {
		var optionsJSON string
		if len(f.Options) > 0 {
			// Filter out empty options
			validOptions := make([]string, 0)
			for _, opt := range f.Options {
				if strings.TrimSpace(opt) != "" {
					validOptions = append(validOptions, strings.TrimSpace(opt))
				}
			}
			if len(validOptions) > 0 {
				if optionsBytes, err := json.Marshal(validOptions); err == nil {
					optionsJSON = string(optionsBytes)
				}
			}
		}
		
		gormFields[i] = gormmodels.Field{
			Name:               f.Name,
			Type:               f.Type,
			Required:           f.Required,
			DataKey:            f.DataKey,
			IsAddressComponent: f.IsAddressComponent,
			PageIndex:          f.PageIndex,
			Options:            optionsJSON,
		}

		if f.Position != nil {
			gormFields[i].PositionTop = int(f.Position.Top)
			gormFields[i].PositionLeft = int(f.Position.Left)
			gormFields[i].PositionWidth = int(f.Position.Width)
			gormFields[i].PositionHeight = int(f.Position.Height)
		}
	}
	return gormFields
}
