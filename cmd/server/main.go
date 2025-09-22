package main

import (
	"context"
	"github.com/oggyb/muzz-exercise/internal/app"
	"github.com/oggyb/muzz-exercise/internal/cache"
	"github.com/oggyb/muzz-exercise/internal/config"
	"github.com/oggyb/muzz-exercise/internal/db"
	"github.com/oggyb/muzz-exercise/internal/logger"
	"github.com/oggyb/muzz-exercise/internal/server"
	"github.com/oggyb/muzz-exercise/internal/service/explore"
)

func main() {
	cfg := config.New()

	// Init logger (global singleton)
	logger.InitFromConfig(cfg)
	log := logger.L() // slog.Logger pointer

	// Init DB
	database, err := db.NewDB(cfg)
	if err != nil {
		log.Error("failed to init db", "err", err)
		return
	}

	// Init Redis
	redisCache := cache.NewRedisCache(cfg)
	if err := redisCache.Ping(context.Background()); err != nil {
		log.Error("failed to connect to redis", "err", err)
		return
	}

	// Inject logger into app context
	appCtx := app.New(database, redisCache, log)

	registrars := []server.Registrar{
		explore.NewRegistrar(appCtx),
	}

	if cfg.App.ENV == "development" {
		if err := db.SeedTestData(database); err != nil {
			log.Error("failed to seed: %v", err)
		}
	}

	addr := cfg.GRPC.Host + ":" + cfg.GRPC.Port
	log.Info("starting gRPC server", "addr", addr)

	if err := server.StartGRPCServer(cfg, registrars...); err != nil {
		log.Error("failed to start gRPC server", "err", err)
	}
}
