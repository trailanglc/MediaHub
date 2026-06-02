package reset

import (
	"context"
	"fmt"
	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const seedRootFolderSQL = `
INSERT INTO media_objects (public_id, parent_id, type, name, status)
SELECT '00000000-0000-4000-8000-000000000001'::uuid, NULL, 'folder', 'Root', 'active'
WHERE NOT EXISTS (
    SELECT 1 FROM media_objects WHERE public_id = '00000000-0000-4000-8000-000000000001'::uuid
);

INSERT INTO object_paths (ancestor_id, descendant_id, depth)
SELECT m.id, m.id, 0
FROM media_objects m
WHERE m.public_id = '00000000-0000-4000-8000-000000000001'::uuid
  AND NOT EXISTS (
    SELECT 1 FROM object_paths op
    WHERE op.ancestor_id = m.id AND op.descendant_id = m.id
  );
`

const truncateSQL = `
TRUNCATE TABLE
    storage_deletion_jobs,
    upload_sessions,
    refresh_tokens,
    audit_logs,
    convert_jobs,
    stream_policies,
    permissions,
    api_keys,
    video_assets,
    object_paths,
    media_objects,
    users
RESTART IDENTITY CASCADE;
`

type Result struct {
	PostgresTruncated bool
	RootFolderSeeded  bool
	RedisFlushed      bool
	StorageObjects    int
	StorageSkipped    bool
}

func Run(ctx context.Context, cfg *config.Config) (Result, error) {
	var res Result

	pool, err := pgxpool.New(ctx, cfg.DBDSN)
	if err != nil {
		return res, fmt.Errorf("postgres connect: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return res, fmt.Errorf("postgres ping: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return res, fmt.Errorf("postgres begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, truncateSQL); err != nil {
		return res, fmt.Errorf("postgres truncate: %w", err)
	}
	if _, err := tx.Exec(ctx, seedRootFolderSQL); err != nil {
		return res, fmt.Errorf("postgres seed root: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("postgres commit: %w", err)
	}
	res.PostgresTruncated = true
	res.RootFolderSeeded = true

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return res, fmt.Errorf("redis ping: %w", err)
	}
	if err := rdb.FlushAll(ctx).Err(); err != nil {
		return res, fmt.Errorf("redis flush: %w", err)
	}
	res.RedisFlushed = true

	if cfg.Storage.Endpoint == "" || cfg.Storage.Bucket == "" {
		res.StorageSkipped = true
		return res, nil
	}

	store, err := storage.NewS3Storage(ctx, cfg.Storage, 0)
	if err != nil {
		return res, fmt.Errorf("storage client: %w", err)
	}
	n, err := emptyBucket(ctx, store)
	if err != nil {
		return res, fmt.Errorf("storage empty bucket: %w", err)
	}
	res.StorageObjects = n
	return res, nil
}

func emptyBucket(ctx context.Context, store *storage.S3Storage) (int, error) {
	client := store.Client()
	bucket := store.Bucket()
	var deleted int
	var continuation *string

	for {
		out, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			ContinuationToken: continuation,
		})
		if err != nil {
			return deleted, err
		}

		if len(out.Contents) == 0 {
			if !aws.ToBool(out.IsTruncated) {
				break
			}
			continuation = out.NextContinuationToken
			continue
		}

		objects := make([]types.ObjectIdentifier, 0, len(out.Contents))
		for _, obj := range out.Contents {
			objects = append(objects, types.ObjectIdentifier{Key: obj.Key})
		}

		_, err = client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &types.Delete{
				Objects: objects,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return deleted, err
		}
		deleted += len(objects)

		if !aws.ToBool(out.IsTruncated) {
			break
		}
		continuation = out.NextContinuationToken
	}

	return deleted, nil
}
