// Gateway là entry point duy nhất của backend: kiểm tra môi trường và khởi động
// API + scheduler + worker trong một process (dev và production).
package main

import (
	"context"
	"log"
	"os"
	"os/exec"
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
	logger = logger.Named("gateway")
	defer logger.Sync() //nolint:errcheck
	if cfg.LogLevelEnvInvalid {
		logger.Warn("invalid LOG_LEVEL env, using fallback", zap.String("level", cfg.LogLevel))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	for _, note := range cfg.AutoscaleNotes() {
		logger.Info("autoscale", zap.String("applied", note))
	}

	shared, err := runtime.Bootstrap(ctx, cfg, logger)
	if err != nil {
		startup.Fatal(logger, "Không thể khởi động MediaHub — kiểm tra môi trường thất bại", err)
	}
	defer shared.Close()

	opts := runtime.Options{
		SkipWorker: os.Getenv("SKIP_WORKER") == "1",
	}
	if !opts.SkipWorker {
		if _, err := exec.LookPath(cfg.FFmpegPath); err != nil {
			logger.Warn("ffmpeg không có — bỏ worker",
				zap.String("path", cfg.FFmpegPath),
				zap.String("hint", "cài ffmpeg hoặc SKIP_WORKER=1"),
			)
			opts.SkipWorker = true
		}
	}

	if err := runtime.RunAll(ctx, shared, opts); err != nil && err != context.Canceled {
		startup.Fatal(logger, "MediaHub gateway thoát bất thường", err)
	}
	logger.Info("gateway stopped")
}
