package db

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/oggyb/muzz-exercise/internal/config"
)

// NewDB initializes the database connection using DSN from config.
func NewDB(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DB.DSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // log SQL queries
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	// AutoMigrate ensures schema is in sync with models.
	if err := db.AutoMigrate(&User{}, &Decision{}); err != nil {
		return nil, fmt.Errorf("failed to migrate schema: %w", err)
	}

	return db, nil
}
