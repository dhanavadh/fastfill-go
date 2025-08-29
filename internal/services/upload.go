package services

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/dhanavadh/fastfill-backend/internal"
	gormmodels "github.com/dhanavadh/fastfill-backend/internal/models/gorm"
	"github.com/dhanavadh/fastfill-backend/internal/storage"

	"gorm.io/gorm"
)

type UploadService struct {
	gcsClient *storage.GCSClient
}

func NewUploadService(gcsClient *storage.GCSClient) *UploadService {
	return &UploadService{
		gcsClient: gcsClient,
	}
}

func (s *UploadService) UploadSVG(ctx context.Context, templateID string, file multipart.File, header *multipart.FileHeader) (*gormmodels.SVGFile, error) {
	objectName := storage.GenerateObjectName(templateID, header.Filename)

	result, err := s.gcsClient.UploadFile(ctx, file, objectName, header.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("failed to upload to GCS: %w", err)
	}

	svgFile := &gormmodels.SVGFile{
		TemplateID:   templateID,
		Filename:     header.Filename,
		OriginalName: header.Filename,
		FilePath:     objectName, // Store GCS path instead of public URL
		GCSPath:      objectName,
		FileSize:     result.Size,
		MimeType:     header.Header.Get("Content-Type"),
	}

	if err := internal.DB.Create(svgFile).Error; err != nil {
		s.gcsClient.DeleteFile(ctx, objectName)
		return nil, fmt.Errorf("failed to save file metadata: %w", err)
	}

	return svgFile, nil
}

func (s *UploadService) GetSVGFile(templateID string) (*gormmodels.SVGFile, error) {
	var svgFile gormmodels.SVGFile

	err := internal.DB.Where("template_id = ?", templateID).Order("created_at DESC").First(&svgFile).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch SVG file: %w", err)
	}

	return &svgFile, nil
}

func (s *UploadService) GetSVGFileURL(templateID string) (string, error) {
	svgFile, err := s.GetSVGFile(templateID)
	if err != nil {
		return "", err
	}
	if svgFile == nil {
		return "", fmt.Errorf("SVG file not found")
	}

	// Generate signed URL valid for 1 hour
	signedURL, err := s.gcsClient.GetSignedURL(svgFile.GCSPath, time.Hour)
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	return signedURL, nil
}

func (s *UploadService) DeleteSVGFile(ctx context.Context, templateID string) error {
	var svgFile gormmodels.SVGFile

	err := internal.DB.Where("template_id = ?", templateID).First(&svgFile).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return fmt.Errorf("failed to fetch SVG file: %w", err)
	}

	if svgFile.GCSPath != "" {
		if err := s.gcsClient.DeleteFile(ctx, svgFile.GCSPath); err != nil {
			return fmt.Errorf("failed to delete from GCS: %w", err)
		}
	}

	if err := internal.DB.Delete(&svgFile).Error; err != nil {
		return fmt.Errorf("failed to delete file metadata: %w", err)
	}

	return nil
}

func (s *UploadService) GetSVGContent(templateID, svgID string) ([]byte, error) {
	// Get signed URL and fetch content via HTTP
	signedURL, err := s.GetSVGFileURL(templateID)
	if err != nil {
		return nil, err
	}

	// Fetch content using the signed URL
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", signedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch SVG: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch SVG: status %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read SVG content: %w", err)
	}

	return content, nil
}
