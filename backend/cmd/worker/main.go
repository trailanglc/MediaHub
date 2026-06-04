package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/platform"
	"github.com/anhtuanlc/mediahub/internal/platform/postgres"
	platredis "github.com/anhtuanlc/mediahub/internal/platform/redis"
	"github.com/anhtuanlc/mediahub/internal/platform/rediscache"
	"github.com/anhtuanlc/mediahub/internal/platform/resource"
	convertprogress "github.com/anhtuanlc/mediahub/internal/platform/convert"
	workerhb "github.com/anhtuanlc/mediahub/internal/platform/worker"
	internalworker "github.com/anhtuanlc/mediahub/internal/worker"
	"github.com/anhtuanlc/mediahub/internal/storage"
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

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, cfg.DBDSN)
	if err != nil {
		logger.Fatal("postgres", zap.Error(err))
	}
	defer pool.Close()

	redisClient := platredis.NewClientFromConfig(cfg)
	redisCache := rediscache.NewStore(redisClient)
	defer redisClient.Close()

	store, err := storage.NewS3Storage(ctx, cfg.Storage, cfg.HealthMetricsCacheTTL)
	if err != nil {
		logger.Fatal("storage", zap.Error(err))
	}

	heartbeat := workerhb.NewHeartbeat(redisClient)
	deletePrefix := store.DeletePrefix

	resPolicy := resource.PolicyFromConfig(cfg)
	gate := resource.NewDynamicGate(cfg.ConvertMaxConcurrent)
	var gov *resource.Governor
	govCtx, govCancel := context.WithCancel(context.Background())
	defer govCancel()
	if cfg.ResourceGovernorEnabled {
		resource.SeedLatestLimits(resPolicy)
		gov = resource.NewGovernor(logger, redisClient, resPolicy, gate, cfg.ResourceSampleInterval, true)
		go gov.Run(govCtx)
	}

	processor := internalworker.NewConvertProcessor(internalworker.ConvertDeps{
		Log:             logger,
		Pool:            pool,
		Store:           store,
		FFmpegPath:      cfg.FFmpegPath,
		FFprobePath:     cfg.FFprobePath,
		JobTimeout:      cfg.ConvertJobTimeout,
		DeletePrefix:    deletePrefix,
		ConvertProgress: convertprogress.NewProgressStore(redisClient),
		StreamCache:     redisCache,
		GovernorEnabled: cfg.ResourceGovernorEnabled,
		ResourcePolicy:  resPolicy,
		Gate:            gate,
		HeartbeatTouch: func(ctx context.Context) error {
			return heartbeat.Touch(ctx)
		},
	})

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr},
		asynq.Config{Concurrency: cfg.ConvertMaxConcurrent},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(internalworker.TypeVideoConvert, processor.ProcessTask)

	hbCtx, hbCancel := context.WithCancel(context.Background())
	defer hbCancel()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				_ = heartbeat.Touch(hbCtx)
			}
		}
	}()

	go func() {
		logger.Info("worker starting", zap.String("redis", cfg.RedisAddr))
		if err := srv.Run(mux); err != nil {
			logger.Fatal("worker", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	hbCancel()
	govCancel()

	const shutdownTimeout = 60 * time.Second
	done := make(chan struct{})
	go func() {
		srv.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("worker stopped")
	case <-time.After(shutdownTimeout):
		logger.Warn("worker shutdown timed out; exiting",
			zap.Duration("timeout", shutdownTimeout))
	}
}
