package logger

import (
	"fmt"
	"log/slog"
	"os"

	"orders-go/internal/config"
)

func New(cfg config.Config) (*slog.Logger, error) {
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, fmt.Errorf("nivel de registro no valido: %s", cfg.LogLevel)
	}

	options := &slog.HandlerOptions{Level: level}
	if cfg.LogFormat == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, options)), nil
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, options)), nil
}
