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

var (
	ErrVideoAssetNotFound = errors.New("video asset not found")
	ErrConvertJobNotFound = errors.New("convert job not found")
	ErrActiveConvertJob   = errors.New("active convert job exists")
)

type VideoAsset struct {
	ID               int64
	ObjectID         int64
	DurationSeconds  *int
	Width            *int
	Height           *int
	Codec            *string
	Bitrate          *int64
	HLSStatus        string
	HLSMasterKey     *string
	HLSStoragePrefix *string
	ThumbnailKey     *string
	LastError        *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// VideoRow joins media_objects with video_assets for API listing.
type VideoRow struct {
	Media MediaObject
	Asset VideoAsset
}

type VideoListFilter struct {
	HLSStatus []string
	Query     string
	Cursor    int64
	Limit     int
}

type ConvertJob struct {
	ID           int64
	PublicID     uuid.UUID
	VideoAssetID int64
	Status       string
	Attempts     int
	MaxAttempts  int
	Error        *string
	StartedAt    *time.Time
	FinishedAt   *time.Time
	CreatedBy    *int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type StreamPolicy struct {
	ID              int64
	VideoAssetID    int64
	AccessMode      string
	AllowedDomains  []string
	TokenTTLSeconds int
	AllowDownload   bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type VideoRepository struct {
	pool *pgxpool.Pool
}

func NewVideoRepository(pool *pgxpool.Pool) *VideoRepository {
	return &VideoRepository{pool: pool}
}

func (r *VideoRepository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

func scanVideoAsset(row pgx.Row) (*VideoAsset, error) {
	var v VideoAsset
	err := row.Scan(
		&v.ID, &v.ObjectID, &v.DurationSeconds, &v.Width, &v.Height, &v.Codec, &v.Bitrate,
		&v.HLSStatus, &v.HLSMasterKey, &v.HLSStoragePrefix, &v.ThumbnailKey, &v.LastError,
		&v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVideoAssetNotFound
		}
		return nil, err
	}
	return &v, nil
}

func (r *VideoRepository) GetByObjectID(ctx context.Context, objectID int64) (*VideoAsset, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, object_id, duration_seconds, width, height, codec, bitrate,
		       hls_status, hls_master_key, hls_storage_prefix, thumbnail_key, last_error,
		       created_at, updated_at
		FROM video_assets WHERE object_id = $1
	`, objectID)
	return scanVideoAsset(row)
}

func (r *VideoRepository) GetByObjectPublicID(ctx context.Context, publicID uuid.UUID) (*VideoRow, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT m.id, m.public_id, m.parent_id, p.public_id, m.type, m.name, m.original_name,
		       m.mime_type, m.size_bytes, m.storage_key, m.checksum, m.thumbnail_key, m.status,
		       m.created_by, m.updated_by, m.deleted_at, m.created_at, m.updated_at,
		       v.id, v.object_id, v.duration_seconds, v.width, v.height, v.codec, v.bitrate,
		       v.hls_status, v.hls_master_key, v.hls_storage_prefix, v.thumbnail_key, v.last_error,
		       v.created_at, v.updated_at
		FROM media_objects m
		LEFT JOIN media_objects p ON p.id = m.parent_id
		JOIN video_assets v ON v.object_id = m.id
		WHERE m.public_id = $1 AND m.deleted_at IS NULL AND m.type = 'video'
	`, publicID)
	var m MediaObject
	var v VideoAsset
	err := row.Scan(
		&m.ID, &m.PublicID, &m.ParentID, &m.ParentPublic, &m.Type, &m.Name, &m.OriginalName,
		&m.MimeType, &m.SizeBytes, &m.StorageKey, &m.Checksum, &m.ThumbnailKey, &m.Status,
		&m.CreatedBy, &m.UpdatedBy, &m.DeletedAt, &m.CreatedAt, &m.UpdatedAt,
		&v.ID, &v.ObjectID, &v.DurationSeconds, &v.Width, &v.Height, &v.Codec, &v.Bitrate,
		&v.HLSStatus, &v.HLSMasterKey, &v.HLSStoragePrefix, &v.ThumbnailKey, &v.LastError,
		&v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVideoAssetNotFound
		}
		return nil, err
	}
	v.ObjectID = m.ID
	return &VideoRow{Media: m, Asset: v}, nil
}

func (r *VideoRepository) ListVideos(ctx context.Context, f VideoListFilter) ([]VideoRow, error) {
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args := []any{}
	where := `m.deleted_at IS NULL AND m.type = 'video'`
	argN := 1
	if f.Cursor > 0 {
		where += fmt.Sprintf(` AND m.id < $%d`, argN)
		args = append(args, f.Cursor)
		argN++
	}
	if len(f.HLSStatus) > 0 {
		where += fmt.Sprintf(` AND v.hls_status = ANY($%d)`, argN)
		args = append(args, f.HLSStatus)
		argN++
	}
	if q := trimQuery(f.Query); q != "" {
		where += nameILikeClause(argN)
		args = append(args, q)
		argN++
	}
	args = append(args, limit+1)
	q := fmt.Sprintf(`
		SELECT m.id, m.public_id, m.parent_id, p.public_id, m.type, m.name, m.original_name,
		       m.mime_type, m.size_bytes, m.storage_key, m.checksum, m.thumbnail_key, m.status,
		       m.created_by, m.updated_by, m.deleted_at, m.created_at, m.updated_at,
		       v.id, v.object_id, v.duration_seconds, v.width, v.height, v.codec, v.bitrate,
		       v.hls_status, v.hls_master_key, v.hls_storage_prefix, v.thumbnail_key, v.last_error,
		       v.created_at, v.updated_at
		FROM media_objects m
		LEFT JOIN media_objects p ON p.id = m.parent_id
		JOIN video_assets v ON v.object_id = m.id
		WHERE %s
		ORDER BY m.id DESC
		LIMIT $%d
	`, where, argN)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []VideoRow
	for rows.Next() {
		var row VideoRow
		var m MediaObject
		var v VideoAsset
		if err := rows.Scan(
			&m.ID, &m.PublicID, &m.ParentID, &m.ParentPublic, &m.Type, &m.Name, &m.OriginalName,
			&m.MimeType, &m.SizeBytes, &m.StorageKey, &m.Checksum, &m.ThumbnailKey, &m.Status,
			&m.CreatedBy, &m.UpdatedBy, &m.DeletedAt, &m.CreatedAt, &m.UpdatedAt,
			&v.ID, &v.ObjectID, &v.DurationSeconds, &v.Width, &v.Height, &v.Codec, &v.Bitrate,
			&v.HLSStatus, &v.HLSMasterKey, &v.HLSStoragePrefix, &v.ThumbnailKey, &v.LastError,
			&v.CreatedAt, &v.UpdatedAt,
		); err != nil {
			return nil, err
		}
		v.ObjectID = m.ID
		row.Media = m
		row.Asset = v
		list = append(list, row)
	}
	return list, rows.Err()
}

func trimQuery(q string) string {
	for len(q) > 0 && (q[0] == ' ' || q[0] == '\t') {
		q = q[1:]
	}
	for len(q) > 0 && (q[len(q)-1] == ' ' || q[len(q)-1] == '\t') {
		q = q[:len(q)-1]
	}
	return q
}

func (r *VideoRepository) UpdateHLSStatus(ctx context.Context, assetID int64, status string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE video_assets SET hls_status = $2, updated_at = now() WHERE id = $1
	`, assetID, status)
	return err
}

func (r *VideoRepository) SetProbeMetadata(ctx context.Context, assetID int64, duration *int, width, height *int, codec *string, bitrate *int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE video_assets
		SET duration_seconds = $2, width = $3, height = $4, codec = $5, bitrate = $6, updated_at = now()
		WHERE id = $1
	`, assetID, duration, width, height, codec, bitrate)
	return err
}

