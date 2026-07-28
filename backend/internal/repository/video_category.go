package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrVideoCategoryNotFound = errors.New("video category not found")
	ErrVideoCategoryExists   = errors.New("video category already exists")
)

type VideoCategory struct {
	ID         int64
	PublicID   uuid.UUID
	Name       string
	SortOrder  int
	CreatedBy  *int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
	VideoCount int64
}

type VideoCategoryRepository struct {
	pool *pgxpool.Pool
}

func NewVideoCategoryRepository(pool *pgxpool.Pool) *VideoCategoryRepository {
	return &VideoCategoryRepository{pool: pool}
}

func scanVideoCategory(row pgx.Row) (*VideoCategory, error) {
	var c VideoCategory
	err := row.Scan(
		&c.ID, &c.PublicID, &c.Name, &c.SortOrder, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVideoCategoryNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *VideoCategoryRepository) List(ctx context.Context) ([]VideoCategory, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.public_id, c.name, c.sort_order, c.created_by, c.created_at, c.updated_at,
		       COALESCE(COUNT(m.id), 0)
		FROM video_categories c
		LEFT JOIN video_assets v ON v.category_id = c.id
		LEFT JOIN media_objects m ON m.id = v.object_id AND m.deleted_at IS NULL AND m.type = 'video'
		GROUP BY c.id
		ORDER BY c.sort_order ASC, lower(c.name) ASC, c.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []VideoCategory
	for rows.Next() {
		var c VideoCategory
		if err := rows.Scan(
			&c.ID, &c.PublicID, &c.Name, &c.SortOrder, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
			&c.VideoCount,
		); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *VideoCategoryRepository) GetByPublicID(ctx context.Context, publicID uuid.UUID) (*VideoCategory, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, public_id, name, sort_order, created_by, created_at, updated_at
		FROM video_categories WHERE public_id = $1
	`, publicID)
	return scanVideoCategory(row)
}

func (r *VideoCategoryRepository) Create(ctx context.Context, name string, createdBy *int64) (*VideoCategory, error) {
	name = strings.TrimSpace(name)
	row := r.pool.QueryRow(ctx, `
		INSERT INTO video_categories (public_id, name, created_by)
		VALUES ($1, $2, $3)
		RETURNING id, public_id, name, sort_order, created_by, created_at, updated_at
	`, uuid.New(), name, createdBy)
	c, err := scanVideoCategory(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrVideoCategoryExists
		}
		return nil, err
	}
	return c, nil
}

func (r *VideoCategoryRepository) UpdateName(ctx context.Context, id int64, name string) (*VideoCategory, error) {
	name = strings.TrimSpace(name)
	row := r.pool.QueryRow(ctx, `
		UPDATE video_categories
		SET name = $2, updated_at = now()
		WHERE id = $1
		RETURNING id, public_id, name, sort_order, created_by, created_at, updated_at
	`, id, name)
	c, err := scanVideoCategory(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrVideoCategoryExists
		}
		return nil, err
	}
	return c, nil
}

func (r *VideoCategoryRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM video_categories WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrVideoCategoryNotFound
	}
	return nil
}
