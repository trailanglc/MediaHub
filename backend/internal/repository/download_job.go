package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrDownloadJobNotFound = errors.New("download job not found")
var ErrDownloadJobNotRunnable = errors.New("download job not runnable")

type DownloadJob struct {
	ID             int64
	PublicID       uuid.UUID
	CreatedBy      *int64
	SourceURL      string
	ResolvedURL    *string
	Kind           string
	Status         string
	Attempts       int
	MaxAttempts    int
	Title          *string
	ThumbnailURL   *string
	MimeHint       *string
	SelectedFormat json.RawMessage
	ParentFolderID *int64
	VideoPublicID  *uuid.UUID
	ObjectID       *int64
	ProgressPct    int
	BytesDone      int64
	BytesTotal     int64
	LastError      *string
	StartedAt      *time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateDownloadJobInput struct {
	PublicID       uuid.UUID
	CreatedBy      int64
	SourceURL      string
	ResolvedURL    *string
	Kind           string
	Title          *string
	ThumbnailURL   *string
	MimeHint       *string
	SelectedFormat json.RawMessage
	ParentFolderID *int64
	MaxAttempts    int
}

type DownloadJobRepository struct {
	pool *pgxpool.Pool
}

func NewDownloadJobRepository(pool *pgxpool.Pool) *DownloadJobRepository {
	return &DownloadJobRepository{pool: pool}
}

func (r *DownloadJobRepository) Create(ctx context.Context, in CreateDownloadJobInput) (*DownloadJob, error) {
	if in.MaxAttempts <= 0 {
		in.MaxAttempts = 3
	}
	if len(in.SelectedFormat) == 0 {
		in.SelectedFormat = json.RawMessage(`{}`)
	}
	row := &DownloadJob{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO download_jobs (
			public_id, created_by, source_url, resolved_url, kind, status,
			max_attempts, title, thumbnail_url, mime_hint, selected_format, parent_folder_id
		) VALUES ($1,$2,$3,$4,$5,'pending',$6,$7,$8,$9,$10,$11)
		RETURNING id, public_id, created_by, source_url, resolved_url, kind, status,
			attempts, max_attempts, title, thumbnail_url, mime_hint, selected_format,
			parent_folder_id, video_public_id, object_id, progress_pct, bytes_done, bytes_total,
			last_error, started_at, finished_at, created_at, updated_at
	`, in.PublicID, in.CreatedBy, in.SourceURL, in.ResolvedURL, in.Kind, in.MaxAttempts,
		in.Title, in.ThumbnailURL, in.MimeHint, in.SelectedFormat, in.ParentFolderID,
	).Scan(
		&row.ID, &row.PublicID, &row.CreatedBy, &row.SourceURL, &row.ResolvedURL, &row.Kind, &row.Status,
		&row.Attempts, &row.MaxAttempts, &row.Title, &row.ThumbnailURL, &row.MimeHint, &row.SelectedFormat,
		&row.ParentFolderID, &row.VideoPublicID, &row.ObjectID, &row.ProgressPct, &row.BytesDone, &row.BytesTotal,
		&row.LastError, &row.StartedAt, &row.FinishedAt, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (r *DownloadJobRepository) GetByPublicID(ctx context.Context, publicID uuid.UUID) (*DownloadJob, error) {
	row := &DownloadJob{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, public_id, created_by, source_url, resolved_url, kind, status,
			attempts, max_attempts, title, thumbnail_url, mime_hint, selected_format,
			parent_folder_id, video_public_id, object_id, progress_pct, bytes_done, bytes_total,
			last_error, started_at, finished_at, created_at, updated_at
		FROM download_jobs WHERE public_id = $1
	`, publicID).Scan(
		&row.ID, &row.PublicID, &row.CreatedBy, &row.SourceURL, &row.ResolvedURL, &row.Kind, &row.Status,
		&row.Attempts, &row.MaxAttempts, &row.Title, &row.ThumbnailURL, &row.MimeHint, &row.SelectedFormat,
		&row.ParentFolderID, &row.VideoPublicID, &row.ObjectID, &row.ProgressPct, &row.BytesDone, &row.BytesTotal,
		&row.LastError, &row.StartedAt, &row.FinishedAt, &row.CreatedAt, &row.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDownloadJobNotFound
	}
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (r *DownloadJobRepository) ListByUser(ctx context.Context, userID int64, cursor int64, limit int) ([]DownloadJob, *int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args := []any{userID, limit + 1}
	q := `
		SELECT id, public_id, created_by, source_url, resolved_url, kind, status,
			attempts, max_attempts, title, thumbnail_url, mime_hint, selected_format,
			parent_folder_id, video_public_id, object_id, progress_pct, bytes_done, bytes_total,
			last_error, started_at, finished_at, created_at, updated_at
		FROM download_jobs
		WHERE created_by = $1
	`
	if cursor > 0 {
		args = append(args, cursor)
		q += fmt.Sprintf(` AND id < $%d`, len(args))
	}
	q += ` ORDER BY id DESC LIMIT $2`

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var items []DownloadJob
	for rows.Next() {
		var row DownloadJob
		if err := rows.Scan(
			&row.ID, &row.PublicID, &row.CreatedBy, &row.SourceURL, &row.ResolvedURL, &row.Kind, &row.Status,
			&row.Attempts, &row.MaxAttempts, &row.Title, &row.ThumbnailURL, &row.MimeHint, &row.SelectedFormat,
			&row.ParentFolderID, &row.VideoPublicID, &row.ObjectID, &row.ProgressPct, &row.BytesDone, &row.BytesTotal,
			&row.LastError, &row.StartedAt, &row.FinishedAt, &row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			return nil, nil, err
		}
		items = append(items, row)
	}
	var next *int64
	if len(items) > limit {
		items = items[:limit]
		n := items[len(items)-1].ID
		next = &n
	}
	return items, next, rows.Err()
}

func (r *DownloadJobRepository) MarkRunning(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE download_jobs
		SET status = 'running', attempts = attempts + 1, started_at = COALESCE(started_at, now()),
			updated_at = now(), last_error = NULL
		WHERE id = $1 AND status IN ('pending', 'failed', 'running')
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrDownloadJobNotRunnable
	}
	return nil
}

func (r *DownloadJobRepository) UpdateProgress(ctx context.Context, id int64, pct int, done, total int64) error {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE download_jobs
		SET progress_pct = $2, bytes_done = $3, bytes_total = $4, updated_at = now()
		WHERE id = $1 AND status = 'running'
	`, id, pct, done, total)
	return err
}

// LinkMedia records the created media object before the job is marked succeeded
// so retries can resume convert without re-downloading.
func (r *DownloadJobRepository) LinkMedia(ctx context.Context, id int64, videoPID uuid.UUID, objectID int64) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE download_jobs
		SET video_public_id = $2, object_id = $3, updated_at = now()
		WHERE id = $1 AND status = 'running'
	`, id, videoPID, objectID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrDownloadJobNotRunnable
	}
	return nil
}

