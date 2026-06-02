package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/platform"
	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Storage struct {
	client     *s3.Client
	bucket     string
	endpoint   string
	accessKey  string
	secretKey  string
	quotaBytes int64
	statsCache *platform.TTLCache[*StorageStats]
}

func NewS3Storage(ctx context.Context, cfg config.StorageConfig, metricsCacheTTL time.Duration) (*S3Storage, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("storage endpoint and bucket are required")
	}

	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
		if service == s3.ServiceID {
			return aws.Endpoint{
				URL:               cfg.Endpoint,
				SigningRegion:     cfg.Region,
				HostnameImmutable: true,
			}, nil
		}
		return aws.Endpoint{}, fmt.Errorf("unknown endpoint service: %s", service)
	})

	awsCfg, err := awscfg.LoadDefaultConfig(ctx,
		awscfg.WithRegion(cfg.Region),
		awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		)),
		awscfg.WithEndpointResolverWithOptions(resolver),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
		// MinIO and other S3-compatible stores often reject default AWS SDK CRC32 checksums.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})

	var statsCache *platform.TTLCache[*StorageStats]
	if metricsCacheTTL > 0 {
		statsCache = platform.NewTTLCache[*StorageStats](metricsCacheTTL)
	}
	return &S3Storage{
		client:     client,
		bucket:     cfg.Bucket,
		endpoint:   cfg.Endpoint,
		accessKey:  cfg.AccessKey,
		secretKey:  cfg.SecretKey,
		quotaBytes: cfg.QuotaBytes,
		statsCache: statsCache,
	}, nil
}

// StatsCacheAt returns when storage stats were last computed (UTC).
func (s *S3Storage) StatsCacheAt() time.Time {
	if s.statsCache == nil {
		return time.Time{}
	}
	return s.statsCache.CachedAt()
}

// StatsCacheTTL returns the configured cache duration for storage stats.
func (s *S3Storage) StatsCacheTTL() time.Duration {
	if s.statsCache == nil {
		return 0
	}
	return s.statsCache.TTL()
}

// EnsureBucket creates the bucket if missing (dev convenience).
func (s *S3Storage) EnsureBucket(ctx context.Context) error {
	_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		var exists *types.BucketAlreadyExists
		var owned *types.BucketAlreadyOwnedByYou
		if errors.As(err, &exists) || errors.As(err, &owned) {
			return nil
		}
		return err
	}
	return nil
}

func (s *S3Storage) Client() *s3.Client {
	return s.client
}

func (s *S3Storage) Bucket() string {
	return s.bucket
}

func (s *S3Storage) Ping(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	return err
}

func (s *S3Storage) PutObject(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	in := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if size >= 0 {
		in.ContentLength = aws.Int64(size)
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	_, err := s.client.PutObject(ctx, in)
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}
	return nil
}

func (s *S3Storage) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get object %s: %w", key, err)
	}
	return out.Body, nil
}

func (s *S3Storage) DeleteObject(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object %s: %w", key, err)
	}
	return nil
}

// DeletePrefix removes all objects under a prefix using batched DeleteObjects (for HLS/temp).
func (s *S3Storage) DeletePrefix(ctx context.Context, prefix string) error {
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("list prefix %s: %w", prefix, err)
		}
		if len(page.Contents) == 0 {
			continue
		}
		keys := make([]types.ObjectIdentifier, 0, len(page.Contents))
		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, types.ObjectIdentifier{Key: obj.Key})
			}
		}
		if len(keys) == 0 {
			continue
		}
		if err := s.deleteObjectBatch(ctx, keys); err != nil {
			return fmt.Errorf("delete prefix %s: %w", prefix, err)
		}
	}
	return nil
}

func (s *S3Storage) deleteObjectBatch(ctx context.Context, keys []types.ObjectIdentifier) error {
	const batchSize = 1000
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}
		chunk := keys[i:end]
		out, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &types.Delete{
				Objects: chunk,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return err
		}
		if len(out.Errors) > 0 {
			e := out.Errors[0]
			return fmt.Errorf("delete objects: %s %s", aws.ToString(e.Key), aws.ToString(e.Message))
		}
	}
	return nil
}

