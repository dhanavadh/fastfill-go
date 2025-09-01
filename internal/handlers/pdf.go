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
	TemplateID      string                 `json:"templateId" binding:"required"`
	Data            map[string]interface{} `json:"data" binding:"required"`
	FormattingData  map[string]interface{} `json:"formattingData,omitempty"`
	HtmlData        map[string]interface{} `json:"htmlData,omitempty"`
}

func (h *PDFHandler) GeneratePDF(c *gin.Context) {
	var req GeneratePDFRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Failed to bind JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	
	log.Printf("PDF generation request received: templateId=%s, data keys=%v, htmlData keys=%v", 
		req.TemplateID, getKeys(req.Data), getKeys(req.HtmlData))

	template, err := h.templateService.GetByID(req.TemplateID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch template"})
		return
	}

	if template == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	log.Printf("About to generate HTML with data: %+v", req.Data)
	log.Printf("About to generate HTML with htmlData: %+v", req.HtmlData)
	
	htmlContent, err := h.generateHTML(c, *template, req.Data, req.FormattingData, req.HtmlData)
	if err != nil {
		log.Printf("Failed to generate HTML: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate HTML"})
		return
	}
	
	log.Printf("Generated HTML content length: %d", len(htmlContent))
	log.Printf("HTML content preview: %s", htmlContent[:min(1000, len(htmlContent))])

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

	htmlContent, err := h.generateHTML(c, *template, submission.FormData, submission.FormattingData, submission.HtmlData)
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

