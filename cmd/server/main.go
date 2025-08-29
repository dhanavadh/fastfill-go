package main

import (
	"log"
	"strings"

	"github.com/dhanavadh/fastfill-backend/internal"
	"github.com/dhanavadh/fastfill-backend/internal/config"
	"github.com/dhanavadh/fastfill-backend/internal/handlers"
	"github.com/dhanavadh/fastfill-backend/internal/services"
	"github.com/dhanavadh/fastfill-backend/internal/storage"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	if err := internal.InitDB(cfg); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer internal.CloseDB()

	var gcsClient *storage.GCSClient
	if cfg.GCS.BucketName != "" {
		gcsClient, err = storage.NewGCSClient(cfg.GCS.BucketName, cfg.GCS.CredentialsPath)
		if err != nil {
			log.Fatal("Failed to initialize GCS client:", err)
		}
		log.Println("GCS client initialized successfully")
	} else {
		log.Fatal("GCS bucket name is required")
	}

	templateService := services.NewTemplateService()
	formService := services.NewFormService()
	uploadService := services.NewUploadService(gcsClient)

	templateHandler := handlers.NewTemplateHandler(templateService)
	formHandler := handlers.NewFormHandler(formService, templateService)
	uploadHandler := handlers.NewUploadHandler(uploadService, templateService)
	pdfHandler := handlers.NewPDFHandler(templateService, formService, uploadHandler)
	legacyHandler := handlers.NewLegacyHandler(templateService)

	r := gin.Default()

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = cfg.Server.AllowOrigins
	corsConfig.AllowCredentials = true
	r.Use(cors.New(corsConfig))

	api := r.Group("/api")
	{
		api.GET("/templates", templateHandler.GetAll)
		api.GET("/templates/:id", templateHandler.GetByID)
		api.PUT("/templates/:id", templateHandler.Update)
		api.DELETE("/templates/:id", templateHandler.Delete)
		api.POST("/templates", templateHandler.Create)

		api.POST("/upload/svg/:templateId", uploadHandler.UploadSVG)
		api.GET("/templates/:id/svg", uploadHandler.GetSVG)
		api.GET("/files/svg/:id", uploadHandler.ServeSVG)

		api.POST("/forms/submit", formHandler.Submit)
		api.GET("/forms/:id", formHandler.GetByID)
		api.PUT("/forms/:id", formHandler.Update)
		api.DELETE("/forms/:id", formHandler.Delete)
		api.GET("/templates/:id/forms", formHandler.GetByTemplateID)

		api.POST("/generate-pdf", pdfHandler.GeneratePDF)
		api.POST("/forms/:id/generate-pdf", pdfHandler.GeneratePDFFromSubmission)

		api.GET("/form-templates", legacyHandler.GetFormTemplates)
		api.POST("/templates/from-form-svg", legacyHandler.CreateTemplateFromFormSVG)

		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
	}

	r.Static("/static", "./static")

	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/static/") {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "*")
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(204)
				return
			}
		}
		c.Next()
	})

	log.Printf("Server starting on :%s", cfg.Server.Port)
	r.Run(":" + cfg.Server.Port)
}
