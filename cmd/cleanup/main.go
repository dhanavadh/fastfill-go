package main

import (
	"flag"
	"log"

	"github.com/dhanavadh/fastfill-backend/internal"
	"github.com/dhanavadh/fastfill-backend/internal/config"
	"github.com/dhanavadh/fastfill-backend/internal/utils"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "Show what would be updated without making changes")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	// Initialize database
	if err := internal.InitDB(cfg); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer internal.CloseDB()

	if *dryRun {
		log.Println("Running in DRY RUN mode - no changes will be made")
		if err := utils.CleanupTemplateURLsDryRun(internal.DB); err != nil {
			log.Fatal("Failed to run dry run:", err)
		}
	} else {
		log.Println("Cleaning up template URLs...")
		if err := utils.CleanupTemplateURLs(internal.DB); err != nil {
			log.Fatal("Failed to cleanup URLs:", err)
		}
		log.Println("Cleanup completed successfully!")
	}
}