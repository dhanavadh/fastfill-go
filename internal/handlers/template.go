package handlers

import (
	"fmt"
	"net/http"
	"time"

	gormmodels "github.com/dhanavadh/fastfill-backend/internal/models/gorm"
	"github.com/dhanavadh/fastfill-backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TemplateHandler struct {
	templateService *services.TemplateService
}

func NewTemplateHandler(templateService *services.TemplateService) *TemplateHandler {
	return &TemplateHandler{
		templateService: templateService,
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
		response[i] = h.toTemplateResponse(t)
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

	c.JSON(http.StatusOK, h.toTemplateResponse(*template))
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

	c.JSON(http.StatusCreated, h.toTemplateResponse(*template))
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update template"})
			return
		}
	}

	c.JSON(http.StatusOK, h.toTemplateResponse(*template))
}

func (h *TemplateHandler) Delete(c *gin.Context) {
	templateID := c.Param("id")

	if err := h.templateService.Delete(templateID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Template deleted successfully"})
}

func (h *TemplateHandler) toTemplateResponse(t gormmodels.Template) TemplateResponse {
	fields := make([]FieldResponse, len(t.Fields))
	for i, f := range t.Fields {
		fields[i] = FieldResponse{
			Name:               f.Name,
			Type:               f.Type,
			Required:           f.Required,
			DataKey:            f.DataKey,
			IsAddressComponent: f.IsAddressComponent,
			PageIndex:          f.PageIndex,
			Position: &PositionResponse{
				Top:    float64(f.PositionTop),
				Left:   float64(f.PositionLeft),
				Width:  float64(f.PositionWidth),
				Height: float64(f.PositionHeight),
			},
		}
	}

	svgFiles := make([]SVGFileResponse, len(t.SVGFiles))
	for i, svf := range t.SVGFiles {
		scheme := "http"
		host := "localhost:8080" // Default fallback
		fileURL := fmt.Sprintf("%s://%s/api/files/svg/%s/page/%d", scheme, host, t.ID, svf.PageIndex)
		
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
		gormFields[i] = gormmodels.Field{
			Name:               f.Name,
			Type:               f.Type,
			Required:           f.Required,
			DataKey:            f.DataKey,
			IsAddressComponent: f.IsAddressComponent,
			PageIndex:          f.PageIndex,
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
