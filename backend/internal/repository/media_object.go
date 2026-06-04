package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MediaObject struct {
	ID           int64
	PublicID     uuid.UUID
	ParentID     *int64
	ParentPublic *uuid.UUID
	Type         string
	Name         string
	OriginalName *string
	MimeType     *string
	SizeBytes    int64
	StorageKey    *string
	Checksum      *string
	ThumbnailKey  *string
	Status        string
	CreatedBy    *int64
	UpdatedBy    *int64
	DeletedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ObjectListFilter struct {
	Types  []string
	Query  string
	Cursor int64
	Limit  int
}

type BreadcrumbItem struct {
	PublicID uuid.UUID
	Name     string
	Type     string
	Depth    int
}

type MediaObjectRepository struct {
	pool *pgxpool.Pool
}

func NewMediaObjectRepository(pool *pgxpool.Pool) *MediaObjectRepository {
	return &MediaObjectRepository{pool: pool}
}

func (r *MediaObjectRepository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

var ErrMediaObjectNotFound = errors.New("media object not found")

var ErrInvalidMove = errors.New("invalid move")

// nameILikeClause matches names accent-insensitively (anh → Ảnh). Requires f_unaccent (migration 000012).
func nameILikeClause(argN int) string {
	return fmt.Sprintf(
		" AND f_unaccent(m.name) ILIKE '%%' || f_unaccent($%d) || '%%'",
		argN,
	)
}

func (r *MediaObjectRepository) GetByPublicIDAny(ctx context.Context, publicID uuid.UUID) (*MediaObject, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT m.id, m.public_id, m.parent_id, p.public_id, m.type, m.name, m.original_name,
		       m.mime_type, m.size_bytes, m.storage_key, m.checksum, m.thumbnail_key, m.status,
		       m.created_by, m.updated_by, m.deleted_at, m.created_at, m.updated_at
		FROM media_objects m
		LEFT JOIN media_objects p ON p.id = m.parent_id
		WHERE m.public_id = $1
	`, publicID)
	return scanMediaObject(row)
}

func (r *MediaObjectRepository) GetByPublicID(ctx context.Context, publicID uuid.UUID) (*MediaObject, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT m.id, m.public_id, m.parent_id, p.public_id, m.type, m.name, m.original_name,
		       m.mime_type, m.size_bytes, m.storage_key, m.checksum, m.thumbnail_key, m.status,
		       m.created_by, m.updated_by, m.deleted_at, m.created_at, m.updated_at
		FROM media_objects m
		LEFT JOIN media_objects p ON p.id = m.parent_id
		WHERE m.public_id = $1 AND m.deleted_at IS NULL
	`, publicID)
	return scanMediaObject(row)
}

func (r *MediaObjectRepository) ListByPublicIDs(ctx context.Context, publicIDs []uuid.UUID) ([]MediaObject, error) {
	if len(publicIDs) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.public_id, m.parent_id, p.public_id, m.type, m.name, m.original_name,
		       m.mime_type, m.size_bytes, m.storage_key, m.checksum, m.thumbnail_key, m.status,
		       m.created_by, m.updated_by, m.deleted_at, m.created_at, m.updated_at
		FROM media_objects m
		LEFT JOIN media_objects p ON p.id = m.parent_id
		WHERE m.public_id = ANY($1) AND m.deleted_at IS NULL
	`, publicIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []MediaObject
	for rows.Next() {
		m, err := scanMediaObject(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *m)
	}
	return list, rows.Err()
}

func (r *MediaObjectRepository) SetThumbnailKey(ctx context.Context, objectID int64, key string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE media_objects SET thumbnail_key = $1, updated_at = now()
		WHERE id = $2 AND deleted_at IS NULL
	`, key, objectID)
	return err
}

