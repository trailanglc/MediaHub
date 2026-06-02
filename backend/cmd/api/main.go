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

	ctx := context.Background()

	pool, err := platform.NewPostgresPool(ctx, cfg.DBDSN)
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

	redisClient := platform.NewRedisClient(cfg.RedisAddr)

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

	tokenRevoke := platform.NewTokenRevocation(redisClient)
	sessionInvalidate := platform.NewSessionInvalidation(redisClient)
	loginLimiter := platform.NewLoginRateLimiter(redisClient, cfg.LoginMaxAttempts, cfg.LoginLockoutWindow)

	authSvc := service.NewAuthService(userRepo, refreshRepo, auditRepo, tokenRevoke, loginLimiter, cfg)
	memberSvc := service.NewMemberService(userRepo, permRepo, refreshRepo, auditRepo, sessionInvalidate)
	permSvc := service.NewPermissionService(permRepo, userRepo, mediaRepo, auditRepo)
	settingsRepo := repository.NewSettingsRepository(pool)
	settingsSvc := service.NewSettingsService(settingsRepo, auditRepo, cfg)
	authzSvc := authz.NewService(permRepo)
	uploadRepo := repository.NewUploadSessionRepository(pool)
	deletionRepo := repository.NewStorageDeletionRepository(pool)
	storageCleanup := service.NewStorageCleanupService(deletionRepo, store)
	thumbnailSvc := service.NewThumbnailService(mediaRepo, store)
	mediaSvc := service.NewMediaObjectService(mediaRepo, settingsSvc, authzSvc, auditRepo, store, storageCleanup, thumbnailSvc)
	uploadLimiter := platform.NewUploadRateLimiter(redisClient, cfg.UploadInitPerMinute)
	uploadSvc := service.NewUploadService(uploadRepo, mediaSvc, pool, store, thumbnailSvc, uploadLimiter, cfg.MaxPendingUploadsPerUser)

	authMW := middleware.NewAuthMiddleware(authSvc.Issuer(), tokenRevoke, sessionInvalidate, userRepo)

	health := &handler.HealthHandler{
		DB:              pool,
		Redis:           redisClient,
		Storage:         store,
		MetricsCacheTTL: cfg.HealthMetricsCacheTTL,
		HostCache:       platform.NewTTLCache[*platform.HostStats](cfg.HealthMetricsCacheTTL),
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

	router := handler.NewRouter(handler.RouterDeps{
		Logger:   logger,
		Health:   health,
		System:   systemHandler,
		Setup:    setupHandler,
		Auth:     handler.NewAuthHandler(authSvc, passwordTransport),
		Member:   handler.NewMemberHandler(memberSvc, passwordTransport),
		Perm:     handler.NewPermissionHandler(permSvc),
		Settings: handler.NewSettingsHandler(settingsSvc, logger),
		Objects:  handler.NewObjectHandler(mediaSvc),
		Upload:   handler.NewUploadHandler(uploadSvc, logger),
		Password: passwordTransport,
		AuthMW:   authMW,
		AppURL:   cfg.AppURL,
		AppEnv:   cfg.AppEnv,
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
