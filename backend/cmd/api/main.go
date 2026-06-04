package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anhtuanlc/mediahub/internal/auth"
	"github.com/anhtuanlc/mediahub/internal/authz"
	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/handler"
	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/observability"
	"github.com/anhtuanlc/mediahub/internal/platform"
	"github.com/anhtuanlc/mediahub/internal/platform/cache"
	"github.com/anhtuanlc/mediahub/internal/platform/postgres"
	"github.com/anhtuanlc/mediahub/internal/platform/rediscache"
	platredis "github.com/anhtuanlc/mediahub/internal/platform/redis"
	"github.com/anhtuanlc/mediahub/internal/platform/session"
	convertprogress "github.com/anhtuanlc/mediahub/internal/platform/convert"
	streamplat "github.com/anhtuanlc/mediahub/internal/platform/stream"
	"github.com/anhtuanlc/mediahub/internal/platform/resource"
	"github.com/anhtuanlc/mediahub/internal/platform/upload"
	"github.com/anhtuanlc/mediahub/internal/platform/webhook"
	integrationquota "github.com/anhtuanlc/mediahub/internal/platform/integrationquota"
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

	ctx := context.Background()

	pool, err := postgres.NewPool(ctx, cfg.DBDSN)
	if err != nil {
		logger.Fatal("postgres", zap.Error(err))
	}
	defer pool.Close()

	var settingsTable bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'system_settings'
		)
	`).Scan(&settingsTable); err != nil {
		logger.Warn("settings table check failed", zap.Error(err))
	} else if !settingsTable {
		logger.Warn("system_settings table missing — run: make migrate-up")
	}

	var uploadSessionsTable bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'upload_sessions'
		)
	`).Scan(&uploadSessionsTable); err != nil {
		logger.Warn("upload_sessions table check failed", zap.Error(err))
	} else if !uploadSessionsTable {
		logger.Warn("upload_sessions table missing — run: make migrate-up (migration 000006)")
	}

	redisClient := platredis.NewClientFromConfig(cfg)
	resReader := resource.NewReaderFromConfig(cfg, redisClient)
	redisCache := rediscache.NewStore(redisClient).WithResources(resReader)

	store, err := storage.NewS3Storage(ctx, cfg.Storage, cfg.HealthMetricsCacheTTL)
	if err != nil {
		logger.Fatal("storage", zap.Error(err))
	}
	if cfg.AppEnv == "development" {
		if err := store.EnsureBucket(ctx); err != nil {
			logger.Warn("storage ensure bucket", zap.Error(err))
		}
	}

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
		logger.Fatal("password cipher", zap.Error(err))
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
	systemInfo := &handler.SystemInfoHandler{
		Health:   health,
		APIKeys:  apiKeyRepo,
		Settings: settingsSvc,
		Cfg:      cfg,
	}
	convertEnqueue := service.NewConvertEnqueue(cfg.RedisAddr)
	defer convertEnqueue.Close() //nolint:errcheck
	streamTok := service.NewStreamTokenService(cfg.StreamSigningSecret)
	convertProg := convertprogress.NewProgressStore(redisClient)
	videoSvc := service.NewVideoService(videoRepo, mediaRepo, authzSvc, auditRepo, settingsSvc, store, convertEnqueue, streamTok, cfg.APIPublicURL, convertProg, redisCache, resReader)
	videoSvc.SetWebhooks(webhookDispatcher)
	streamLoader := &rediscache.StreamLoader{
		Store:         redisCache,
		Videos:        videoRepo,
		GlobalDomains: settingsSvc.GlobalAllowedDomains,
	}
	apiKeySvc := service.NewAPIKeyService(apiKeyRepo, auditRepo, mediaRepo)
	deliveryBase := cfg.DeliveryBaseURL()
	deliverySvc := service.NewDeliveryService(streamTok, deliveryBase, cfg.AssetDeliveryURLTTL)
	imageTransform := service.NewImageTransformService(store, redisCache, cfg.ImageTransformCacheTTL)
	integrationQuota := integrationquota.NewLimiter(redisClient, cfg.APIKeyUploadInitPerMin, cfg.APIKeyConvertPerHour)
	integrationSvc := service.NewIntegrationService(mediaRepo, uploadSvc, mediaSvc, videoSvc, deliverySvc, integrationQuota)
	streamMetrics := streamplat.NewMetrics(redisClient)
	streamLimiter := streamplat.NewRateLimiter(redisClient, cfg.StreamRateLimitPerMin, cfg.RedisFailClosed).
		WithSegmentLimit(cfg.StreamSegmentRateLimitPerMin)

	s3Store := store

	queueHandler := handler.NewQueueHandler(videoRepo, mediaRepo, cfg.RedisAddr, streamMetrics)
	defer queueHandler.Close() //nolint:errcheck

	if cfg.AppEnv == "development" {
		if cfg.JWTSecret == "" {
			logger.Warn("JWT_SECRET unset — using insecure dev default; set secrets before production")
		}
		if cfg.StreamSigningSecret == "dev_stream_signing_secret" {
			logger.Warn("STREAM_SIGNING_SECRET unset — using dev default")
		}
	}

	router := handler.NewRouter(handler.RouterDeps{
		Logger:     logger,
		Health:     health,
		SystemInfo: systemInfo,
		System:     systemHandler,
		Queue:    queueHandler,
		Setup:    setupHandler,
		Auth:     handler.NewAuthHandler(authSvc, passwordTransport),
		Member:   handler.NewMemberHandler(memberSvc, passwordTransport),
		Perm:     handler.NewPermissionHandler(permSvc),
		Settings: handler.NewSettingsHandler(settingsSvc, logger),
		Objects:  handler.NewObjectHandler(mediaSvc),
		Upload:   handler.NewUploadHandler(uploadSvc, logger),
		Videos:   handler.NewVideoHandler(videoSvc, s3Store),
		APIKeys:  handler.NewAPIKeyHandler(apiKeySvc),
		Webhooks: handler.NewWebhookHandler(webhookSvc),
		Integration: handler.NewIntegrationV1Handler(integrationSvc),
		Assets:      handler.NewAssetDeliveryHandler(mediaRepo, videoSvc, deliverySvc, imageTransform, store),
		Embed:       handler.NewEmbedHandler(videoRepo, deliverySvc, streamTok, deliveryBase),
		OEmbed:      handler.NewOEmbedHandler(videoRepo, mediaRepo, deliverySvc, deliveryBase, cfg.AppURL, cfg.APIPublicURL),
		APIKeySvc:   apiKeySvc,
		Stream: handler.NewStreamHandler(streamLoader, streamTok, store, apiKeySvc, integrationSvc, streamMetrics, streamLimiter, cfg.AppURL, cfg.AppEnv, authMW, authzSvc, handler.StreamHandlerOptions{
			SegmentURLWindow:       cfg.StreamSegmentURLTTL,
			InternalRedirectPrefix: cfg.StreamInternalRedirectPrefix,
		}),
		Password: passwordTransport,
		AuthMW:   authMW,
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

	go func() {
		logger.Info("api listening", zap.String("addr", cfg.APIAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", zap.Error(err))
	}
	_ = redisClient.Close()
}