func (r *MediaObjectRepository) GetByID(ctx context.Context, id int64) (*MediaObject, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT m.id, m.public_id, m.parent_id, p.public_id, m.type, m.name, m.original_name,
		       m.mime_type, m.size_bytes, m.storage_key, m.checksum, m.thumbnail_key, m.status,
		       m.created_by, m.updated_by, m.deleted_at, m.created_at, m.updated_at
		FROM media_objects m
		LEFT JOIN media_objects p ON p.id = m.parent_id
		WHERE m.id = $1 AND m.deleted_at IS NULL
	`, id)
	return scanMediaObject(row)
}

func (r *MediaObjectRepository) GetByIDAny(ctx context.Context, id int64) (*MediaObject, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT m.id, m.public_id, m.parent_id, p.public_id, m.type, m.name, m.original_name,
		       m.mime_type, m.size_bytes, m.storage_key, m.checksum, m.thumbnail_key, m.status,
		       m.created_by, m.updated_by, m.deleted_at, m.created_at, m.updated_at
		FROM media_objects m
		LEFT JOIN media_objects p ON p.id = m.parent_id
		WHERE m.id = $1
	`, id)
	return scanMediaObject(row)
}

func scanMediaObject(row pgx.Row) (*MediaObject, error) {
	var m MediaObject
	err := row.Scan(
		&m.ID, &m.PublicID, &m.ParentID, &m.ParentPublic, &m.Type, &m.Name, &m.OriginalName,
		&m.MimeType, &m.SizeBytes, &m.StorageKey, &m.Checksum, &m.ThumbnailKey, &m.Status,
		&m.CreatedBy, &m.UpdatedBy, &m.DeletedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMediaObjectNotFound
		}
		return nil, fmt.Errorf("scan media object: %w", err)
	}
	return &m, nil
}

