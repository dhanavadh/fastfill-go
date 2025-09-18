package internal

import (
	"fmt"
	"log"

	"github.com/dhanavadh/fastfill-backend/internal/config"
	"github.com/dhanavadh/fastfill-backend/internal/models/gorm"

	"gorm.io/driver/mysql"
	gormdb "gorm.io/gorm"
)

var DB *gormdb.DB

func InitDB(cfg *config.Config) error {
	var err error
	dsn := cfg.Database.DSN()
	log.Printf("Connecting to database with DSN: %s", dsn)
	DB, err = gormdb.Open(mysql.Open(dsn), &gormdb.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Printf("Successfully connected to MySQL database: %s", cfg.Database.DBName)

	if err := autoMigrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func autoMigrate() error {
	return DB.AutoMigrate(
		&gorm.Template{},
		&gorm.Field{},
		&gorm.SVGFile{},
		&gorm.FormSubmission{},
	)
}

func CloseDB() {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}
