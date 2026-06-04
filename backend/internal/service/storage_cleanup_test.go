package service_test

import (
	"context"
	"os"
	"testing"

	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/platform/postgres"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/joho/godotenv"
)

func TestStorageDeletionJobEnqueue(t *testing.T) {
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
	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		t.Skipf("postgres: %v", err)
	}
	defer pool.Close()

	var tableExists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'storage_deletion_jobs'
		)
	`).Scan(&tableExists); err != nil || !tableExists {
		t.Skip("storage_deletion_jobs table missing — run make migrate-up")
	}

	repo := repository.NewStorageDeletionRepository(pool)
	_ = service.NewStorageCleanupService(repo, nil, nil)

	if err := repo.Enqueue(ctx, 1, "temp/test-key", false); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
}
