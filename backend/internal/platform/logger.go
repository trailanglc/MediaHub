package platform

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var validLogLevels = map[string]struct{}{
	"debug": {},
	"info":  {},
	"warn":  {},
	"error": {},
}

// NormalizeLogLevel returns a valid zap level string; invalid values fall back to fallback.
func NormalizeLogLevel(raw, fallback string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return fallback
	}
	if _, ok := validLogLevels[raw]; ok {
		return raw
	}
	return fallback
}

// DefaultLogLevel returns the default log level for an app environment.
func DefaultLogLevel(appEnv string) string {
	if strings.ToLower(strings.TrimSpace(appEnv)) == "development" {
		return "debug"
	}
	return "info"
}

// NewLogger builds a zap logger for appEnv with the given log level.
func NewLogger(appEnv, logLevel string) (*zap.Logger, error) {
	levelStr := NormalizeLogLevel(logLevel, DefaultLogLevel(appEnv))
	level, err := zapcore.ParseLevel(levelStr)
	if err != nil {
		level = zapcore.InfoLevel
	}

	var cfg zap.Config
	if appEnv == "production" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	cfg.Level = zap.NewAtomicLevelAt(level)
	return cfg.Build()
}
