package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Database DatabaseConfig
	Server   ServerConfig
	GCS      GCSConfig
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

type ServerConfig struct {
	Port         string
	Environment  string
	AllowOrigins []string
}

type GCSConfig struct {
	BucketName      string
	ProjectID       string
	CredentialsPath string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Failed to load .env file: %v, using system environment variables\n", err)
	}

	config := &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "3306"),
			User:     getEnv("DB_USER", "root"),
			Password: getEnv("DB_PASSWORD", ""),
			DBName:   getEnv("DB_NAME", "fastfill_db"),
		},
		Server: ServerConfig{
			Port:        getEnv("PORT", getEnv("SERVER_PORT", "8080")),
			Environment: getEnv("ENVIRONMENT", "development"),
			AllowOrigins: []string{
				getEnv("FRONTEND_URL_1", "http://localhost:3000"),
				getEnv("FRONTEND_URL_2", "http://localhost:3001"),
			},
		},
		GCS: GCSConfig{
			BucketName:      getEnv("GCS_BUCKET_NAME", ""),
			ProjectID:       getEnv("GOOGLE_CLOUD_PROJECT", ""),
			CredentialsPath: getEnv("GCS_CREDENTIALS_PATH", ""),
		},
	}

	return config, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		d.User, d.Password, d.Host, d.Port, d.DBName)
}