func (r *MediaObjectRepository) scanMediaObjectRows(ctx context.Context, query string, args ...any) ([]MediaObject, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query media objects: %w", err)
	}
	defer rows.Close()
	var list []MediaObject
	for rows.Next() {
		var m MediaObject
		if err := rows.Scan(
			&m.ID, &m.PublicID, &m.ParentID, &m.ParentPublic, &m.Type, &m.Name, &m.OriginalName,
			&m.MimeType, &m.SizeBytes, &m.StorageKey, &m.Checksum, &m.ThumbnailKey, &m.Status,
			&m.CreatedBy, &m.UpdatedBy, &m.DeletedAt, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *MediaObjectRepository) ListChildren(ctx context.Context, parentID int64, f ObjectListFilter) ([]MediaObject, error) {
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	args := []any{parentID}
	query := `
		SELECT m.id, m.public_id, m.parent_id, p.public_id, m.type, m.name, m.original_name,
		       m.mime_type, m.size_bytes, m.storage_key, m.checksum, m.thumbnail_key, m.status,
		       m.created_by, m.updated_by, m.deleted_at, m.created_at, m.updated_at
		FROM media_objects m
		LEFT JOIN media_objects p ON p.id = m.parent_id
		WHERE m.parent_id = $1 AND m.deleted_at IS NULL AND m.status = 'active'
	`
	argN := 2

	if f.Cursor > 0 {
		query += fmt.Sprintf(" AND m.id > $%d", argN)
		args = append(args, f.Cursor)
		argN++
	}
	if len(f.Types) > 0 {
		query += fmt.Sprintf(" AND m.type = ANY($%d)", argN)
		args = append(args, f.Types)
		argN++
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		// Accent-insensitive ILIKE (pg_trgm index on f_unaccent(name), migration 000012).
		query += nameILikeClause(argN)
		args = append(args, q)
		argN++
	}
	query += " ORDER BY (m.type = 'folder') DESC, m.name ASC, m.id ASC"
	query += fmt.Sprintf(" LIMIT $%d", argN)
	args = append(args, limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list children: %w", err)
	}
	defer rows.Close()

	var list []MediaObject
	for rows.Next() {
		m, err := scanMediaObject(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *m)
	}
	return list, rows.Err()
}

func (r *MediaObjectRepository) GetAncestors(ctx context.Context, objectID int64) ([]BreadcrumbItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.public_id, m.name, m.type, MIN(op.depth) AS depth
		FROM object_paths op
		JOIN media_objects m ON m.id = op.ancestor_id
		WHERE op.descendant_id = $1 AND m.deleted_at IS NULL
		GROUP BY m.id, m.public_id, m.name, m.type
		ORDER BY depth DESC
	`, objectID)
	if err != nil {
		return nil, fmt.Errorf("get ancestors: %w", err)
	}
	defer rows.Close()

	var items []BreadcrumbItem
	for rows.Next() {
		var it BreadcrumbItem
		if err := rows.Scan(&it.PublicID, &it.Name, &it.Type, &it.Depth); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// GetAncestorsBatch returns ancestor breadcrumbs for many objects in one query.
func (r *MediaObjectRepository) GetAncestorsBatch(ctx context.Context, objectIDs []int64) (map[int64][]BreadcrumbItem, error) {
	out := make(map[int64][]BreadcrumbItem)
	if len(objectIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT op.descendant_id, m.public_id, m.name, m.type, MIN(op.depth) AS depth
		FROM object_paths op
		JOIN media_objects m ON m.id = op.ancestor_id
		WHERE op.descendant_id = ANY($1) AND m.deleted_at IS NULL
		GROUP BY op.descendant_id, m.id, m.public_id, m.name, m.type
		ORDER BY op.descendant_id, depth DESC
	`, objectIDs)
	if err != nil {
		return nil, fmt.Errorf("get ancestors batch: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var descendantID int64
		var it BreadcrumbItem
		if err := rows.Scan(&descendantID, &it.PublicID, &it.Name, &it.Type, &it.Depth); err != nil {
			return nil, err
		}
		out[descendantID] = append(out[descendantID], it)
	}
	return out, rows.Err()
}

// ListExpiredDeletedRoots lists soft-deleted roots of trash trees older than cutoff.
func (r *MediaObjectRepository) ListExpiredDeletedRoots(ctx context.Context, cutoff time.Time, limit int) ([]MediaObject, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.public_id, m.parent_id, p.public_id, m.type, m.name, m.original_name,
		       m.mime_type, m.size_bytes, m.storage_key, m.checksum, m.thumbnail_key, m.status,
		       m.created_by, m.updated_by, m.deleted_at, m.created_at, m.updated_at
		FROM media_objects m
		LEFT JOIN media_objects p ON p.id = m.parent_id
		WHERE m.deleted_at IS NOT NULL
		  AND m.deleted_at < $1
		  AND NOT EXISTS (
		    SELECT 1 FROM object_paths op
		    JOIN media_objects anc ON anc.id = op.ancestor_id
		    WHERE op.descendant_id = m.id
		      AND anc.id != m.id
		      AND anc.deleted_at IS NOT NULL
		      AND anc.deleted_at < $1
		  )
		ORDER BY m.deleted_at ASC, m.id ASC
		LIMIT $2
	`, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("list expired deleted roots: %w", err)
	}
	defer rows.Close()

	var list []MediaObject
	for rows.Next() {
		var m MediaObject
		if err := rows.Scan(
			&m.ID, &m.PublicID, &m.ParentID, &m.ParentPublic, &m.Type, &m.Name, &m.OriginalName,
			&m.MimeType, &m.SizeBytes, &m.StorageKey, &m.Checksum, &m.ThumbnailKey, &m.Status,
			&m.CreatedBy, &m.UpdatedBy, &m.DeletedAt, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *MediaObjectRepository) IsDescendant(ctx context.Context, ancestorID, descendantID int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM object_paths
			WHERE ancestor_id = $1 AND descendant_id = $2 AND depth > 0
		)
	`, ancestorID, descendantID).Scan(&exists)
	return exists, err
}

type CreateMediaObjectInput struct {
	PublicID     uuid.UUID
	ParentID     *int64
	Type         string
	Name         string
	OriginalName *string
	MimeType     *string
	SizeBytes    int64
	StorageKey   *string
	Checksum     *string
	CreatedBy    int64
}