func (r *VideoRepository) SetHLSReady(ctx context.Context, assetID int64, masterKey, prefix, thumbKey string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE video_assets
		SET hls_status = 'ready', hls_master_key = $2, hls_storage_prefix = $3,
		    thumbnail_key = NULLIF($4, ''), last_error = NULL, updated_at = now()
		WHERE id = $1
	`, assetID, masterKey, prefix, thumbKey)
	return err
}

func (r *VideoRepository) ClearHLS(ctx context.Context, assetID int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE video_assets
		SET hls_status = 'deleted', hls_master_key = NULL, hls_storage_prefix = NULL,
		    last_error = NULL, updated_at = now()
		WHERE id = $1
	`, assetID)
	return err
}

func (r *VideoRepository) SetLastError(ctx context.Context, assetID int64, msg string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE video_assets SET hls_status = 'failed', last_error = $2, updated_at = now() WHERE id = $1
	`, assetID, msg)
	return err
}

func (r *VideoRepository) SetThumbnailKey(ctx context.Context, assetID int64, key string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE video_assets SET thumbnail_key = $2, updated_at = now() WHERE id = $1
	`, assetID, key)
	return err
}

func (r *VideoRepository) CountByHLSStatus(ctx context.Context) (map[string]int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT v.hls_status, COUNT(*)
		FROM video_assets v
		JOIN media_objects m ON m.id = v.object_id AND m.deleted_at IS NULL
		GROUP BY v.hls_status
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int64)
	for rows.Next() {
		var status string
		var n int64
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = n
	}
	return out, rows.Err()
}

