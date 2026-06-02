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

type UploadPartRecord struct {
	PartNumber int32  `json:"part_number"`
	ETag       string `json:"etag"`
}

type UploadSession struct {
	ID                int64
	PublicID          uuid.UUID
	UserID            int64
	ParentObjectID    int64
	OriginalName      string
	MimeType          *string
	TotalSize         int64
	ChunkSize         int
	Status            string
	StoragePrefix     string
	S3UploadID        *string
	FinalStorageKey   *string
	Parts             []UploadPartRecord
	CommittedObjectID *int64
	ExpiresAt         time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type UploadSessionRepository struct {
	pool *pgxpool.Pool
}

func NewUploadSessionRepository(pool *pgxpool.Pool) *UploadSessionRepository {
	return &UploadSessionRepository{pool: pool}
}

var ErrUploadSessionNotFound = errors.New("upload session not found")

type CreateUploadSessionInput struct {
	PublicID        uuid.UUID
	UserID          int64
	ParentObjectID  int64
	OriginalName    string
	MimeType        *string
	TotalSize       int64
	ChunkSize       int
	StoragePrefix   string
	S3UploadID      string
	FinalStorageKey string
	ExpiresAt       time.Time
}

const uploadSessionSelectCols = `
	id, public_id, user_id, parent_object_id, original_name, mime_type,
	total_size, chunk_size, status, storage_prefix, s3_upload_id, final_storage_key,
	parts_json, committed_object_id, expires_at, created_at, updated_at
`

func scanUploadSession(row pgx.Row) (*UploadSession, error) {
	var s UploadSession
	var partsJSON []byte
	var s3UploadID, finalKey *string
	err := row.Scan(
		&s.ID, &s.PublicID, &s.UserID, &s.ParentObjectID, &s.OriginalName, &s.MimeType,
		&s.TotalSize, &s.ChunkSize, &s.Status, &s.StoragePrefix, &s3UploadID, &finalKey,
		&partsJSON, &s.CommittedObjectID, &s.ExpiresAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUploadSessionNotFound
		}
		return nil, err
	}
	s.S3UploadID = s3UploadID
	s.FinalStorageKey = finalKey
	if len(partsJSON) > 0 {
		if err := json.Unmarshal(partsJSON, &s.Parts); err != nil {
			return nil, fmt.Errorf("parse parts_json: %w", err)
		}
	}
	return &s, nil
}

func (r *UploadSessionRepository) Create(ctx context.Context, in CreateUploadSessionInput) (*UploadSession, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO upload_sessions (
			public_id, user_id, parent_object_id, original_name, mime_type,
			total_size, chunk_size, storage_prefix, s3_upload_id, final_storage_key, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING `+uploadSessionSelectCols,
		in.PublicID, in.UserID, in.ParentObjectID, in.OriginalName, in.MimeType,
		in.TotalSize, in.ChunkSize, in.StoragePrefix, in.S3UploadID, in.FinalStorageKey, in.ExpiresAt,
	)
	s, err := scanUploadSession(row)
	if err != nil {
		return nil, fmt.Errorf("create upload session: %w", err)
	}
	return s, nil
}

func (r *UploadSessionRepository) GetByPublicID(ctx context.Context, publicID uuid.UUID) (*UploadSession, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+uploadSessionSelectCols+`
		FROM upload_sessions WHERE public_id = $1
	`, publicID)
	return scanUploadSession(row)
}

func (r *UploadSessionRepository) GetByPublicIDForUpdate(ctx context.Context, publicID uuid.UUID) (*UploadSession, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+uploadSessionSelectCols+`
		FROM upload_sessions WHERE public_id = $1 FOR UPDATE
	`, publicID)
	return scanUploadSession(row)
}

func (r *UploadSessionRepository) GetByPublicIDForUpdateTx(ctx context.Context, tx pgx.Tx, publicID uuid.UUID) (*UploadSession, error) {
	row := tx.QueryRow(ctx, `
		SELECT `+uploadSessionSelectCols+`
		FROM upload_sessions WHERE public_id = $1 FOR UPDATE
	`, publicID)
	return scanUploadSession(row)
}

func (r *UploadSessionRepository) AppendPart(ctx context.Context, sessionID int64, part UploadPartRecord) error {
	payload, err := json.Marshal([]UploadPartRecord{part})
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE upload_sessions
		SET parts_json = parts_json || $2::jsonb, updated_at = now()
		WHERE id = $1 AND status = 'pending'
	`, sessionID, payload)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUploadSessionNotFound
	}
	return nil
}

func (r *UploadSessionRepository) MarkCompleted(ctx context.Context, tx pgx.Tx, sessionID, objectID int64) error {
	_, err := tx.Exec(ctx, `
		UPDATE upload_sessions
		SET status = 'completed', committed_object_id = $1, updated_at = now()
		WHERE id = $2
	`, objectID, sessionID)
	return err
}

func (r *UploadSessionRepository) CountPendingByUser(ctx context.Context, userID int64) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM upload_sessions
		WHERE user_id = $1 AND status = 'pending' AND expires_at > now()
	`, userID).Scan(&n)
	return n, err
}

func (r *UploadSessionRepository) MarkAborted(ctx context.Context, sessionID int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE upload_sessions SET status = 'aborted', updated_at = now()
		WHERE id = $1 AND status = 'pending'
	`, sessionID)
	return err
}

func (r *UploadSessionRepository) MarkExpired(ctx context.Context, sessionID int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE upload_sessions SET status = 'expired', updated_at = now()
		WHERE id = $1 AND status = 'pending'
	`, sessionID)
	return err
}

// ListExpiredPending returns pending sessions past expires_at (oldest first).
func (r *UploadSessionRepository) ListExpiredPending(ctx context.Context, limit int) ([]UploadSession, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+uploadSessionSelectCols+`
		FROM upload_sessions
		WHERE status = 'pending' AND expires_at < now()
		ORDER BY expires_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []UploadSession
	for rows.Next() {
		s, err := scanUploadSession(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *s)
	}
	return list, rows.Err()
}

// ListPendingStorageGuards returns keys/prefixes for in-progress uploads that must not be purged.
func (r *UploadSessionRepository) ListPendingStorageGuards(ctx context.Context) (prefixes []string, keys []string, err error) {
	rows, err := r.pool.Query(ctx, `
		SELECT storage_prefix, final_storage_key
		FROM upload_sessions
		WHERE status = 'pending'
	`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var prefix string
		var finalKey *string
		if err := rows.Scan(&prefix, &finalKey); err != nil {
			return nil, nil, err
		}
		if prefix != "" {
			prefixes = append(prefixes, prefix)
		}
		if finalKey != nil && *finalKey != "" {
			keys = append(keys, *finalKey)
		}
	}
	return prefixes, keys, rows.Err()
}