func (r *MediaObjectRepository) CreateWithClosure(ctx context.Context, in CreateMediaObjectInput) (*MediaObject, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO media_objects (
			public_id, parent_id, type, name, original_name, mime_type, size_bytes, storage_key, checksum, created_by, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
		RETURNING id
	`, in.PublicID, in.ParentID, in.Type, in.Name, in.OriginalName, in.MimeType, in.SizeBytes, in.StorageKey, in.Checksum, in.CreatedBy).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert media object: %w", err)
	}

	if err := insertClosurePaths(ctx, tx, id, in.ParentID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func insertClosurePaths(ctx context.Context, tx pgx.Tx, objectID int64, parentID *int64) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO object_paths (ancestor_id, descendant_id, depth) VALUES ($1, $1, 0)
	`, objectID); err != nil {
		return fmt.Errorf("insert self path: %w", err)
	}
	if parentID == nil {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO object_paths (ancestor_id, descendant_id, depth)
		SELECT op.ancestor_id, $1, op.depth + 1
		FROM object_paths op
		WHERE op.descendant_id = $2
	`, objectID, *parentID)
	if err != nil {
		return fmt.Errorf("insert ancestor paths: %w", err)
	}
	return nil
}

func (r *MediaObjectRepository) UpdateName(ctx context.Context, id, updatedBy int64, name string) error {
	return r.updateName(ctx, r.pool, id, updatedBy, name)
}

func (r *MediaObjectRepository) UpdateNameTx(ctx context.Context, tx pgx.Tx, id, updatedBy int64, name string) error {
	return r.updateName(ctx, tx, id, updatedBy, name)
}

func (r *MediaObjectRepository) updateName(ctx context.Context, exec interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, id, updatedBy int64, name string) error {
	tag, err := exec.Exec(ctx, `
		UPDATE media_objects SET name = $1, updated_by = $2, updated_at = now()
		WHERE id = $3 AND deleted_at IS NULL
	`, name, updatedBy, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrMediaObjectNotFound
	}
	return nil
}

func (r *MediaObjectRepository) Move(ctx context.Context, objectID int64, newParentID *int64, updatedBy int64) error {
	if newParentID != nil && *newParentID == objectID {
		return ErrInvalidMove
	}
	if newParentID != nil {
		isDesc, err := r.IsDescendant(ctx, objectID, *newParentID)
		if err != nil {
			return err
		}
		if isDesc {
			return ErrInvalidMove
		}
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	tag, err := tx.Exec(ctx, `
		UPDATE media_objects SET parent_id = $1, updated_by = $2, updated_at = now()
		WHERE id = $3 AND deleted_at IS NULL
	`, newParentID, updatedBy, objectID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrMediaObjectNotFound
	}

	// Detach subtree from old ancestors (keep internal subtree paths).
	_, err = tx.Exec(ctx, `
		DELETE FROM object_paths op
		WHERE op.descendant_id IN (
			SELECT descendant_id FROM object_paths WHERE ancestor_id = $1
		)
		AND op.ancestor_id IN (
			SELECT ancestor_id FROM object_paths
			WHERE descendant_id = $1 AND ancestor_id != $1
		)
	`, objectID)
	if err != nil {
		return fmt.Errorf("delete old closure paths: %w", err)
	}

	if newParentID != nil {
		_, err = tx.Exec(ctx, `
			INSERT INTO object_paths (ancestor_id, descendant_id, depth)
			SELECT super.ancestor_id, sub.descendant_id, super.depth + sub.depth + 1
			FROM object_paths super
			CROSS JOIN object_paths sub
			WHERE super.descendant_id = $2
			  AND sub.ancestor_id = $1
			ON CONFLICT (ancestor_id, descendant_id)
			DO UPDATE SET depth = EXCLUDED.depth
		`, objectID, *newParentID)
		if err != nil {
			return fmt.Errorf("insert move closure paths: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *MediaObjectRepository) SoftDelete(ctx context.Context, id, updatedBy int64) error {
	n, err := r.SoftDeleteSubtree(ctx, id, updatedBy)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrMediaObjectNotFound
	}
	return nil
}

const subtreePageDefault = 200

// ListActiveInSubtree returns all active objects in the subtree rooted at ancestorID (inclusive).
func (r *MediaObjectRepository) ListActiveInSubtree(ctx context.Context, ancestorID int64) ([]MediaObject, error) {
	return r.listActiveInSubtreePage(ctx, ancestorID, 0, 0)
}

// ListActiveInSubtreePage returns a page of active subtree objects after afterID (0 = start).
func (r *MediaObjectRepository) ListActiveInSubtreePage(ctx context.Context, ancestorID, afterID int64, limit int) ([]MediaObject, error) {
	return r.listActiveInSubtreePage(ctx, ancestorID, afterID, limit)
}

func (r *MediaObjectRepository) listActiveInSubtreePage(ctx context.Context, ancestorID, afterID int64, limit int) ([]MediaObject, error) {
	unbounded := limit <= 0
	pageLimit := limit
	if !unbounded && pageLimit > 500 {
		pageLimit = subtreePageDefault
	}
	args := []any{ancestorID}
	query := `
		SELECT m.id, m.public_id, m.parent_id, p.public_id, m.type, m.name, m.original_name,
		       m.mime_type, m.size_bytes, m.storage_key, m.checksum, m.thumbnail_key, m.status,
		       m.created_by, m.updated_by, m.deleted_at, m.created_at, m.updated_at
		FROM object_paths op
		JOIN media_objects m ON m.id = op.descendant_id
		LEFT JOIN media_objects p ON p.id = m.parent_id
		WHERE op.ancestor_id = $1 AND m.deleted_at IS NULL AND m.status = 'active'
	`
	if afterID > 0 {
		query += ` AND m.id > $2`
		args = append(args, afterID)
	}
	query += ` ORDER BY m.id ASC`
	if !unbounded {
		n := len(args) + 1
		query += fmt.Sprintf(` LIMIT $%d`, n)
		args = append(args, pageLimit)
	}
	return r.scanMediaObjectRows(ctx, query, args...)
}

const deletedTrashRootFilter = `
		  AND NOT EXISTS (
		    SELECT 1 FROM object_paths op
		    JOIN media_objects anc ON anc.id = op.ancestor_id
		    WHERE op.descendant_id = m.id
		      AND anc.id != m.id
		      AND anc.deleted_at IS NOT NULL
		  )`

// ListDeletedRoots lists top-level trash entries: deleted files/folders whose parent
// is not deleted. Deleting a folder appears once; nested contents are hidden until restore/purge.
func (r *MediaObjectRepository) ListDeletedRoots(ctx context.Context, f ObjectListFilter) ([]MediaObject, error) {
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args := []any{}
	query := `
		SELECT m.id, m.public_id, m.parent_id, p.public_id, m.type, m.name, m.original_name,
		       m.mime_type, m.size_bytes, m.storage_key, m.checksum, m.thumbnail_key, m.status,
		       m.created_by, m.updated_by, m.deleted_at, m.created_at, m.updated_at
		FROM media_objects m
		LEFT JOIN media_objects p ON p.id = m.parent_id
		WHERE m.deleted_at IS NOT NULL
	` + deletedTrashRootFilter
	argN := 1
	if f.Cursor > 0 {
		query += fmt.Sprintf(" AND m.id < $%d", argN)
		args = append(args, f.Cursor)
		argN++
	}
	if len(f.Types) > 0 {
		query += fmt.Sprintf(" AND m.type = ANY($%d)", argN)
		args = append(args, f.Types)
		argN++
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		query += nameILikeClause(argN)
		args = append(args, q)
		argN++
	}
	query += " ORDER BY m.deleted_at DESC, m.id DESC"
	query += fmt.Sprintf(" LIMIT $%d", argN)
	args = append(args, limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []MediaObject
	for rows.Next() {
		var m MediaObject
		if err := rows.Scan(
			&m.ID, &m.PublicID, &m.ParentID, &m.ParentPublic, &m.Type, &m.Name, &m.OriginalName,
			&m.MimeType, &m.SizeBytes, &m.StorageKey, &m.Checksum, &m.ThumbnailKey, &m.Status,
			&m.CreatedBy, &m.UpdatedBy, &m.DeletedAt, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// ListDeleted lists every soft-deleted object (newest first), including nested trash.
func (r *MediaObjectRepository) ListDeleted(ctx context.Context, f ObjectListFilter) ([]MediaObject, error) {
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args := []any{}
	query := `
		SELECT m.id, m.public_id, m.parent_id, p.public_id, m.type, m.name, m.original_name,
		       m.mime_type, m.size_bytes, m.storage_key, m.checksum, m.thumbnail_key, m.status,
		       m.created_by, m.updated_by, m.deleted_at, m.created_at, m.updated_at
		FROM media_objects m
		LEFT JOIN media_objects p ON p.id = m.parent_id
		WHERE m.deleted_at IS NOT NULL
	`
	argN := 1
	if f.Cursor > 0 {
		query += fmt.Sprintf(" AND m.id < $%d", argN)
		args = append(args, f.Cursor)
		argN++
	}
	if len(f.Types) > 0 {
		query += fmt.Sprintf(" AND m.type = ANY($%d)", argN)
		args = append(args, f.Types)
		argN++
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		query += nameILikeClause(argN)
		args = append(args, q)
		argN++
	}
	query += " ORDER BY m.deleted_at DESC, m.id DESC"
	query += fmt.Sprintf(" LIMIT $%d", argN)
	args = append(args, limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []MediaObject
	for rows.Next() {
		var m MediaObject
		if err := rows.Scan(
			&m.ID, &m.PublicID, &m.ParentID, &m.ParentPublic, &m.Type, &m.Name, &m.OriginalName,
			&m.MimeType, &m.SizeBytes, &m.StorageKey, &m.Checksum, &m.ThumbnailKey, &m.Status,
			&m.CreatedBy, &m.UpdatedBy, &m.DeletedAt, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// SearchActiveByName searches active objects by name across the workspace (no parent filter).
func (r *MediaObjectRepository) SearchActiveByName(ctx context.Context, f ObjectListFilter) ([]MediaObject, error) {
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := strings.TrimSpace(f.Query)
	if q == "" {
		return nil, nil
	}
	args := []any{q}
	query := `
		SELECT m.id, m.public_id, m.parent_id, p.public_id, m.type, m.name, m.original_name,
		       m.mime_type, m.size_bytes, m.storage_key, m.checksum, m.thumbnail_key, m.status,
		       m.created_by, m.updated_by, m.deleted_at, m.created_at, m.updated_at
		FROM media_objects m
		LEFT JOIN media_objects p ON p.id = m.parent_id
		WHERE m.deleted_at IS NULL AND m.status = 'active'
		  AND f_unaccent(m.name) ILIKE '%' || f_unaccent($1) || '%'
	`
	argN := 2
	if f.Cursor > 0 {
		query += fmt.Sprintf(" AND m.id > $%d", argN)
		args = append(args, f.Cursor)
		argN++
	}
	if len(f.Types) > 0 {
		query += fmt.Sprintf(" AND m.type = ANY($%d)", argN)
		args = append(args, f.Types)
		argN++
	}
	query += " ORDER BY (m.type = 'folder') DESC, m.name ASC, m.id ASC"
	query += fmt.Sprintf(" LIMIT $%d", argN)
	args = append(args, limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []MediaObject
	for rows.Next() {
		var m MediaObject
		if err := rows.Scan(
			&m.ID, &m.PublicID, &m.ParentID, &m.ParentPublic, &m.Type, &m.Name, &m.OriginalName,
			&m.MimeType, &m.SizeBytes, &m.StorageKey, &m.Checksum, &m.ThumbnailKey, &m.Status,
			&m.CreatedBy, &m.UpdatedBy, &m.DeletedAt, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// SearchActiveInSubtree finds active objects by name under ancestor (all depths, excluding ancestor).
func (r *MediaObjectRepository) SearchActiveInSubtree(ctx context.Context, ancestorID int64, f ObjectListFilter) ([]MediaObject, error) {
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := strings.TrimSpace(f.Query)
	if q == "" {
		return nil, nil
	}
	args := []any{ancestorID, q}
	query := `
		SELECT m.id, m.public_id, m.parent_id, p.public_id, m.type, m.name, m.original_name,
		       m.mime_type, m.size_bytes, m.storage_key, m.checksum, m.thumbnail_key, m.status,
		       m.created_by, m.updated_by, m.deleted_at, m.created_at, m.updated_at
		FROM object_paths op
		JOIN media_objects m ON m.id = op.descendant_id
		LEFT JOIN media_objects p ON p.id = m.parent_id
		WHERE op.ancestor_id = $1
		  AND op.depth > 0
		  AND m.deleted_at IS NULL AND m.status = 'active'
		  AND f_unaccent(m.name) ILIKE '%' || f_unaccent($2) || '%'
	`
	argN := 3
	if f.Cursor > 0 {
		query += fmt.Sprintf(" AND m.id > $%d", argN)
		args = append(args, f.Cursor)
		argN++
	}
	if len(f.Types) > 0 {
		query += fmt.Sprintf(" AND m.type = ANY($%d)", argN)
		args = append(args, f.Types)
		argN++
	}
	query += " ORDER BY (m.type = 'folder') DESC, m.name ASC, m.id ASC"
	query += fmt.Sprintf(" LIMIT $%d", argN)
	args = append(args, limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []MediaObject
	for rows.Next() {
		var m MediaObject
		if err := rows.Scan(
			&m.ID, &m.PublicID, &m.ParentID, &m.ParentPublic, &m.Type, &m.Name, &m.OriginalName,
			&m.MimeType, &m.SizeBytes, &m.StorageKey, &m.Checksum, &m.ThumbnailKey, &m.Status,
			&m.CreatedBy, &m.UpdatedBy, &m.DeletedAt, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *MediaObjectRepository) Restore(ctx context.Context, id, updatedBy int64) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE media_objects
		SET deleted_at = NULL, status = 'active', updated_by = $1, updated_at = now()
		WHERE id = $2 AND deleted_at IS NOT NULL
	`, updatedBy, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrMediaObjectNotFound
	}
	return nil
}

