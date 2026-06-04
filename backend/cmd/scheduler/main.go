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
	"github.com/anhtuanlc/mediahub/internal/platform/postgres"
	"github.com/anhtuanlc/mediahub/internal/platform/rediscache"
	platredis "github.com/anhtuanlc/mediahub/internal/platform/redis"
	"github.com/anhtuanlc/mediahub/internal/platform/resource"
	"github.com/anhtuanlc/mediahub/internal/platform/upload"
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

	pool, err := postgres.NewPool(ctx, cfg.DBDSN)
	if err != nil {
		logger.Fatal("postgres", zap.Error(err))
	}
	defer pool.Close()

	redisClient := platredis.NewClientFromConfig(cfg)
	defer redisClient.Close()

	store, err := storage.NewS3Storage(ctx, cfg.Storage, cfg.HealthMetricsCacheTTL)
	if err != nil {
		logger.Fatal("storage", zap.Error(err))
	}

	mediaRepo := repository.NewMediaObjectRepository(pool)
	videoRepo := repository.NewVideoRepository(pool)
	refreshRepo := repository.NewRefreshTokenRepository(pool)
	settingsRepo := repository.NewSettingsRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	permRepo := repository.NewPermissionRepository(pool)
	uploadRepo := repository.NewUploadSessionRepository(pool)
	deletionRepo := repository.NewStorageDeletionRepository(pool)

	settingsSvc := service.NewSettingsService(settingsRepo, auditRepo, cfg, rediscache.NewStore(redisClient))
	authzSvc := authz.NewService(permRepo)
	resReader := resource.NewReaderFromConfig(cfg, redisClient)
	storageCleanup := service.NewStorageCleanupService(deletionRepo, store, resReader)
	thumbnailSvc := service.NewThumbnailService(mediaRepo, store)
	mediaSvc := service.NewMediaObjectService(mediaRepo, settingsSvc, authzSvc, auditRepo, store, storageCleanup, thumbnailSvc)
	uploadLimiter := upload.NewRateLimiter(redisClient, cfg.UploadInitPerMinute)
	uploadSvc := service.NewUploadService(uploadRepo, mediaSvc, pool, store, thumbnailSvc, uploadLimiter, cfg.MaxPendingUploadsPerUser, cfg.PresignedPutURLTTL)

	sched := &background.Scheduler{
		Log:            logger,
		Upload:         uploadSvc,
		StorageCleanup: storageCleanup,
		DeletionRepo:   deletionRepo,
		Media:          mediaSvc,
		MediaRepo:      mediaRepo,
		Settings:       settingsSvc,
		Videos:         videoRepo,
		Refresh:        refreshRepo,
		Resources:      resReader,
	}

	logger.Info("scheduler starting")
	sched.Run(ctx)
	logger.Info("scheduler stopped")
}
