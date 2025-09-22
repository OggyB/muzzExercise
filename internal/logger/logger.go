package logger

import (
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/oggyb/muzz-exercise/internal/config"
)

type Format string

const (
	FormatJSON Format = "json"
	FormatText Format = "text"
)

type Config struct {
	Level      string
	Format     Format
	Component  string
	WithSource bool
}

var (
	mu     sync.RWMutex
	logger *slog.Logger
	cfg    = Config{
		Level:      "info",
		Format:     FormatText,
		Component:  "",
		WithSource: false,
	}
)

// InitFromConfig initializes global logger from app config.
func InitFromConfig(c *config.Config) {
	if c == nil {
		Init(nil)
		return
	}
	Init(&Config{
		Level:      c.Log.Level,
		Format:     Format(c.Log.Format),
		Component:  c.Log.Component,
		WithSource: c.Log.Source,
	})
}

// Init sets up the global logger. Safe to call multiple times.
func Init(c *Config) {
	mu.Lock()
	defer mu.Unlock()

	if c != nil {
		cfg = *c
	}

	lvl := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{
		Level:     lvl,
		AddSource: cfg.WithSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && cfg.Format == FormatText {
				return slog.String(slog.TimeKey, time.Now().Format("2006-01-02 15:04:05"))
			}
			return a
		},
	}

	var handler slog.Handler
	if strings.ToLower(string(cfg.Format)) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	base := slog.New(handler)
	if cfg.Component != "" {
		base = base.With("component", cfg.Component)
	}
	logger = base
}

// L returns the global logger. Always returns a non-nil instance.
func L() *slog.Logger {
	mu.RLock()
	if logger != nil {
		defer mu.RUnlock()
		return logger
	}
	mu.RUnlock()

	// initialize default logger if not set
	Init(nil)

	mu.RLock()
	defer mu.RUnlock()
	return logger
}

// With creates a child logger with additional attributes.
func With(args ...any) *slog.Logger { return L().With(args...) }

func Debug(msg string, args ...any) { L().Debug(msg, args...) }
func Info(msg string, args ...any)  { L().Info(msg, args...) }
func Warn(msg string, args ...any)  { L().Warn(msg, args...) }
func Error(msg string, args ...any) { L().Error(msg, args...) }

// --- helpers ---

func parseLevel(s string) slog.Leveler {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
