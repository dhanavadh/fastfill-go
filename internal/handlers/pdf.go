package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	gormmodels "github.com/dhanavadh/fastfill-backend/internal/models/gorm"
	"github.com/dhanavadh/fastfill-backend/internal/services"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
)

type PDFHandler struct {
	templateService *services.TemplateService
	formService     *services.FormService
	uploadHandler   *UploadHandler
}

func NewPDFHandler(templateService *services.TemplateService, formService *services.FormService, uploadHandler *UploadHandler) *PDFHandler {
	return &PDFHandler{
		templateService: templateService,
		formService:     formService,
		uploadHandler:   uploadHandler,
	}
}

type GeneratePDFRequest struct {
	TemplateID string                 `json:"templateId" binding:"required"`
	Data       map[string]interface{} `json:"data" binding:"required"`
}

func (h *PDFHandler) GeneratePDF(c *gin.Context) {
	var req GeneratePDFRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	template, err := h.templateService.GetByID(req.TemplateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch template"})
		return
	}

	if template == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	htmlContent, err := h.generateHTML(*template, req.Data)
	if err != nil {
		log.Printf("Failed to generate HTML: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate HTML"})
		return
	}

	pdfBytes, err := h.htmlToPDF(htmlContent)
	if err != nil {
		log.Printf("Failed to generate PDF: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate PDF"})
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.pdf", req.TemplateID))
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

func (h *PDFHandler) GeneratePDFFromSubmission(c *gin.Context) {
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

	template, err := h.templateService.GetByID(submission.TemplateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch template"})
		return
	}

	if template == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	htmlContent, err := h.generateHTML(*template, submission.FormData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate HTML"})
		return
	}

	pdfBytes, err := h.htmlToPDF(htmlContent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate PDF"})
		return
	}

	filename := fmt.Sprintf("%s_%s.pdf", template.DisplayName, submissionID[:8])
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

func (h *PDFHandler) generateHTML(tmplData gormmodels.Template, data map[string]interface{}) (string, error) {
	// Convert SVG URL to data URI for embedding
	svgDataURI, err := h.convertToDataURI(tmplData.SVGBackground)
	if err != nil {
		return "", fmt.Errorf("failed to convert SVG to data URI: %w", err)
	}
	htmlTemplate := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        @page {
            margin: 0;
            size: A4;
        }
        
        body {
            margin: 0;
            padding: 0;
            font-family: 'Times New Roman', serif;
            position: relative;
        }
        
        .document-container {
            position: relative;
            width: 210mm;
            height: 297mm;
            background-image: url('{{.SVGBackground}}');
            background-size: cover;
            background-repeat: no-repeat;
            background-position: center;
        }
        
        .field {
            position: absolute;
            font-size: 12px;
            color: black;
            display: flex;
            align-items: center;
            word-wrap: break-word;
            overflow: hidden;
        }
        
        .field-text {
            width: 100%;
            text-align: left;
        }
    </style>
</head>
<body>
    <div class="document-container">
        {{range .Fields}}
        <div class="field" style="
            top: {{.PositionTop}}px;
            left: {{.PositionLeft}}px;
            width: {{.PositionWidth}}px;
            height: {{.PositionHeight}}px;
        ">
            <div class="field-text">{{index $.Data .DataKey}}</div>
        </div>
        {{end}}
    </div>
</body>
</html>`

	tmpl, err := template.New("document").Parse(htmlTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	templateData := struct {
		SVGBackground string
		Fields        []gormmodels.Field
		Data          map[string]interface{}
	}{
		SVGBackground: svgDataURI,
		Fields:        tmplData.Fields,
		Data:          data,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, templateData)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func (h *PDFHandler) htmlToPDF(htmlContent string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	var pdfBytes []byte

	err := chromedp.Run(chromeCtx,
		chromedp.Navigate("data:text/html,"+htmlContent),
		chromedp.WaitReady("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBytes, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithPaperWidth(8.27).
				WithPaperHeight(11.69).
				WithMarginTop(0).
				WithMarginBottom(0).
				WithMarginLeft(0).
				WithMarginRight(0).
				Do(ctx)
			return err
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	return pdfBytes, nil
}

func (h *PDFHandler) convertToDataURI(url string) (string, error) {
	if url == "" {
		return "", nil
	}

	// If it's already a data URI, return as is
	if strings.HasPrefix(url, "data:") {
		return url, nil
	}

	// Extract SVG ID from the URL path
	// URL format: "templates/templateId/timestamp.svg"
	parts := strings.Split(strings.TrimPrefix(url, "/"), "/")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid SVG URL format: %s", url)
	}
	
	templateID := parts[1]
	filename := parts[2]
	svgID := strings.TrimSuffix(filename, ".svg")

	// Use the upload handler to get SVG content
	content, err := h.uploadHandler.GetSVGContent(templateID, svgID)
	if err != nil {
		return "", fmt.Errorf("failed to get SVG content: %w", err)
	}

	// Convert to data URI
	encoded := base64.StdEncoding.EncodeToString(content)
	return fmt.Sprintf("data:image/svg+xml;base64,%s", encoded), nil
}