// RestoreSubtree restores ancestor and every soft-deleted descendant via object_paths.
func (r *MediaObjectRepository) RestoreSubtree(ctx context.Context, ancestorID, updatedBy int64) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE media_objects m
		SET deleted_at = NULL, status = 'active', updated_by = $2, updated_at = now()
		FROM object_paths op
		WHERE op.ancestor_id = $1
		  AND op.descendant_id = m.id
		  AND m.deleted_at IS NOT NULL
	`, ancestorID, updatedBy)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ListDeletedInSubtree returns soft-deleted objects under ancestor (including ancestor).
func (r *MediaObjectRepository) ListDeletedInSubtree(ctx context.Context, ancestorID int64) ([]MediaObject, error) {
	return r.listDeletedInSubtreePage(ctx, ancestorID, 0, 0)
}

// ListDeletedInSubtreePage returns a page of soft-deleted subtree objects after afterID.
func (r *MediaObjectRepository) ListDeletedInSubtreePage(ctx context.Context, ancestorID, afterID int64, limit int) ([]MediaObject, error) {
	return r.listDeletedInSubtreePage(ctx, ancestorID, afterID, limit)
}

func (r *MediaObjectRepository) listDeletedInSubtreePage(ctx context.Context, ancestorID, afterID int64, limit int) ([]MediaObject, error) {
	unbounded := limit <= 0
	pageLimit := limit
	if !unbounded && pageLimit > 500 {
		pageLimit = subtreePageDefault
	}
	args := []any{ancestorID}
	query := `
		SELECT m.id, m.public_id, m.parent_id, p.public_id, m.type, m.name, m.original_name,
		       m.mime_type, m.size_bytes, m.storage_key, m.checksum, m.thumbnail_key, m.status,
		       m.created_by, m.updated_by, m.deleted_at, m.created_at, m.updated_at
		FROM media_objects m
		JOIN object_paths op ON op.descendant_id = m.id
		LEFT JOIN media_objects p ON p.id = m.parent_id
		WHERE op.ancestor_id = $1 AND m.deleted_at IS NOT NULL
	`
	if afterID > 0 {
		query += ` AND m.id > $2`
		args = append(args, afterID)
	}
	query += ` ORDER BY m.id ASC`
	if !unbounded {
		n := len(args) + 1
		query += fmt.Sprintf(` LIMIT $%d`, n)
		args = append(args, pageLimit)
	}
	return r.scanMediaObjectRows(ctx, query, args...)
}

// HardDeleteSubtree permanently removes soft-deleted ancestor and descendants.
func (r *MediaObjectRepository) HardDeleteSubtree(ctx context.Context, ancestorID int64) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM media_objects m
		USING object_paths op
		WHERE op.ancestor_id = $1
		  AND op.descendant_id = m.id
		  AND m.deleted_at IS NOT NULL
	`, ancestorID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// SoftDeleteSubtree soft-deletes ancestor and every active descendant via object_paths.
