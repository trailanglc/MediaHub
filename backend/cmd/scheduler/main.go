package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/platform"
	"github.com/anhtuanlc/mediahub/internal/platform/startup"
	"github.com/anhtuanlc/mediahub/internal/runtime"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	logger, err := platform.NewLogger(cfg.AppEnv, cfg.LogLevel)
	if err != nil {
		log.Fatalf("logger: %v", err)
	}
	logger = logger.Named("scheduler")
	defer logger.Sync() //nolint:errcheck
	if cfg.LogLevelEnvInvalid {
		logger.Warn("invalid LOG_LEVEL env, using fallback", zap.String("level", cfg.LogLevel))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shared, err := runtime.Bootstrap(ctx, cfg, logger)
	if err != nil {
		startup.Fatal(logger, "Không thể khởi động scheduler — kiểm tra môi trường thất bại", err)
	}
	defer shared.Close()

	if err := runtime.RunScheduler(ctx, shared); err != nil && err != context.Canceled {
		startup.Fatal(logger, "Scheduler thoát bất thường", err)
	}
}
