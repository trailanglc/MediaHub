package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/anhtuanlc/mediahub/internal/authz"
	"github.com/anhtuanlc/mediahub/internal/background"
	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/platform"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/anhtuanlc/mediahub/internal/storage"
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := platform.NewPostgresPool(ctx, cfg.DBDSN)
	if err != nil {
		logger.Fatal("postgres", zap.Error(err))
	}
	defer pool.Close()

	redisClient := platform.NewRedisClient(cfg.RedisAddr)
	defer redisClient.Close()

	store, err := storage.NewS3Storage(ctx, cfg.Storage, cfg.HealthMetricsCacheTTL)
	if err != nil {
		logger.Fatal("storage", zap.Error(err))
	}

	mediaRepo := repository.NewMediaObjectRepository(pool)
	settingsRepo := repository.NewSettingsRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	permRepo := repository.NewPermissionRepository(pool)
	uploadRepo := repository.NewUploadSessionRepository(pool)
	deletionRepo := repository.NewStorageDeletionRepository(pool)

	settingsSvc := service.NewSettingsService(settingsRepo, auditRepo, cfg)
	authzSvc := authz.NewService(permRepo)
	storageCleanup := service.NewStorageCleanupService(deletionRepo, store)
	thumbnailSvc := service.NewThumbnailService(mediaRepo, store)
	mediaSvc := service.NewMediaObjectService(mediaRepo, settingsSvc, authzSvc, auditRepo, store, storageCleanup, thumbnailSvc)
	uploadLimiter := platform.NewUploadRateLimiter(redisClient, cfg.UploadInitPerMinute)
	uploadSvc := service.NewUploadService(uploadRepo, mediaSvc, pool, store, thumbnailSvc, uploadLimiter, cfg.MaxPendingUploadsPerUser)

	sched := &background.Scheduler{
		Log:            logger,
		Upload:         uploadSvc,
		StorageCleanup: storageCleanup,
		DeletionRepo:   deletionRepo,
		Media:          mediaSvc,
		MediaRepo:      mediaRepo,
	}

	logger.Info("scheduler starting")
	sched.Run(ctx)
	logger.Info("scheduler stopped")
}
