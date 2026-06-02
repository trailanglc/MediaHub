package storage

import (
	"context"
	"io"
	"time"
)

type ObjectStorage interface {
	PutObject(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
	DeleteObject(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	PresignGetObject(ctx context.Context, key string, ttl time.Duration) (string, error)
	Ping(ctx context.Context) error
	Stats(ctx context.Context) (*StorageStats, error)

	CreateMultipartUpload(ctx context.Context, key, contentType string) (uploadID string, err error)
	UploadPart(ctx context.Context, key, uploadID string, partNumber int32, body io.Reader, size int64) (etag string, err error)
	CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []CompletedPart) error
	AbortMultipartUpload(ctx context.Context, key, uploadID string) error

	// HashObjectSHA256 streams the object and returns a lowercase hex digest.
	HashObjectSHA256(ctx context.Context, key string, sizeBytes int64) (string, error)
	DeleteObjectsOlderThan(ctx context.Context, prefix string, olderThan time.Time) (int, error)
}