func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *types.NotFound
		if errors.As(err, &nsk) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *S3Storage) CreateMultipartUpload(ctx context.Context, key, contentType string) (string, error) {
	in := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	out, err := s.client.CreateMultipartUpload(ctx, in)
	if err != nil {
		return "", fmt.Errorf("create multipart upload %s: %w", key, err)
	}
	if out.UploadId == nil || *out.UploadId == "" {
		return "", fmt.Errorf("create multipart upload %s: empty upload id", key)
	}
	return *out.UploadId, nil
}

func (s *S3Storage) UploadPart(ctx context.Context, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	in := &s3.UploadPartInput{
		Bucket:     aws.String(s.bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(partNumber),
		Body:       body,
	}
	if size >= 0 {
		in.ContentLength = aws.Int64(size)
	}
	out, err := s.client.UploadPart(ctx, in)
	if err != nil {
		return "", fmt.Errorf("upload part %s#%d: %w", key, partNumber, err)
	}
	if out.ETag == nil || *out.ETag == "" {
		return "", fmt.Errorf("upload part %s#%d: empty etag", key, partNumber)
	}
	return *out.ETag, nil
}

func (s *S3Storage) CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []CompletedPart) error {
	if len(parts) == 0 {
		return fmt.Errorf("complete multipart %s: no parts", key)
	}
	sorted := make([]CompletedPart, len(parts))
	copy(sorted, parts)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].PartNumber < sorted[j].PartNumber
	})
	completed := make([]types.CompletedPart, 0, len(sorted))
	for _, p := range sorted {
		completed = append(completed, types.CompletedPart{
			PartNumber: aws.Int32(p.PartNumber),
			ETag:       aws.String(p.ETag),
		})
	}
	_, err := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completed,
		},
	})
	if err != nil {
		return fmt.Errorf("complete multipart %s: %w", key, err)
	}
	return nil
}

func (s *S3Storage) AbortMultipartUpload(ctx context.Context, key, uploadID string) error {
	_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	if err != nil {
		return fmt.Errorf("abort multipart %s: %w", key, err)
	}
	return nil
}

func (s *S3Storage) HashObjectSHA256(ctx context.Context, key string, sizeBytes int64) (string, error) {
	body, err := s.GetObject(ctx, key)
	if err != nil {
		return "", err
	}
	defer body.Close()

	h := sha256.New()
	limit := sizeBytes + 1
	if sizeBytes <= 0 {
		limit = -1
	}
	var reader io.Reader = body
	if limit > 0 {
		reader = io.LimitReader(body, limit)
	}
	written, err := io.Copy(h, reader)
	if err != nil {
		return "", fmt.Errorf("hash object %s: %w", key, err)
	}
	if sizeBytes > 0 && written != sizeBytes {
		return "", fmt.Errorf("hash object %s: size mismatch got %d want %d", key, written, sizeBytes)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ListObjectKeys returns all object keys under prefix (paginated).
func (s *S3Storage) ListObjectKeys(ctx context.Context, prefix string) ([]string, error) {
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	var keys []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return keys, fmt.Errorf("list prefix %s: %w", prefix, err)
		}
		for _, obj := range page.Contents {
			if obj.Key != nil && *obj.Key != "" {
				keys = append(keys, *obj.Key)
			}
		}
	}
	return keys, nil
}

// DeleteObjectsOlderThan removes objects under prefix with LastModified before cutoff.
func (s *S3Storage) DeleteObjectsOlderThan(ctx context.Context, prefix string, olderThan time.Time) (int, error) {
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	var deleted int
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return deleted, fmt.Errorf("list prefix %s: %w", prefix, err)
		}
		for _, obj := range page.Contents {
			if obj.Key == nil || obj.LastModified == nil {
				continue
			}
			if !obj.LastModified.Before(olderThan) {
				continue
			}
			if err := s.DeleteObject(ctx, *obj.Key); err != nil {
				return deleted, err
			}
			deleted++
		}
	}
	return deleted, nil
}

func (s *S3Storage) PresignGetObject(ctx context.Context, key string, ttl time.Duration) (string, error) {
	presigner := s3.NewPresignClient(s.client)
	out, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign get %s: %w", key, err)
	}
	return out.URL, nil
}
