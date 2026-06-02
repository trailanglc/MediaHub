package storage_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/joho/godotenv"
)

func TestS3PutObjectIntegration(t *testing.T) {
	_ = godotenv.Load(".env")
	cfg, err := config.Load()
	if err != nil {
		t.Skip(err)
	}
	if os.Getenv("STORAGE_ENDPOINT") == "" && cfg.Storage.Endpoint == "" {
		t.Skip("storage not configured")
	}

	ctx := context.Background()
	store, err := storage.NewS3Storage(ctx, cfg.Storage, 0)
	if err != nil {
		t.Fatalf("NewS3Storage: %v", err)
	}
	if err := store.Ping(ctx); err != nil {
		t.Skipf("storage ping: %v", err)
	}

	body := []byte("mediahub upload test")
	key := "temp/test-put-" + t.Name() + ".bin"
	if err := store.PutObject(ctx, key, bytes.NewReader(body), int64(len(body)), "application/octet-stream"); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	t.Cleanup(func() {
		_ = store.DeleteObject(context.Background(), key)
	})
}