func (r *VideoRepository) CreateConvertJob(ctx context.Context, publicID uuid.UUID, videoAssetID, createdBy int64) (*ConvertJob, error) {
	var j ConvertJob
	err := r.pool.QueryRow(ctx, `
		INSERT INTO convert_jobs (public_id, video_asset_id, status, created_by)
		VALUES ($1, $2, 'pending', $3)
		RETURNING id, public_id, video_asset_id, status, attempts, max_attempts, error,
		          started_at, finished_at, created_by, created_at, updated_at
	`, publicID, videoAssetID, createdBy).Scan(
		&j.ID, &j.PublicID, &j.VideoAssetID, &j.Status, &j.Attempts, &j.MaxAttempts, &j.Error,
		&j.StartedAt, &j.FinishedAt, &j.CreatedBy, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create convert job: %w", err)
	}
	return &j, nil
}

func (r *VideoRepository) GetActiveJobForAsset(ctx context.Context, videoAssetID int64) (*ConvertJob, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, public_id, video_asset_id, status, attempts, max_attempts, error,
		       started_at, finished_at, created_by, created_at, updated_at
		FROM convert_jobs
		WHERE video_asset_id = $1 AND status IN ('pending', 'running')
		ORDER BY id DESC LIMIT 1
	`, videoAssetID)
	var j ConvertJob
	err := row.Scan(
		&j.ID, &j.PublicID, &j.VideoAssetID, &j.Status, &j.Attempts, &j.MaxAttempts, &j.Error,
		&j.StartedAt, &j.FinishedAt, &j.CreatedBy, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &j, nil
}

func (r *VideoRepository) GetConvertJobByPublicID(ctx context.Context, publicID uuid.UUID) (*ConvertJob, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, public_id, video_asset_id, status, attempts, max_attempts, error,
		       started_at, finished_at, created_by, created_at, updated_at
		FROM convert_jobs WHERE public_id = $1
	`, publicID)
	var j ConvertJob
	err := row.Scan(
		&j.ID, &j.PublicID, &j.VideoAssetID, &j.Status, &j.Attempts, &j.MaxAttempts, &j.Error,
		&j.StartedAt, &j.FinishedAt, &j.CreatedBy, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConvertJobNotFound
		}
		return nil, err
	}
	return &j, nil
}

