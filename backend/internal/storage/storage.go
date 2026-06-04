package storage

import (
	"context"
	"io"
	"time"
)

type ObjectStorage interface {
	PutObject(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
	// StatObject returns size and metadata without downloading the body.
	StatObject(ctx context.Context, key string) (*ObjectInfo, error)
	// GetObjectRange reads [offset, offset+length); length < 0 means to end of object.
	GetObjectRange(ctx context.Context, key string, offset, length int64) (io.ReadCloser, *ObjectInfo, error)
	// GetRange fetches an object, passing through a raw HTTP Range header ("" = full object).
	// The returned ObjectInfo carries Size (body length), TotalSize, ETag, ContentType and
	// ContentRange (set for 206 partial responses). This avoids a separate HeadObject round-trip.
	GetRange(ctx context.Context, key, rangeHeader string) (io.ReadCloser, *ObjectInfo, error)
	DeleteObject(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	PresignGetObject(ctx context.Context, key string, ttl time.Duration) (string, error)
	// PresignPutObject returns a URL for a single PUT upload with fixed size and content type.
	PresignPutObject(ctx context.Context, key, contentType string, size int64, ttl time.Duration) (string, error)
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
