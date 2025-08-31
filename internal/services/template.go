package services

import (
	"fmt"

	"github.com/dhanavadh/fastfill-backend/internal"
	gormmodels "github.com/dhanavadh/fastfill-backend/internal/models/gorm"

	"gorm.io/gorm"
)

type TemplateService struct{}

func NewTemplateService() *TemplateService {
	return &TemplateService{}
}

func (s *TemplateService) GetAll() ([]gormmodels.Template, error) {
	var templates []gormmodels.Template

	err := internal.DB.Preload("Fields").Preload("SVGFiles").Order("created_at DESC").Find(&templates).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch templates: %w", err)
	}

	return templates, nil
}

func (s *TemplateService) GetByID(id string) (*gormmodels.Template, error) {
	var template gormmodels.Template

	err := internal.DB.Preload("Fields").Preload("SVGFiles").Where("id = ?", id).First(&template).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch template: %w", err)
	}

	return &template, nil
}

func (s *TemplateService) Create(template *gormmodels.Template) error {
	err := internal.DB.Create(template).Error
	if err != nil {
		return fmt.Errorf("failed to create template: %w", err)
	}
	return nil
}

func (s *TemplateService) Update(template *gormmodels.Template) error {
	err := internal.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(template).Updates(template).Error; err != nil {
			return err
		}

		if err := tx.Where("template_id = ?", template.ID).Delete(&gormmodels.Field{}).Error; err != nil {
			return err
		}

		for i := range template.Fields {
			template.Fields[i].TemplateID = template.ID
			if err := tx.Create(&template.Fields[i]).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to update template: %w", err)
	}
	return nil
}

func (s *TemplateService) Delete(id string) error {
	err := internal.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("template_id = ?", id).Delete(&gormmodels.Field{}).Error; err != nil {
			return err
		}

		if err := tx.Where("template_id = ?", id).Delete(&gormmodels.SVGFile{}).Error; err != nil {
			return err
		}

		if err := tx.Where("id = ?", id).Delete(&gormmodels.Template{}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}
	return nil
}
