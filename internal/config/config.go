package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Log struct {
		Level     string
		Format    string
		Component string
		Source    bool
	}

	DB struct {
		DSN      string
		Host     string
		Port     string
		User     string
		Password string
		Name     string
	}

	Redis struct {
		Addr     string
		Password string
		DB       int
	}

	GRPC struct {
		Host string
		Port string
	}
}

func New() *Config {
	cfg := &Config{}

	// Logger
	cfg.Log.Level = getEnvDefault("LOG_LEVEL", "info")
	cfg.Log.Format = getEnvDefault("LOG_FORMAT", "text")
	cfg.Log.Component = getEnvDefault("LOG_COMPONENT", "grpc_server")
	cfg.Log.Source = isTruthy(os.Getenv("LOG_SOURCE"))

	// Database
	cfg.DB.DSN = os.Getenv("MYSQL_DSN")
	if cfg.DB.DSN == "" {
		cfg.DB.Host = getEnvDefault("DB_HOST", "localhost")
		cfg.DB.Port = getEnvDefault("DB_PORT", "3306")
		cfg.DB.User = getEnvDefault("DB_USER", "root")
		cfg.DB.Password = getEnvDefault("DB_PASSWORD", "root")
		cfg.DB.Name = getEnvDefault("DB_NAME", "muzz")

		cfg.DB.DSN = fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=UTC",
			cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.Name,
		)
	}

	// Redis
	cfg.Redis.Addr = getEnvDefault("REDIS_ADDR", "localhost:6379")
	cfg.Redis.Password = getEnvDefault("REDIS_PASSWORD", "")
	if dbStr := getEnvDefault("REDIS_DB", "0"); dbStr != "" {
		if dbInt, err := strconv.Atoi(dbStr); err == nil {
			cfg.Redis.DB = dbInt
		}
	}

	// gRPC
	cfg.GRPC.Host = getEnvDefault("GRPC_HOST", "127.0.0.1")
	cfg.GRPC.Port = getEnvDefault("GRPC_PORT", "50051")

	return cfg
}

func getEnvDefault(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	}
	return false
}
