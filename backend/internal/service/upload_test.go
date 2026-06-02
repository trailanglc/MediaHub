package service_test

import (
	"context"
	"os"
	"testing"

	"github.com/anhtuanlc/mediahub/internal/authz"
	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/platform"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func TestUploadInit(t *testing.T) {
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")
	cfg, err := config.Load()
	if err != nil {
		t.Skip(err)
	}
	dsn := cfg.DBDSN
	if dsn == "" {
		dsn = os.Getenv("DB_DSN")
	}
	if dsn == "" {
		t.Skip("DB_DSN not set")
	}

	ctx := context.Background()
	pool, err := platform.NewPostgresPool(ctx, dsn)
	if err != nil {
		t.Skipf("postgres: %v", err)
	}
	defer pool.Close()

	store, err := storage.NewS3Storage(ctx, cfg.Storage, cfg.HealthMetricsCacheTTL)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}

	var ownerID int64
	err = pool.QueryRow(ctx, `SELECT id FROM users WHERE role = 'owner' LIMIT 1`).Scan(&ownerID)
	if err != nil {
		t.Skipf("no owner user: %v", err)
	}

	mediaRepo := repository.NewMediaObjectRepository(pool)
	settingsRepo := repository.NewSettingsRepository(pool)
	auditRepo := repository.NewAuditRepository(pool)
	permRepo := repository.NewPermissionRepository(pool)
	settingsSvc := service.NewSettingsService(settingsRepo, auditRepo, cfg)
	authzSvc := authz.NewService(permRepo)
	uploadRepo := repository.NewUploadSessionRepository(pool)
	mediaSvc := service.NewMediaObjectService(mediaRepo, settingsSvc, authzSvc, auditRepo, store, nil, nil)
	thumbnailSvc := service.NewThumbnailService(mediaRepo, store)
	uploadSvc := service.NewUploadService(uploadRepo, mediaSvc, pool, store, thumbnailSvc, nil, 10)

	root, _ := uuid.Parse(repository.DefaultRootFolderPublicID)
	out, err := uploadSvc.Init(ctx, ownerID, "owner", service.InitUploadInput{
		ParentPublicID: root,
		FileName:       "test.png",
		Size:           1024,
		MimeType:       "image/png",
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if out.SessionPublicID == "" {
		t.Fatal("expected session id")
	}
}
