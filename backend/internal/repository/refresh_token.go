package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrRefreshTokenNotFound = errors.New("refresh token not found")

type RefreshToken struct {
	ID        int64
	UserID    int64
	TokenHash string
	FamilyID  uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type RefreshTokenRepository struct {
	pool *pgxpool.Pool
}

func NewRefreshTokenRepository(pool *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{pool: pool}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, userID int64, tokenHash string, familyID uuid.UUID, expiresAt time.Time) (*RefreshToken, error) {
	var rt RefreshToken
	err := r.pool.QueryRow(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, family_id, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, token_hash, family_id, expires_at, revoked_at
	`, userID, tokenHash, familyID, expiresAt).Scan(
		&rt.ID, &rt.UserID, &rt.TokenHash, &rt.FamilyID, &rt.ExpiresAt, &rt.RevokedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}
	return &rt, nil
}

func (r *RefreshTokenRepository) GetByHash(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	var rt RefreshToken
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, family_id, expires_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`, tokenHash).Scan(
		&rt.ID, &rt.UserID, &rt.TokenHash, &rt.FamilyID, &rt.ExpiresAt, &rt.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRefreshTokenNotFound
		}
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return &rt, nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, id int64, replacedBy *int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = now(), replaced_by = $2
		WHERE id = $1 AND revoked_at IS NULL
	`, id, replacedBy)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID)
	if err != nil {
		return fmt.Errorf("revoke all for user: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) RevokeFamily(ctx context.Context, familyID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE family_id = $1 AND revoked_at IS NULL
	`, familyID)
	if err != nil {
		return fmt.Errorf("revoke refresh family: %w", err)
	}
	return nil
}