func (r *DownloadJobRepository) MarkSucceeded(ctx context.Context, id int64, videoPID uuid.UUID, objectID int64) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE download_jobs
		SET status = 'succeeded', progress_pct = 100, video_public_id = $2, object_id = $3,
			finished_at = now(), updated_at = now(), last_error = NULL
		WHERE id = $1 AND status = 'running'
	`, id, videoPID, objectID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrDownloadJobNotRunnable
	}
	return nil
}

func (r *DownloadJobRepository) MarkFailed(ctx context.Context, id int64, msg string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE download_jobs
		SET status = 'failed', last_error = $2, finished_at = now(), updated_at = now()
		WHERE id = $1 AND status IN ('pending', 'running')
	`, id, msg)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrDownloadJobNotRunnable
	}
	return nil
}

func (r *DownloadJobRepository) MarkCancelled(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE download_jobs
		SET status = 'cancelled', finished_at = now(), updated_at = now()
		WHERE id = $1 AND status IN ('pending', 'running')
	`, id)
	return err
}

// FailStaleRunning marks download jobs stuck in running past olderThan as failed.
func (r *DownloadJobRepository) FailStaleRunning(ctx context.Context, olderThan time.Duration) (int64, error) {
	if olderThan <= 0 {
		olderThan = 2*time.Hour + 5*time.Minute
	}
	cutoff := time.Now().Add(-olderThan)
	tag, err := r.pool.Exec(ctx, `
		UPDATE download_jobs
		SET status = 'failed', last_error = 'stale: worker timeout', finished_at = now(), updated_at = now()
		WHERE status = 'running'
		  AND COALESCE(started_at, created_at) < $1
	`, cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *DownloadJobRepository) ResetForRetry(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE download_jobs
		SET status = 'pending', progress_pct = 0, bytes_done = 0, bytes_total = 0,
			last_error = NULL, started_at = NULL, finished_at = NULL, updated_at = now()
		WHERE id = $1 AND status IN ('failed', 'cancelled')
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrDownloadJobNotFound
	}
	return nil
}

func (r *DownloadJobRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM download_jobs
		WHERE id = $1 AND status IN ('failed', 'cancelled', 'succeeded')
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrDownloadJobNotFound
	}
	return nil
}