func (r *VideoRepository) LatestJobForAsset(ctx context.Context, videoAssetID int64) (*ConvertJob, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, public_id, video_asset_id, status, attempts, max_attempts, error,
		       started_at, finished_at, created_by, created_at, updated_at
		FROM convert_jobs WHERE video_asset_id = $1 ORDER BY id DESC LIMIT 1
	`, videoAssetID)
	var j ConvertJob
	err := row.Scan(
		&j.ID, &j.PublicID, &j.VideoAssetID, &j.Status, &j.Attempts, &j.MaxAttempts, &j.Error,
		&j.StartedAt, &j.FinishedAt, &j.CreatedBy, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &j, nil
}

func (r *VideoRepository) UpdateJobStatus(ctx context.Context, jobID int64, status string, errMsg *string) error {
	now := time.Now()
	switch status {
	case "running":
		_, err := r.pool.Exec(ctx, `
			UPDATE convert_jobs SET status = $2, started_at = $3, attempts = attempts + 1, updated_at = now()
			WHERE id = $1
		`, jobID, status, now)
		return err
	case "succeeded", "failed", "cancelled":
		_, err := r.pool.Exec(ctx, `
			UPDATE convert_jobs SET status = $2, error = $3, finished_at = $4, updated_at = now()
			WHERE id = $1
		`, jobID, status, errMsg, now)
		return err
	default:
		_, err := r.pool.Exec(ctx, `UPDATE convert_jobs SET status = $2, updated_at = now() WHERE id = $1`, jobID, status)
		return err
	}
}

func (r *VideoRepository) CountJobsByStatus(ctx context.Context) (map[string]int64, error) {
	rows, err := r.pool.Query(ctx, `SELECT status, COUNT(*) FROM convert_jobs GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int64)
	for rows.Next() {
		var s string
		var n int64
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		out[s] = n
	}
	return out, rows.Err()
}

type FailedConvertJobRow struct {
	JobPublicID   uuid.UUID
	VideoPublicID uuid.UUID
	VideoName     string
	Attempts      int
	MaxAttempts   int
	Error         *string
	FinishedAt    *time.Time
}

func (r *VideoRepository) ListRecentFailedJobs(ctx context.Context, limit int) ([]FailedConvertJobRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := r.pool.Query(ctx, `
		SELECT j.public_id, m.public_id, m.name, j.attempts, j.max_attempts, j.error, j.finished_at
		FROM convert_jobs j
		JOIN video_assets v ON v.id = j.video_asset_id
		JOIN media_objects m ON m.id = v.object_id AND m.deleted_at IS NULL
		WHERE j.status = 'failed'
		ORDER BY j.finished_at DESC NULLS LAST, j.id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []FailedConvertJobRow
	for rows.Next() {
		var row FailedConvertJobRow
		if err := rows.Scan(
			&row.JobPublicID, &row.VideoPublicID, &row.VideoName,
			&row.Attempts, &row.MaxAttempts, &row.Error, &row.FinishedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, row)
	}
	return list, rows.Err()
}

func (r *VideoRepository) DeleteFinishedJobsOlderThan(ctx context.Context, before time.Time, batchSize int) (int64, error) {
	if batchSize <= 0 {
		batchSize = 1000
	}
	var total int64
	for {
		tag, err := r.pool.Exec(ctx, `
			DELETE FROM convert_jobs
			WHERE id IN (
				SELECT id FROM convert_jobs
				WHERE status IN ('succeeded', 'failed', 'cancelled')
				  AND COALESCE(finished_at, updated_at) < $1
				ORDER BY id
				LIMIT $2
			)
		`, before, batchSize)
		if err != nil {
			return total, err
		}
		n := tag.RowsAffected()
		total += n
		if n < int64(batchSize) {
			break
		}
	}
	return total, nil
}

func scanStreamPolicy(row pgx.Row) (*StreamPolicy, error) {
	var p StreamPolicy
	var domainsJSON []byte
	err := row.Scan(
		&p.ID, &p.VideoAssetID, &p.AccessMode, &domainsJSON, &p.TokenTTLSeconds, &p.AllowDownload,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	_ = json.Unmarshal(domainsJSON, &p.AllowedDomains)
	if p.AllowedDomains == nil {
		p.AllowedDomains = []string{}
	}
	return &p, nil
}

func (r *VideoRepository) GetStreamPolicy(ctx context.Context, videoAssetID int64) (*StreamPolicy, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, video_asset_id, access_mode, allowed_domains, token_ttl_seconds, allow_download,
		       created_at, updated_at
		FROM stream_policies WHERE video_asset_id = $1
	`, videoAssetID)
	return scanStreamPolicy(row)
}

