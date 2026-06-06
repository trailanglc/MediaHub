package runtime

import (
	"context"
	"time"

	convertprogress "github.com/anhtuanlc/mediahub/internal/platform/convert"
	"github.com/anhtuanlc/mediahub/internal/platform/rediscache"
	"github.com/anhtuanlc/mediahub/internal/platform/resource"
	"github.com/anhtuanlc/mediahub/internal/platform/webhook"
	workerhb "github.com/anhtuanlc/mediahub/internal/platform/worker"
	internalworker "github.com/anhtuanlc/mediahub/internal/worker"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// RunWorker consumes the Asynq convert queue until ctx is cancelled.
func RunWorker(ctx context.Context, s *Shared) error {
	cfg := s.Cfg
	logger := s.Logger
	pool := s.Pool
	redisClient := s.Redis
	store := s.Store

	redisCache := rediscache.NewStore(redisClient)
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
		Webhooks:        webhook.NewDispatcher(repository.NewWebhookRepository(pool), logger),
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

	errCh := make(chan error, 1)
	go func() {
		logger.Info("worker starting", zap.String("redis", cfg.RedisAddr))
		errCh <- srv.Run(mux)
	}()

	select {
	case <-ctx.Done():
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
			logger.Warn("worker shutdown timed out", zap.Duration("timeout", shutdownTimeout))
		}
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
