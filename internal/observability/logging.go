// Package observability provides logging, metrics, and tracing utilities.
package observability

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/cory-johannsen/mud/internal/config"
)

// NewLogger creates a structured logger from the given logging configuration.
//
// Precondition: cfg.Level must be one of "debug", "info", "warn", "error".
// Precondition: cfg.Format must be "json" or "console".
// Postcondition: Returns a configured zap.Logger or a non-nil error.
func NewLogger(cfg config.LoggingConfig) (*zap.Logger, error) {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("parsing log level %q: %w", cfg.Level, err)
	}

	var zapCfg zap.Config
	switch cfg.Format {
	case "json":
		zapCfg = zap.NewProductionConfig()
	case "console":
		zapCfg = zap.NewDevelopmentConfig()
	default:
		return nil, fmt.Errorf("unknown log format %q", cfg.Format)
	}

	zapCfg.Level = zap.NewAtomicLevelAt(level)
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := zapCfg.Build()
	if err != nil {
		return nil, fmt.Errorf("building logger: %w", err)
	}
	return logger, nil
}
