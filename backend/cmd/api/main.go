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
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	logger, err := platform.NewLogger(cfg.AppEnv)
	if err != nil {
		log.Fatalf("logger: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shared, err := runtime.Bootstrap(ctx, cfg, logger)
	if err != nil {
		startup.Fatal(logger, "Không thể khởi động API — kiểm tra môi trường thất bại", err)
	}
	defer shared.Close()

	if err := runtime.RunAPI(ctx, shared); err != nil && err != context.Canceled {
		startup.Fatal(logger, "API thoát bất thường", err)
	}
}
