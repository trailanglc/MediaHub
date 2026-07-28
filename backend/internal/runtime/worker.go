package runtime

import (
	"context"
	"time"

	convertprogress "github.com/anhtuanlc/mediahub/internal/platform/convert"
	downloadprog "github.com/anhtuanlc/mediahub/internal/platform/download"
	"github.com/anhtuanlc/mediahub/internal/platform/rediscache"
	"github.com/anhtuanlc/mediahub/internal/platform/resource"
	"github.com/anhtuanlc/mediahub/internal/platform/webhook"
	workerhb "github.com/anhtuanlc/mediahub/internal/platform/worker"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	internalworker "github.com/anhtuanlc/mediahub/internal/worker"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// RunWorker consumes the Asynq convert + download queues until ctx is cancelled.
func RunWorker(ctx context.Context, s *Shared) error {
	cfg := s.Cfg
	logger := s.Logger
	pool := s.Pool
	redisClient := s.Redis
	store := s.Store

	redisCache := rediscache.NewStore(redisClient)
	heartbeat := workerhb.NewHeartbeat(redisClient)
	deletePrefix := store.DeletePrefix

	auditRepo := repository.NewAuditRepository(pool)
	settingsRepo := repository.NewSettingsRepository(pool)
	settingsSvc := service.NewSettingsService(settingsRepo, auditRepo, cfg, redisCache)

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

	convertEnqueue := service.NewConvertEnqueue(cfg.RedisAddr, cfg.ConvertQueueMaxDepth)
	defer convertEnqueue.Close() //nolint:errcheck

	processor := internalworker.NewConvertProcessor(internalworker.ConvertDeps{
		Log:               logger,
		Pool:              pool,
		Store:             store,
		FFmpegPath:        cfg.FFmpegPath,
		FFprobePath:       cfg.FFprobePath,
		FFmpegHwAccel:     cfg.FFmpegHwAccel,
		FFmpegVAAPIDevice: cfg.FFmpegVAAPIDevice,
		JobTimeout:        cfg.ConvertJobTimeout,
		DeletePrefix:      deletePrefix,
		ConvertProgress:   convertprogress.NewProgressStore(redisClient),
		StreamCache:       redisCache,
		GovernorEnabled:   cfg.ResourceGovernorEnabled,
		ResourcePolicy:    resPolicy,
		Gate:              gate,
		Webhooks:          webhook.NewDispatcher(repository.NewWebhookRepository(pool), logger),
		HeartbeatTouch: func(ctx context.Context) error {
			return heartbeat.Touch(ctx)
		},
	})

	initialDownload, _ := settingsSvc.DownloadLimits(ctx)
	if initialDownload.MaxConcurrent < 1 {
		initialDownload.MaxConcurrent = cfg.DownloadMaxConcurrent
		if initialDownload.MaxConcurrent < 1 {
			initialDownload.MaxConcurrent = 2
		}
	}
	downloadGate := resource.NewDynamicGate(initialDownload.MaxConcurrent)

	downloadProcessor := internalworker.NewDownloadProcessor(internalworker.DownloadDeps{
		Log:              logger,
		Pool:             pool,
		Store:            store,
		YTDLPPath:        cfg.YTDLPPath,
		FFmpegPath:       cfg.FFmpegPath,
		ChunkConcurrency: cfg.DownloadChunkConcurrency,
		MaxBytes:         cfg.DownloadMaxBytes,
		JobTimeout:       cfg.DownloadJobTimeout,
		Progress:         downloadprog.NewProgressStore(redisClient),
		StreamCache:      redisCache,
		Gate:             downloadGate,
		ResolveLimits: func(ctx context.Context) internalworker.DownloadRuntimeLimits {
			lim, err := settingsSvc.DownloadLimits(ctx)
			if err != nil {
				return internalworker.DownloadRuntimeLimits{
					ChunkConcurrency: cfg.DownloadChunkConcurrency,
					JobTimeout:       cfg.DownloadJobTimeout,
					MaxConcurrent:    cfg.DownloadMaxConcurrent,
				}
			}
			return internalworker.DownloadRuntimeLimits{
				ChunkConcurrency: lim.ChunkConcurrency,
				JobTimeout:       time.Duration(lim.JobTimeoutSeconds) * time.Second,
				MaxConcurrent:    lim.MaxConcurrent,
			}
		},
		EnqueueConvert: func(ctx context.Context, jobPublicID, videoPublicID uuid.UUID, objectID int64, variants []string, maxAttempts int) error {
			return convertEnqueue.EnqueueConvert(ctx, jobPublicID, videoPublicID, objectID, variants, maxAttempts)
		},
		HeartbeatTouch: func(ctx context.Context) error {
			return heartbeat.Touch(ctx)
		},
	})

	convertConcurrency := cfg.ConvertMaxConcurrent
	if convertConcurrency < 1 {
		convertConcurrency = 1
	}
	// Asynq ceiling; actual parallelism is limited by downloadGate from Settings.
	downloadConcurrency := service.DownloadAsynqConcurrencyCeiling

	retryDelay := func(n int, err error, task *asynq.Task) time.Duration {
		delays := []time.Duration{
			30 * time.Second,
			2 * time.Minute,
			10 * time.Minute,
		}
		if n >= len(delays) {
			return delays[len(delays)-1]
		}
		return delays[n]
	}

	// Separate servers so convert and download concurrency are independent.
	// Download Asynq concurrency is a fixed ceiling; DynamicGate applies Settings max_concurrent.
	redisOpt := asynq.RedisClientOpt{Addr: cfg.RedisAddr}
	convertSrv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency:    convertConcurrency,
		Queues:         map[string]int{"default": 1},
		RetryDelayFunc: retryDelay,
	})
	downloadSrv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency:    downloadConcurrency,
		Queues:         map[string]int{"download": 1},
		RetryDelayFunc: retryDelay,
	})

	convertMux := asynq.NewServeMux()
	convertMux.HandleFunc(internalworker.TypeVideoConvert, processor.ProcessTask)
	downloadMux := asynq.NewServeMux()
	downloadMux.HandleFunc(internalworker.TypeDownloadFetch, downloadProcessor.ProcessTask)

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
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				lim, err := settingsSvc.DownloadLimits(hbCtx)
				if err != nil {
					continue
				}
				if lim.MaxConcurrent > 0 {
					downloadGate.SetLimit(lim.MaxConcurrent)
				}
			}
		}
	}()

	errCh := make(chan error, 2)
	go func() {
		logger.Info("convert worker starting",
			zap.String("redis", cfg.RedisAddr),
			zap.Int("concurrency", convertConcurrency),
		)
		errCh <- convertSrv.Run(convertMux)
	}()
	go func() {
		logger.Info("download worker starting",
			zap.String("redis", cfg.RedisAddr),
			zap.Int("asynq_ceiling", downloadConcurrency),
			zap.Int("max_concurrent", initialDownload.MaxConcurrent),
		)
		errCh <- downloadSrv.Run(downloadMux)
	}()

	select {
	case <-ctx.Done():
		hbCancel()
		govCancel()

		const shutdownTimeout = 60 * time.Second
		done := make(chan struct{})
		go func() {
			convertSrv.Shutdown()
			downloadSrv.Shutdown()
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
		hbCancel()
		govCancel()
		convertSrv.Shutdown()
		downloadSrv.Shutdown()
		return err
	}
}
