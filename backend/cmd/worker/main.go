package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/platform"
	internalworker "github.com/anhtuanlc/mediahub/internal/worker"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
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

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr},
		asynq.Config{Concurrency: 2},
	)

	mux := asynq.NewServeMux()
	convertHandler := &internalworker.ConvertHandler{Log: logger}
	mux.HandleFunc(internalworker.TypeVideoConvert, convertHandler.ProcessTask)

	go func() {
		logger.Info("worker starting", zap.String("redis", cfg.RedisAddr))
		if err := srv.Run(mux); err != nil {
			logger.Fatal("worker", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	srv.Shutdown()
	logger.Info("worker stopped")
}