func (r *MediaObjectRepository) SoftDeleteSubtree(ctx context.Context, ancestorID, updatedBy int64) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE media_objects m
		SET deleted_at = now(), updated_by = $2, updated_at = now(), status = 'deleted'
		FROM object_paths op
		WHERE op.ancestor_id = $1
		  AND op.descendant_id = m.id
		  AND m.deleted_at IS NULL
	`, ancestorID, updatedBy)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// RepairClosurePaths removes duplicate closure rows, keeping the minimum depth per pair.
func (r *MediaObjectRepository) RepairClosurePaths(ctx context.Context) (int64, error) {
	const batch = 5000
	var total int64
	for {
		tag, err := r.pool.Exec(ctx, `
			DELETE FROM object_paths op
			WHERE ctid IN (
				SELECT op2.ctid
				FROM object_paths op2
				WHERE EXISTS (
					SELECT 1 FROM object_paths newer
					WHERE newer.ancestor_id = op2.ancestor_id
					  AND newer.descendant_id = op2.descendant_id
					  AND newer.depth < op2.depth
				)
				LIMIT $1
			)
		`, batch)
		if err != nil {
			return total, fmt.Errorf("repair closure paths: %w", err)
		}
		n := tag.RowsAffected()
		total += n
		if n < batch {
			break
		}
	}
	return total, nil
}

// MediaObjectSummary holds active object counts for dashboard summaries.
type MediaObjectSummary struct {
	Total   int64
	Folders int64
	Files   int64 // non-folder objects (file, image, video, …)
}

// CountActiveSummary counts active media objects grouped for dashboard stats.
func (r *MediaObjectRepository) CountActiveSummary(ctx context.Context) (*MediaObjectSummary, error) {
	var s MediaObjectSummary
	err := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*)::bigint,
			COUNT(*) FILTER (WHERE type = 'folder')::bigint,
			COUNT(*) FILTER (WHERE type <> 'folder')::bigint
		FROM media_objects
		WHERE deleted_at IS NULL AND status = 'active'
	`).Scan(&s.Total, &s.Folders, &s.Files)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// CountActiveChildren returns direct and nested active descendants (excluding the folder itself).
