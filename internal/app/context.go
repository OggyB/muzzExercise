package app

import (
	"github.com/oggyb/muzz-exercise/internal/cache"
	"gorm.io/gorm"
	"log/slog"
)

// AppContext holds shared dependencies (DB, Redis, Logger, etc.)
type AppContext struct {
	DB         *gorm.DB
	RedisCache *cache.RedisCache
	Logger     *slog.Logger
}

// New creates a new AppContext
func New(db *gorm.DB, rdb *cache.RedisCache, logger *slog.Logger) *AppContext {
	return &AppContext{
		DB:         db,
		RedisCache: rdb,
		Logger:     logger,
	}
}
