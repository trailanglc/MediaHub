package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxDeletionAttempts = 5

type StorageDeletionJob struct {
	ID         int64
	ObjectID   int64
	StorageKey string
	IsPrefix   bool
	Status     string
	Attempts   int
	LastError  *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type StorageDeletionRepository struct {
	pool *pgxpool.Pool
}

func NewStorageDeletionRepository(pool *pgxpool.Pool) *StorageDeletionRepository {
	return &StorageDeletionRepository{pool: pool}
}

func (r *StorageDeletionRepository) Enqueue(ctx context.Context, objectID int64, storageKey string, isPrefix bool) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO storage_deletion_jobs (object_id, storage_key, is_prefix)
		SELECT $1, $2, $3
		WHERE NOT EXISTS (
			SELECT 1 FROM storage_deletion_jobs j
			WHERE j.object_id = $1 AND j.storage_key = $2
			  AND j.status IN ('pending', 'processing', 'failed')
		)
	`, objectID, storageKey, isPrefix)
	return err
}

func (r *StorageDeletionRepository) ClaimPending(ctx context.Context, limit int) ([]StorageDeletionJob, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	rows, err := tx.Query(ctx, `
		SELECT id, object_id, storage_key, is_prefix, status, attempts, last_error, created_at, updated_at
		FROM storage_deletion_jobs
		WHERE status IN ('pending', 'failed') AND attempts < $1
		ORDER BY is_prefix ASC, created_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`, maxDeletionAttempts, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	var jobs []StorageDeletionJob
	for rows.Next() {
		var j StorageDeletionJob
		if err := rows.Scan(
			&j.ID, &j.ObjectID, &j.StorageKey, &j.IsPrefix, &j.Status, &j.Attempts, &j.LastError,
			&j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			return nil, err
		}
		ids = append(ids, j.ID)
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, tx.Commit(ctx)
	}

	_, err = tx.Exec(ctx, `
		UPDATE storage_deletion_jobs
		SET status = 'processing', updated_at = now()
		WHERE id = ANY($1)
	`, ids)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	for i := range jobs {
		jobs[i].Status = "processing"
	}
	return jobs, nil
}

// ClaimFastJobsForObject claims single-key deletion jobs for one media object (not HLS prefixes).
func (r *StorageDeletionRepository) ClaimFastJobsForObject(ctx context.Context, objectID int64, limit int) ([]StorageDeletionJob, error) {
	if limit <= 0 || limit > 20 {
		limit = 8
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	rows, err := tx.Query(ctx, `
		SELECT id, object_id, storage_key, is_prefix, status, attempts, last_error, created_at, updated_at
		FROM storage_deletion_jobs
		WHERE object_id = $1
		  AND is_prefix = false
		  AND status IN ('pending', 'failed')
		  AND attempts < $2
		ORDER BY created_at ASC
		LIMIT $3
		FOR UPDATE SKIP LOCKED
	`, objectID, maxDeletionAttempts, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	var jobs []StorageDeletionJob
	for rows.Next() {
		var j StorageDeletionJob
		if err := rows.Scan(
			&j.ID, &j.ObjectID, &j.StorageKey, &j.IsPrefix, &j.Status, &j.Attempts, &j.LastError,
			&j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			return nil, err
		}
		ids = append(ids, j.ID)
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, tx.Commit(ctx)
	}

	_, err = tx.Exec(ctx, `
		UPDATE storage_deletion_jobs
		SET status = 'processing', updated_at = now()
		WHERE id = ANY($1)
	`, ids)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	for i := range jobs {
		jobs[i].Status = "processing"
	}
	return jobs, nil
}

func (r *StorageDeletionRepository) MarkDone(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE storage_deletion_jobs SET status = 'done', updated_at = now() WHERE id = $1
	`, id)
	return err
}

// RequeueStaleProcessing resets jobs left in processing (e.g. after API crash).
func (r *StorageDeletionRepository) RequeueStaleProcessing(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE storage_deletion_jobs
		SET status = 'pending', updated_at = now()
		WHERE status = 'processing' AND updated_at < now() - $1::interval
	`, fmt.Sprintf("%f seconds", olderThan.Seconds()))
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *StorageDeletionRepository) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE storage_deletion_jobs
		SET status = 'failed', attempts = attempts + 1, last_error = $2, updated_at = now()
		WHERE id = $1
	`, id, errMsg)
	return err
}

var ErrStorageDeletionJobNotFound = errors.New("storage deletion job not found")

func (r *StorageDeletionRepository) GetByID(ctx context.Context, id int64) (*StorageDeletionJob, error) {
	var j StorageDeletionJob
	err := r.pool.QueryRow(ctx, `
		SELECT id, object_id, storage_key, is_prefix, status, attempts, last_error, created_at, updated_at
		FROM storage_deletion_jobs WHERE id = $1
	`, id).Scan(
		&j.ID, &j.ObjectID, &j.StorageKey, &j.IsPrefix, &j.Status, &j.Attempts, &j.LastError,
		&j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrStorageDeletionJobNotFound
		}
		return nil, fmt.Errorf("get storage deletion job: %w", err)
	}
	return &j, nil
}

// ListActiveDeletionGuards returns storage keys/prefixes from in-flight deletion jobs.
func (r *StorageDeletionRepository) ListActiveDeletionGuards(ctx context.Context) (keys []string, prefixes []string, err error) {
	rows, err := r.pool.Query(ctx, `
		SELECT storage_key, is_prefix
		FROM storage_deletion_jobs
		WHERE status IN ('pending', 'processing', 'failed')
		  AND attempts < $1
	`, maxDeletionAttempts)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var isPrefix bool
		if err := rows.Scan(&key, &isPrefix); err != nil {
			return nil, nil, err
		}
		if isPrefix {
			prefixes = append(prefixes, key)
		} else {
			keys = append(keys, key)
		}
	}
	return keys, prefixes, rows.Err()
}