func (h *PDFHandler) generateHTML(c *gin.Context, tmplData gormmodels.Template, data map[string]interface{}, formattingData map[string]interface{}, htmlData map[string]interface{}) (string, error) {
	log.Printf("Generating HTML for template %s", tmplData.ID)
	log.Printf("Template has %d fields and %d SVG files", len(tmplData.Fields), len(tmplData.SVGFiles))
	log.Printf("Data keys: %v", getKeys(data))
	
	// Check if this is a multi-page template
	if len(tmplData.SVGFiles) > 0 {
		return h.generateMultiPageHTML(tmplData, data, formattingData, htmlData)
	}
	
	// Fallback to legacy single-page generation
	log.Printf("Using legacy single-page generation with SVG background: %s", tmplData.SVGBackground)
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
            width: 794px;
            height: 1123px;
            background-image: url('{{.SVGBackground}}');
            background-size: cover;
            background-repeat: no-repeat;
            background-position: center;
        }
        
        .field {
            position: absolute;
            color: black;
            display: flex;
            align-items: flex-start;
            word-wrap: break-word;
            word-break: break-word;
            white-space: pre-wrap;
            overflow: hidden;
            padding-top: 2px;
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
            font-size: {{if .FontSize}}{{.FontSize}}pt{{else}}12pt{{end}};
            font-weight: {{if .FontWeight}}{{.FontWeight}}{{else}}normal{{end}};
            font-style: {{if .FontStyle}}{{.FontStyle}}{{else}}normal{{end}};
            text-decoration: {{if .TextDecoration}}{{.TextDecoration}}{{else}}none{{end}};
            color: {{if .TextColor}}{{.TextColor}}{{else}}#000000{{end}};
            font-family: {{if .FontFamily}}'{{.FontFamily}}', serif{{else}}'Times New Roman', serif{{end}};
        ">
            <div class="field-text">{{if index $.HtmlData .DataKey}}{{index $.HtmlData .DataKey}}{{else}}{{index $.Data .DataKey}}{{end}}</div>
        </div>
        {{end}}
    </div>
</body>
</html>`

	tmpl, err := template.New("document").Parse(htmlTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Apply formatting overrides to fields
	fieldsWithFormatting := make([]gormmodels.Field, len(tmplData.Fields))
	copy(fieldsWithFormatting, tmplData.Fields)
	
	if formattingData != nil {
		for i, field := range fieldsWithFormatting {
			if fieldFormatting, exists := formattingData[field.DataKey]; exists {
				if formatting, ok := fieldFormatting.(map[string]interface{}); ok {
					if fontWeight, ok := formatting["fontWeight"].(string); ok && fontWeight != "" {
						fieldsWithFormatting[i].FontWeight = fontWeight
					}
					if fontStyle, ok := formatting["fontStyle"].(string); ok && fontStyle != "" {
						fieldsWithFormatting[i].FontStyle = fontStyle
					}
					if textDecoration, ok := formatting["textDecoration"].(string); ok && textDecoration != "" {
						fieldsWithFormatting[i].TextDecoration = textDecoration
					}
					if textColor, ok := formatting["textColor"].(string); ok && textColor != "" {
						fieldsWithFormatting[i].TextColor = textColor
					}
					if fontFamily, ok := formatting["fontFamily"].(string); ok && fontFamily != "" {
						fieldsWithFormatting[i].FontFamily = fontFamily
					}
				}
			}
		}
	}

	// Convert HTML data to template.HTML type to prevent escaping
	processedHtmlData := make(map[string]template.HTML)
	if htmlData != nil {
		for key, value := range htmlData {
			if str, ok := value.(string); ok {
				processedHtmlData[key] = template.HTML(str)
			}
		}
	}

	templateData := struct {
		SVGBackground template.URL
		Fields        []gormmodels.Field
		Data          map[string]interface{}
		HtmlData      map[string]template.HTML
	}{
		SVGBackground: template.URL(svgDataURI),
		Fields:        fieldsWithFormatting,
		Data:          data,
		HtmlData:      processedHtmlData,
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

func (h *PDFHandler) generateMultiPageHTML(tmplData gormmodels.Template, data map[string]interface{}, formattingData map[string]interface{}, htmlData map[string]interface{}) (string, error) {
	log.Printf("Generating multi-page HTML for template %s", tmplData.ID)
	
	// Group fields by page index
	fieldsByPage := make(map[int][]gormmodels.Field)
	for _, field := range tmplData.Fields {
		fieldsByPage[field.PageIndex] = append(fieldsByPage[field.PageIndex], field)
	}
	
	// Group SVG files by page index
	svgFilesByPage := make(map[int]gormmodels.SVGFile)
	for _, svgFile := range tmplData.SVGFiles {
		svgFilesByPage[svgFile.PageIndex] = svgFile
	}
	
	var htmlPages []string
	
	// Generate HTML for each page that has either fields or SVG files
	maxPage := 0
	for pageIndex := range fieldsByPage {
		if pageIndex > maxPage {
			maxPage = pageIndex
		}
	}
	for pageIndex := range svgFilesByPage {
		if pageIndex > maxPage {
			maxPage = pageIndex
		}
	}
	
	for pageIndex := 0; pageIndex <= maxPage; pageIndex++ {
		_, hasSVG := svgFilesByPage[pageIndex]
		fields := fieldsByPage[pageIndex]
		
		// Skip pages with no SVG and no fields
		if !hasSVG && len(fields) == 0 {
			continue
		}
		
		var svgDataURI string
		if hasSVG {
			// Get SVG content using the page-specific identifier
			pageIdentifier := fmt.Sprintf("page_%d", pageIndex)
			content, err := h.uploadHandler.uploadService.GetSVGContent(tmplData.ID, pageIdentifier)
			if err != nil {
				log.Printf("Warning: Failed to get SVG content for page %d: %v", pageIndex, err)
				svgDataURI = ""
			} else {
				// Convert to data URI
				encoded := base64.StdEncoding.EncodeToString(content)
				svgDataURI = fmt.Sprintf("data:image/svg+xml;base64,%s", encoded)
				log.Printf("Generated data URI for page %d, length: %d", pageIndex, len(svgDataURI))
			}
		}
		
		// Apply formatting overrides to fields for this page
		fieldsWithFormatting := make([]gormmodels.Field, len(fields))
		copy(fieldsWithFormatting, fields)
		
		if formattingData != nil {
			for i, field := range fieldsWithFormatting {
				if fieldFormatting, exists := formattingData[field.DataKey]; exists {
					if formatting, ok := fieldFormatting.(map[string]interface{}); ok {
						if fontWeight, ok := formatting["fontWeight"].(string); ok && fontWeight != "" {
							fieldsWithFormatting[i].FontWeight = fontWeight
						}
						if fontStyle, ok := formatting["fontStyle"].(string); ok && fontStyle != "" {
							fieldsWithFormatting[i].FontStyle = fontStyle
						}
						if textDecoration, ok := formatting["textDecoration"].(string); ok && textDecoration != "" {
							fieldsWithFormatting[i].TextDecoration = textDecoration
						}
						if textColor, ok := formatting["textColor"].(string); ok && textColor != "" {
							fieldsWithFormatting[i].TextColor = textColor
						}
						if fontFamily, ok := formatting["fontFamily"].(string); ok && fontFamily != "" {
							fieldsWithFormatting[i].FontFamily = fontFamily
						}
					}
				}
			}
		}
		
		// Merge HTML data into regular data for this page
		mergedData := make(map[string]interface{})
		for k, v := range data {
			mergedData[k] = v
		}
		// Prioritize HTML data over plain text data
		if htmlData != nil {
			for k, v := range htmlData {
				if v != "" {
					mergedData[k] = v
				}
			}
		}
		
		// Generate HTML for this page
		pageHTML := h.generatePageHTML(svgDataURI, fieldsWithFormatting, mergedData)
		htmlPages = append(htmlPages, pageHTML)
	}
	
	if len(htmlPages) == 0 {
		return "", fmt.Errorf("no pages with SVG files or fields found")
	}
	
	// Combine all pages into single HTML document
	fullHTML := fmt.Sprintf(`<!DOCTYPE html>
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
        }
        
        .page {
            position: relative;
            width: 794px;
            height: 1123px;
            background-size: cover;
            background-repeat: no-repeat;
            background-position: center;
            page-break-after: always;
        }
        
        .page:last-child {
            page-break-after: auto;
        }
        
        .field {
            position: absolute;
            color: black;
            display: flex;
            align-items: flex-start;
            word-wrap: break-word;
            word-break: break-word;
            white-space: pre-wrap;
            overflow: hidden;
            padding-top: 2px;
        }
        
        .field-text {
            width: 100%%;
            text-align: left;
        }
    </style>
