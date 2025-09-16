package utils

import (
	"fmt"
	"regexp"
	"strings"

	gormmodels "github.com/dhanavadh/fastfill-backend/internal/models/gorm"
	"gorm.io/gorm"
)

// CleanupTemplateURLs updates existing templates to use template IDs instead of full URLs
func CleanupTemplateURLs(db *gorm.DB) error {
	var templates []gormmodels.Template
	
	// Find all templates with URLs in SVGBackground field
	if err := db.Where("svg_background LIKE ? OR svg_background LIKE ?", "http://localhost%", "https://asia-southeast-apis.dooform.com%").Find(&templates).Error; err != nil {
		return fmt.Errorf("failed to fetch templates: %w", err)
	}

	// Regular expression to extract template ID from URL
	// Matches: http://localhost:8080/api/files/svg/{template-id} or https://domain/api/files/svg/{template-id}
	urlPattern := regexp.MustCompile(`https?://[^/]+/api/files/svg/([a-fA-F0-9-]+)`)

	updatedCount := 0
	for _, template := range templates {
		if template.SVGBackground != "" {
			// Extract template ID from URL
			matches := urlPattern.FindStringSubmatch(template.SVGBackground)
			if len(matches) > 1 {
				extractedID := matches[1]
				
				// Verify the extracted ID matches the template ID
				if extractedID == template.ID {
					// Update to use just the template ID
					template.SVGBackground = template.ID
					if err := db.Save(&template).Error; err != nil {
						fmt.Printf("Warning: Failed to update template %s: %v\n", template.ID, err)
						continue
					}
					updatedCount++
					fmt.Printf("Updated template: %s - %s\n", template.ID, template.DisplayName)
				} else {
					fmt.Printf("Warning: Extracted ID %s doesn't match template ID %s for template %s\n", 
						extractedID, template.ID, template.DisplayName)
				}
			} else {
				// Handle special cases like the "templates/..." format
				if strings.Contains(template.SVGBackground, "templates/") {
					// Keep as is for now, or handle specially if needed
					fmt.Printf("Skipping special format URL for template %s: %s\n", template.ID, template.SVGBackground)
				} else {
					fmt.Printf("Warning: Could not parse URL for template %s: %s\n", template.ID, template.SVGBackground)
				}
			}
		}
	}

	fmt.Printf("Successfully updated %d templates\n", updatedCount)
	return nil
}

// CleanupTemplateURLsDryRun shows what would be updated without making changes
func CleanupTemplateURLsDryRun(db *gorm.DB) error {
	var templates []gormmodels.Template
	
	if err := db.Where("svg_background LIKE ? OR svg_background LIKE ?", "http://localhost%", "https://asia-southeast-apis.dooform.com%").Find(&templates).Error; err != nil {
		return fmt.Errorf("failed to fetch templates: %w", err)
	}

	urlPattern := regexp.MustCompile(`https?://[^/]+/api/files/svg/([a-fA-F0-9-]+)`)

	fmt.Printf("DRY RUN: Would update %d templates:\n", len(templates))
	for _, template := range templates {
		if template.SVGBackground != "" {
			matches := urlPattern.FindStringSubmatch(template.SVGBackground)
			if len(matches) > 1 {
				extractedID := matches[1]
				if extractedID == template.ID {
					fmt.Printf("  %s - %s: %s -> %s\n", 
						template.ID, template.DisplayName, template.SVGBackground, template.ID)
				}
			}
		}
	}

	return nil
}