func (r *VideoRepository) GetOrCreateStreamPolicy(ctx context.Context, videoAssetID int64, defaultTTL int) (*StreamPolicy, error) {
	p, err := r.GetStreamPolicy(ctx, videoAssetID)
	if err != nil {
		return nil, err
	}
	if p != nil {
		return p, nil
	}
	domains, _ := json.Marshal([]string{})
	row := r.pool.QueryRow(ctx, `
		INSERT INTO stream_policies (video_asset_id, allowed_domains, token_ttl_seconds)
		VALUES ($1, $2, $3)
		RETURNING id, video_asset_id, access_mode, allowed_domains, token_ttl_seconds, allow_download,
		          created_at, updated_at
	`, videoAssetID, domains, defaultTTL)
	return scanStreamPolicy(row)
}

type StreamPolicyUpdate struct {
	AccessMode      *string
	AllowedDomains  *[]string
	TokenTTLSeconds *int
	AllowDownload   *bool
}

func (r *VideoRepository) UpdateStreamPolicy(ctx context.Context, videoAssetID int64, u StreamPolicyUpdate) (*StreamPolicy, error) {
	cur, err := r.GetOrCreateStreamPolicy(ctx, videoAssetID, 3600)
	if err != nil || cur == nil {
		return nil, err
	}
	accessMode := cur.AccessMode
	if u.AccessMode != nil {
		accessMode = *u.AccessMode
	}
	domains := cur.AllowedDomains
	if u.AllowedDomains != nil {
		domains = *u.AllowedDomains
		if domains == nil {
			domains = []string{}
		}
	}
	ttl := cur.TokenTTLSeconds
	if u.TokenTTLSeconds != nil {
		ttl = *u.TokenTTLSeconds
	}
	allowDL := cur.AllowDownload
	if u.AllowDownload != nil {
		allowDL = *u.AllowDownload
	}
	domainsJSON, _ := json.Marshal(domains)
	row := r.pool.QueryRow(ctx, `
		UPDATE stream_policies
		SET access_mode = $2, allowed_domains = $3, token_ttl_seconds = $4, allow_download = $5, updated_at = now()
		WHERE video_asset_id = $1
		RETURNING id, video_asset_id, access_mode, allowed_domains, token_ttl_seconds, allow_download,
		          created_at, updated_at
	`, videoAssetID, accessMode, domainsJSON, ttl, allowDL)
	return scanStreamPolicy(row)
}

func (r *VideoRepository) GetVideoRowByAssetID(ctx context.Context, assetID int64) (*VideoRow, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT m.id, m.public_id, m.parent_id, p.public_id, m.type, m.name, m.original_name,
		       m.mime_type, m.size_bytes, m.storage_key, m.checksum, m.thumbnail_key, m.status,
		       m.created_by, m.updated_by, m.deleted_at, m.created_at, m.updated_at,
		       v.id, v.object_id, v.duration_seconds, v.width, v.height, v.codec, v.bitrate,
		       v.hls_status, v.hls_master_key, v.hls_storage_prefix, v.thumbnail_key, v.last_error,
		       v.created_at, v.updated_at
		FROM media_objects m
		LEFT JOIN media_objects p ON p.id = m.parent_id
		JOIN video_assets v ON v.object_id = m.id
		WHERE v.id = $1 AND m.deleted_at IS NULL
	`, assetID)
	var m MediaObject
	var v VideoAsset
	err := row.Scan(
		&m.ID, &m.PublicID, &m.ParentID, &m.ParentPublic, &m.Type, &m.Name, &m.OriginalName,
		&m.MimeType, &m.SizeBytes, &m.StorageKey, &m.Checksum, &m.ThumbnailKey, &m.Status,
		&m.CreatedBy, &m.UpdatedBy, &m.DeletedAt, &m.CreatedAt, &m.UpdatedAt,
		&v.ID, &v.ObjectID, &v.DurationSeconds, &v.Width, &v.Height, &v.Codec, &v.Bitrate,
		&v.HLSStatus, &v.HLSMasterKey, &v.HLSStoragePrefix, &v.ThumbnailKey, &v.LastError,
		&v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVideoAssetNotFound
		}
		return nil, err
	}
	v.ObjectID = m.ID
	return &VideoRow{Media: m, Asset: v}, nil
}
