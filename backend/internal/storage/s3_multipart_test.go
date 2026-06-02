package storage_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/joho/godotenv"
)

func TestS3MultipartUploadIntegration(t *testing.T) {
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

	key := "temp/test-multipart-" + t.Name() + ".bin"
	body := []byte("mediahub multipart part 1")
	uploadID, err := store.CreateMultipartUpload(ctx, key, "application/octet-stream")
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}
	t.Cleanup(func() {
		_ = store.AbortMultipartUpload(context.Background(), key, uploadID)
		_ = store.DeleteObject(context.Background(), key)
	})

	etag, err := store.UploadPart(ctx, key, uploadID, 1, bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("UploadPart: %v", err)
	}
	if err := store.CompleteMultipartUpload(ctx, key, uploadID, []storage.CompletedPart{
		{PartNumber: 1, ETag: etag},
	}); err != nil {
		t.Fatalf("CompleteMultipartUpload: %v", err)
	}

	rc, err := store.GetObject(ctx, key)
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read object: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("content mismatch: %q vs %q", got, body)
	}
}
