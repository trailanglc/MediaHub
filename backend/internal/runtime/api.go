package runtime

import (
	"context"
	"net/http"
	"time"

	"github.com/anhtuanlc/mediahub/internal/auth"
	"github.com/anhtuanlc/mediahub/internal/authz"
	"github.com/anhtuanlc/mediahub/internal/handler"
	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/observability"
	"github.com/anhtuanlc/mediahub/internal/platform/cache"
	"github.com/anhtuanlc/mediahub/internal/platform/rediscache"
	"github.com/anhtuanlc/mediahub/internal/platform/session"
	convertprogress "github.com/anhtuanlc/mediahub/internal/platform/convert"
	streamplat "github.com/anhtuanlc/mediahub/internal/platform/stream"
	"github.com/anhtuanlc/mediahub/internal/platform/resource"
	"github.com/anhtuanlc/mediahub/internal/platform/upload"
	"github.com/anhtuanlc/mediahub/internal/platform/webhook"
	integrationquota "github.com/anhtuanlc/mediahub/internal/platform/integrationquota"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"go.uber.org/zap"
)

// RunAPI serves the HTTP API until ctx is cancelled.
func RunAPI(ctx context.Context, s *Shared) error {
	cfg := s.Cfg
	logger := s.Logger
	pool := s.Pool
	redisClient := s.Redis
	store := s.Store

	resReader := resource.NewReaderFromConfig(cfg, redisClient)
	redisCache := rediscache.NewStore(redisClient).WithResources(resReader)

	userRepo := repository.NewUserRepository(pool)
	refreshRepo := repository.NewRefreshTokenRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	permRepo := repository.NewPermissionRepository(pool)
	mediaRepo := repository.NewMediaObjectRepository(pool)

	tokenRevoke := session.NewTokenRevocation(redisClient)
	sessionInvalidate := session.NewSessionInvalidation(redisClient, cfg.JWTRefreshTTL)
	loginLimiter := session.NewLoginRateLimiter(redisClient, cfg.LoginMaxAttempts, cfg.LoginLockoutWindow, cfg.RedisFailClosed)

	authSvc := service.NewAuthService(userRepo, refreshRepo, auditRepo, tokenRevoke, loginLimiter, cfg)
	memberSvc := service.NewMemberService(userRepo, permRepo, refreshRepo, auditRepo, sessionInvalidate, redisCache)
	permSvc := service.NewPermissionService(permRepo, userRepo, mediaRepo, auditRepo)
	settingsRepo := repository.NewSettingsRepository(pool)
	settingsSvc := service.NewSettingsService(settingsRepo, auditRepo, cfg, redisCache)
	authzSvc := authz.NewService(permRepo)
	uploadRepo := repository.NewUploadSessionRepository(pool)
	deletionRepo := repository.NewStorageDeletionRepository(pool)
	storageCleanup := service.NewStorageCleanupService(deletionRepo, store, resReader)
	thumbnailSvc := service.NewThumbnailService(mediaRepo, store)
	mediaSvc := service.NewMediaObjectService(mediaRepo, settingsSvc, authzSvc, auditRepo, store, storageCleanup, thumbnailSvc)
	uploadLimiter := upload.NewRateLimiter(redisClient, cfg.UploadInitPerMinute)
	uploadSvc := service.NewUploadService(uploadRepo, mediaSvc, pool, store, thumbnailSvc, uploadLimiter, cfg.MaxPendingUploadsPerUser, cfg.PresignedPutURLTTL)

	webhookRepo := repository.NewWebhookRepository(pool)
	webhookDispatcher := webhook.NewDispatcher(webhookRepo, logger)
	webhookSvc := service.NewWebhookService(webhookRepo)
	uploadSvc.SetWebhooks(webhookDispatcher)
	mediaSvc.SetWebhooks(webhookDispatcher)

	authUserCache := &rediscache.AuthUserCache{Store: redisCache, Users: userRepo}
	authMW := middleware.NewAuthMiddleware(authSvc.Issuer(), tokenRevoke, sessionInvalidate, userRepo, authUserCache)

	health := &handler.HealthHandler{
		DB:              pool,
		Redis:           redisClient,
		Storage:         store,
		MetricsCacheTTL: cfg.HealthMetricsCacheTTL,
		HostCache:       cache.NewTTLCache[*observability.HostStats](cfg.HealthMetricsCacheTTL),
		Resources:       resReader,
	}

	passwordCipher, err := auth.NewPasswordCipher(cfg.LoginRSAPrivateKeyPEM)
	if err != nil {
		return err
	}
	passwordTransport := &handler.PasswordTransport{
		Cipher:           passwordCipher,
		RequireEncrypted: cfg.RequireEncryptedPassword,
	}

	setupSvc := service.NewSetupService(userRepo, cfg)
	setupHandler := handler.NewSetupHandler(setupSvc, passwordTransport)

	maintenanceSvc := service.NewMaintenanceService(storageCleanup, uploadSvc, mediaRepo, deletionRepo, uploadRepo, store, auditRepo)
	systemHandler := handler.NewSystemHandler(maintenanceSvc)

	videoRepo := repository.NewVideoRepository(pool)
	apiKeyRepo := repository.NewAPIKeyRepository(pool)
	convertEnqueue := service.NewConvertEnqueue(cfg.RedisAddr, cfg.ConvertQueueMaxDepth)
	defer convertEnqueue.Close() //nolint:errcheck
	health.ConvertQueue = convertEnqueue
	streamTok := service.NewStreamTokenService(cfg.StreamSigningSecret)
	convertProg := convertprogress.NewProgressStore(redisClient)
	videoSvc := service.NewVideoService(videoRepo, mediaRepo, authzSvc, auditRepo, settingsSvc, store, convertEnqueue, streamTok, cfg.APIPublicURL, convertProg, redisCache, resReader)
	videoSvc.SetWebhooks(webhookDispatcher)
	streamLoader := &rediscache.StreamLoader{
		Store:         redisCache,
		Videos:        videoRepo,
		GlobalDomains: settingsSvc.GlobalAllowedDomains,
	}
	apiKeyCache := &rediscache.APIKeyCache{Store: redisCache, Keys: apiKeyRepo}
	apiKeySvc := service.NewAPIKeyService(apiKeyRepo, auditRepo, mediaRepo, apiKeyCache)
	deliveryBase := cfg.DeliveryBaseURL()
	deliverySvc := service.NewDeliveryService(streamTok, deliveryBase, cfg.AssetDeliveryURLTTL)
	imageTransform := service.NewImageTransformService(store, redisCache, cfg.ImageTransformCacheTTL)
	integrationQuota := integrationquota.NewLimiter(redisClient, cfg.APIKeyUploadInitPerMin, cfg.APIKeyConvertPerHour)
	integrationSvc := service.NewIntegrationService(mediaRepo, uploadSvc, mediaSvc, videoSvc, deliverySvc, integrationQuota)
	streamMetrics := streamplat.NewMetrics(redisClient)
	streamLimiter := streamplat.NewRateLimiter(redisClient, cfg.StreamRateLimitPerMin, cfg.RedisFailClosed).
		WithSegmentLimit(cfg.StreamSegmentRateLimitPerMin)

	queueHandler := handler.NewQueueHandler(videoRepo, videoSvc, mediaRepo, deletionRepo, convertEnqueue, auditRepo, cfg.RedisAddr, streamMetrics)
	defer queueHandler.Close() //nolint:errcheck

	systemInfo := &handler.SystemInfoHandler{
		Health:   health,
		Queue:    queueHandler,
		APIKeys:  apiKeyRepo,
		Settings: settingsSvc,
		Cfg:      cfg,
	}

	if cfg.AppEnv == "development" {
		if cfg.JWTSecret == "" {
			logger.Warn("JWT_SECRET unset — using insecure dev default; set secrets before production")
		}
		if cfg.StreamSigningSecret == "dev_stream_signing_secret" {
			logger.Warn("STREAM_SIGNING_SECRET unset — using dev default")
		}
	}

	homepageAssets := service.NewHomepageAssetResolver(mediaRepo, deliverySvc)

	router := handler.NewRouter(handler.RouterDeps{
		Logger:     logger,
		Health:     health,
		SystemInfo: systemInfo,
		System:     systemHandler,
		Audit:      handler.NewAuditHandler(auditRepo),
		Queue:      queueHandler,
		Setup:      setupHandler,
		Auth:       handler.NewAuthHandler(authSvc, passwordTransport, logger),
		Member:     handler.NewMemberHandler(memberSvc, passwordTransport),
		Perm:       handler.NewPermissionHandler(permSvc, authzSvc, mediaRepo),
		Settings:   handler.NewSettingsHandler(settingsSvc, homepageAssets, logger),
		Objects:    handler.NewObjectHandler(mediaSvc),
		Upload:     handler.NewUploadHandler(uploadSvc, logger),
		Videos:     handler.NewVideoHandler(videoSvc, store),
		APIKeys:    handler.NewAPIKeyHandler(apiKeySvc),
		Webhooks:   handler.NewWebhookHandler(webhookSvc),
		Integration: handler.NewIntegrationV1Handler(integrationSvc),
		Assets:      handler.NewAssetDeliveryHandler(mediaRepo, videoSvc, deliverySvc, imageTransform, store),
		Embed:       handler.NewEmbedHandler(videoRepo, deliverySvc, streamTok, deliveryBase),
		OEmbed:      handler.NewOEmbedHandler(videoRepo, mediaRepo, deliverySvc, deliveryBase, cfg.AppURL, cfg.APIPublicURL),
		APIKeySvc:   apiKeySvc,
		Stream: handler.NewStreamHandler(streamLoader, streamTok, store, apiKeySvc, integrationSvc, streamMetrics, streamLimiter, cfg.AppURL, cfg.AppEnv, authMW, authzSvc, handler.StreamHandlerOptions{
			SegmentURLWindow:       cfg.StreamSegmentURLTTL,
			InternalRedirectPrefix: cfg.StreamInternalRedirectPrefix,
		}),
		Password:       passwordTransport,
		AuthMW:         authMW,
		AppURL:         cfg.AppURL,
		AppEnv:         cfg.AppEnv,
		TrustedProxies: cfg.TrustedProxies,
	})

	srv := &http.Server{
		Addr:         cfg.APIAddr,
		Handler:      router,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("api listening", zap.String("addr", cfg.APIAddr))
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("api shutdown", zap.Error(err))
		}
		return ctx.Err()
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
