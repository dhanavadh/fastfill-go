package services

import (
	"fmt"

	"github.com/dhanavadh/fastfill-backend/internal"
	gormmodels "github.com/dhanavadh/fastfill-backend/internal/models/gorm"

	"gorm.io/gorm"
)

type FormService struct{}

func NewFormService() *FormService {
	return &FormService{}
}

func (s *FormService) Create(submission *gormmodels.FormSubmission) error {
	err := internal.DB.Create(submission).Error
	if err != nil {
		return fmt.Errorf("failed to create form submission: %w", err)
	}
	return nil
}

func (s *FormService) GetByID(id string) (*gormmodels.FormSubmission, error) {
	var submission gormmodels.FormSubmission

	err := internal.DB.Where("id = ?", id).First(&submission).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch form submission: %w", err)
	}

	return &submission, nil
}

func (s *FormService) GetByTemplateID(templateID string) ([]gormmodels.FormSubmission, error) {
	var submissions []gormmodels.FormSubmission

	err := internal.DB.Where("template_id = ?", templateID).Order("created_at DESC").Find(&submissions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch form submissions: %w", err)
	}

	return submissions, nil
}

func (s *FormService) Update(submission *gormmodels.FormSubmission) error {
	err := internal.DB.Model(submission).Updates(submission).Error
	if err != nil {
		return fmt.Errorf("failed to update form submission: %w", err)
	}
	return nil
}

func (s *FormService) Delete(id string) error {
	err := internal.DB.Where("id = ?", id).Delete(&gormmodels.FormSubmission{}).Error
	if err != nil {
		return fmt.Errorf("failed to delete form submission: %w", err)
	}
	return nil
}