func (r *MediaObjectRepository) CountActiveChildren(ctx context.Context, folderID int64) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM object_paths op
		JOIN media_objects m ON m.id = op.descendant_id
		WHERE op.ancestor_id = $1
		  AND op.depth > 0
		  AND m.deleted_at IS NULL
		  AND m.status = 'active'
	`, folderID).Scan(&n)
	return n, err
}

// ReferencedStorageKeysAmong returns which keys in keys exist as storage_key or thumbnail_key in DB.
func (r *MediaObjectRepository) ReferencedStorageKeysAmong(ctx context.Context, keys []string) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	if len(keys) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT storage_key FROM media_objects
		WHERE storage_key = ANY($1)
		UNION
		SELECT thumbnail_key FROM media_objects
		WHERE thumbnail_key = ANY($1)
	`, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out[key] = struct{}{}
	}
	return out, rows.Err()
}

// ListReferencedStorageKeys returns storage_key and thumbnail_key values from all media_objects.
func (r *MediaObjectRepository) ListReferencedStorageKeys(ctx context.Context) (map[string]struct{}, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT storage_key FROM media_objects WHERE storage_key IS NOT NULL AND storage_key <> ''
		UNION
		SELECT thumbnail_key FROM media_objects WHERE thumbnail_key IS NOT NULL AND thumbnail_key <> ''
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]struct{})
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out[key] = struct{}{}
	}
	return out, rows.Err()
}

// ListKnownPublicIDs returns all media_objects.public_id strings (any type/status).
func (r *MediaObjectRepository) ListKnownPublicIDs(ctx context.Context) (map[string]struct{}, error) {
	rows, err := r.pool.Query(ctx, `SELECT public_id::text FROM media_objects`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

func (r *MediaObjectRepository) CreateVideoAsset(ctx context.Context, objectID int64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO video_assets (object_id, hls_status)
		VALUES ($1, 'none')
		ON CONFLICT (object_id) DO NOTHING
	`, objectID)
	return err
}
