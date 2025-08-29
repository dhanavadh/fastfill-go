package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
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

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

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

	htmlContent, err := h.generateHTML(c, *template, req.Data)
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

	htmlContent, err := h.generateHTML(c, *template, submission.FormData)
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

func (h *PDFHandler) generateHTML(c *gin.Context, tmplData gormmodels.Template, data map[string]interface{}) (string, error) {
	log.Printf("Generating HTML for template %s with SVG background: %s", tmplData.ID, tmplData.SVGBackground)
	log.Printf("Template has %d fields", len(tmplData.Fields))
	log.Printf("Data keys: %v", getKeys(data))
	
	// Convert SVG URL to data URI for embedding
	svgDataURI, err := h.convertToDataURI(tmplData.SVGBackground)
	if err != nil {
		return "", fmt.Errorf("failed to convert SVG to data URI: %w", err)
	}
	log.Printf("SVG data URI length: %d", len(svgDataURI))
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
		SVGBackground template.URL
		Fields        []gormmodels.Field
		Data          map[string]interface{}
	}{
		SVGBackground: template.URL(svgDataURI),
		Fields:        tmplData.Fields,
		Data:          data,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, templateData)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	htmlContent := buf.String()
	log.Printf("Generated HTML length: %d characters", len(htmlContent))
	log.Printf("HTML preview (first 500 chars): %s", htmlContent[:min(500, len(htmlContent))])
	
	return htmlContent, nil
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
	log.Printf("Converting URL to data URI: %s", url)
	if url == "" {
		log.Printf("Empty URL provided")
		return "", nil
	}

	// If it's already a data URI, return as is
	if strings.HasPrefix(url, "data:") {
		log.Printf("URL is already a data URI")
		return url, nil
	}

	var templateID string
	var svgID string

	// Handle different URL formats
	if strings.Contains(url, "/api/files/svg/") {
		// Current format: "/api/files/svg/{templateId}"
		parts := strings.Split(strings.TrimPrefix(url, "/"), "/")
		if len(parts) >= 4 && parts[0] == "api" && parts[1] == "files" && parts[2] == "svg" {
			templateID = parts[3]
			svgID = "" // Will use most recent SVG for this template
		} else {
			return "", fmt.Errorf("invalid SVG URL format: %s", url)
		}
	} else if strings.Contains(url, "templates/") {
		// Legacy format: "templates/templateId/timestamp.svg" (may or may not have leading slash)
		urlPath := strings.TrimPrefix(url, "/")
		parts := strings.Split(urlPath, "/")
		if len(parts) >= 3 && parts[0] == "templates" {
			templateID = parts[1]
			filename := parts[2]
			svgID = strings.TrimSuffix(filename, ".svg")
		} else {
			return "", fmt.Errorf("invalid SVG URL format: %s", url)
		}
	} else {
		return "", fmt.Errorf("unsupported SVG URL format: %s", url)
	}

	log.Printf("Parsed templateID: %s, svgID: %s", templateID, svgID)
	
	// Use the upload handler to get SVG content
	content, err := h.uploadHandler.GetSVGContent(templateID, svgID)
	if err != nil {
		return "", fmt.Errorf("failed to get SVG content: %w", err)
	}

	log.Printf("Retrieved SVG content length: %d bytes", len(content))
	
	// Convert to data URI
	encoded := base64.StdEncoding.EncodeToString(content)
	dataURI := fmt.Sprintf("data:image/svg+xml;base64,%s", encoded)
	log.Printf("Generated data URI (first 100 chars): %s...", dataURI[:min(100, len(dataURI))])
	return dataURI, nil
}

func (h *PDFHandler) convertToDirectURL(c *gin.Context, url string) string {
	// Build absolute URL with scheme and host
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	
	// If it's already a proper API URL, make it absolute
	if strings.Contains(url, "/api/files/svg/") {
		return baseURL + url
	}
	
	// Convert legacy format to new SVG serving route with absolute URL
	if strings.Contains(url, "templates/") {
		urlPath := strings.TrimPrefix(url, "/")
		parts := strings.Split(urlPath, "/")
		if len(parts) >= 3 && parts[0] == "templates" {
			templateID := parts[1]
			filename := parts[2]
			// Use the new SVG route that serves SVGs directly
			return fmt.Sprintf("%s/api/svg/%s/%s", baseURL, templateID, filename)
		}
	}
	
	// Fallback to original URL
	return url
}

func (h *PDFHandler) getSignedSVGURL(url string) (string, error) {
	// Parse the template ID from the URL
	var templateID string
	
	if strings.Contains(url, "/api/files/svg/") {
		parts := strings.Split(strings.TrimPrefix(url, "/"), "/")
		if len(parts) >= 4 && parts[0] == "api" && parts[1] == "files" && parts[2] == "svg" {
			templateID = parts[3]
		}
	} else if strings.Contains(url, "templates/") {
		urlPath := strings.TrimPrefix(url, "/")
		parts := strings.Split(urlPath, "/")
		if len(parts) >= 3 && parts[0] == "templates" {
			templateID = parts[1]
		}
	} else {
		return url, nil // Return original if we can't parse it
	}
	
	if templateID == "" {
		return url, nil
	}
	
	// Get the signed URL directly from upload service
	signedURL, err := h.uploadHandler.uploadService.GetSVGFileURL(templateID)
	if err != nil {
		return "", fmt.Errorf("failed to get signed URL: %w", err)
	}
	
	return signedURL, nil
}
