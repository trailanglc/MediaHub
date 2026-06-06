package runtime

import (
	"context"

	"github.com/anhtuanlc/mediahub/internal/authz"
	"github.com/anhtuanlc/mediahub/internal/background"
	"github.com/anhtuanlc/mediahub/internal/platform/rediscache"
	"github.com/anhtuanlc/mediahub/internal/platform/resource"
	"github.com/anhtuanlc/mediahub/internal/platform/upload"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
)

// RunScheduler runs periodic maintenance jobs until ctx is cancelled.
func RunScheduler(ctx context.Context, s *Shared) error {
	cfg := s.Cfg
	logger := s.Logger
	pool := s.Pool
	redisClient := s.Redis
	store := s.Store

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
	return ctx.Err()
}