</head>
<body>
%s
</body>
</html>`, strings.Join(htmlPages, "\n"))
	
	log.Printf("Generated multi-page HTML with %d pages, total length: %d characters", len(htmlPages), len(fullHTML))
	return fullHTML, nil
}

func (h *PDFHandler) generatePageHTML(svgDataURI string, fields []gormmodels.Field, data map[string]interface{}) string {
	var fieldsHTML strings.Builder
	
	for _, field := range fields {
		value, exists := data[field.DataKey]
		if !exists {
			value = ""
		}
		
		fieldsHTML.WriteString(fmt.Sprintf(`
        <div class="field" style="
            top: %dpx;
            left: %dpx;
            width: %dpx;
            height: %dpx;
            font-size: 12pt;
        ">
            <div class="field-text">%v</div>
        </div>`, field.PositionTop, field.PositionLeft, field.PositionWidth, field.PositionHeight, value))
	}
	
	backgroundStyle := ""
	if svgDataURI != "" {
		backgroundStyle = fmt.Sprintf("background-image: url('%s');", svgDataURI)
	}
	
	return fmt.Sprintf(`    <div class="page" style="%s">
%s
    </div>`, backgroundStyle, fieldsHTML.String())
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
		parts := strings.Split(strings.TrimPrefix(url, "/"), "/")
		if len(parts) >= 6 && parts[0] == "api" && parts[1] == "files" && parts[2] == "svg" && parts[4] == "page" {
			// New format: "/api/files/svg/{templateId}/page/{pageIndex}"
			templateID = parts[3]
			pageIndex := parts[5]
			svgID = fmt.Sprintf("page_%s", pageIndex) // Use page index as identifier
		} else if len(parts) >= 4 && parts[0] == "api" && parts[1] == "files" && parts[2] == "svg" {
			// Legacy format: "/api/files/svg/{templateId}"
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
