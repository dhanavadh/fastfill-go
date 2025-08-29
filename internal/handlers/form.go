package handlers

import (
	"net/http"

	gormmodels "github.com/dhanavadh/fastfill-backend/internal/models/gorm"
	"github.com/dhanavadh/fastfill-backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type FormHandler struct {
	formService     *services.FormService
	templateService *services.TemplateService
}

func NewFormHandler(formService *services.FormService, templateService *services.TemplateService) *FormHandler {
	return &FormHandler{
		formService:     formService,
		templateService: templateService,
	}
}

type SubmitFormRequest struct {
	TemplateID string                 `json:"templateId" binding:"required"`
	FormData   map[string]interface{} `json:"formData" binding:"required"`
	Status     string                 `json:"status"`
}

type UpdateFormRequest struct {
	FormData map[string]interface{} `json:"formData"`
	Status   string                 `json:"status"`
}

func (h *FormHandler) Submit(c *gin.Context) {
	var req SubmitFormRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	if req.Status == "" {
		req.Status = "draft"
	}

	submission := &gormmodels.FormSubmission{
		ID:         uuid.New().String(),
		TemplateID: req.TemplateID,
		FormData:   req.FormData,
		Status:     req.Status,
	}

	if err := h.formService.Create(submission); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save form submission"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":      submission.ID,
		"message": "Form submitted successfully",
		"status":  submission.Status,
	})
}

func (h *FormHandler) GetByID(c *gin.Context) {
	submissionID := c.Param("id")

	submission, err := h.formService.GetByID(submissionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch form submission"})
		return
	}

	if submission == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Form submission not found"})
		return
	}

	c.JSON(http.StatusOK, submission)
}

func (h *FormHandler) Update(c *gin.Context) {
	submissionID := c.Param("id")

	var req UpdateFormRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	submission, err := h.formService.GetByID(submissionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch form submission"})
		return
	}

	if submission == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Form submission not found"})
		return
	}

	submission.FormData = req.FormData
	if req.Status != "" {
		submission.Status = req.Status
	}

	if err := h.formService.Update(submission); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update form submission"})
		return
	}

	c.JSON(http.StatusOK, submission)
}

func (h *FormHandler) Delete(c *gin.Context) {
	submissionID := c.Param("id")

	if err := h.formService.Delete(submissionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete form submission"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Form submission deleted successfully"})
}

func (h *FormHandler) GetByTemplateID(c *gin.Context) {
	templateID := c.Param("id")

	submissions, err := h.formService.GetByTemplateID(templateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch form submissions"})
		return
	}

	c.JSON(http.StatusOK, submissions)
}
