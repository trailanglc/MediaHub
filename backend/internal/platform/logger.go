package platform

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(appEnv string) (*zap.Logger, error) {
	var cfg zap.Config
	if appEnv == "production" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	return cfg.Build()
}
