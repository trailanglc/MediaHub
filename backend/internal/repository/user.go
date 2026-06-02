package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

var ErrUserNotFound = errors.New("user not found")

type User struct {
	ID        int64
	PublicID  uuid.UUID
	Email     string
	Role      string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) HasOwner(ctx context.Context) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM users WHERE role = 'owner' AND status = 'active'
		)
	`).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("has owner: %w", err)
	}
	return exists, nil
}

func (r *UserRepository) CreateOwner(ctx context.Context, email, passwordHash string, publicID uuid.UUID, ip, userAgent string) (*User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var ownerExists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM users WHERE role = 'owner' AND status = 'active'
		)
	`).Scan(&ownerExists); err != nil {
		return nil, fmt.Errorf("check owner in tx: %w", err)
	}
	if ownerExists {
		return nil, ErrOwnerAlreadyExists
	}

	var u User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (public_id, email, password_hash, role, status)
		VALUES ($1, $2, $3, 'owner', 'active')
		RETURNING id, public_id, email, role, status, created_at, updated_at
	`, publicID, email, passwordHash).Scan(
		&u.ID, &u.PublicID, &u.Email, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert owner: %w", err)
	}

	metadata, _ := json.Marshal(map[string]string{"email": email})
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, action, target_type, target_id, ip, user_agent, metadata)
		VALUES ($1, 'setup.owner_created', 'user', $1, $2, $3, $4)
	`, u.ID, ip, userAgent, metadata)
	if err != nil {
		return nil, fmt.Errorf("insert audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &u, nil
}

var (
	ErrOwnerAlreadyExists = errors.New("owner already exists")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrCannotModifyOwner  = errors.New("cannot modify owner")
)

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	var u User
	var passwordHash string
	err := r.pool.QueryRow(ctx, `
		SELECT id, public_id, email, password_hash, role, status, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(
		&u.ID, &u.PublicID, &u.Email, &passwordHash, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", ErrUserNotFound
		}
		return nil, "", fmt.Errorf("get by email: %w", err)
	}
	return &u, passwordHash, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		SELECT id, public_id, email, role, status, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.PublicID, &u.Email, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get by id: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) GetByPublicID(ctx context.Context, publicID uuid.UUID) (*User, error) {
	var u User
	err := r.pool.QueryRow(ctx, `
		SELECT id, public_id, email, role, status, created_at, updated_at
		FROM users WHERE public_id = $1
	`, publicID).Scan(&u.ID, &u.PublicID, &u.Email, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get by public id: %w", err)
	}
	return &u, nil
}

type ListMembersParams struct {
	Limit  int
	Cursor int64
}

func (r *UserRepository) ListMembers(ctx context.Context, p ListMembersParams) ([]User, error) {
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, public_id, email, role, status, created_at, updated_at
		FROM users
		WHERE role != 'owner' AND id > $1
		ORDER BY id ASC
		LIMIT $2
	`, p.Cursor, p.Limit)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.PublicID, &u.Email, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepository) CreateMember(ctx context.Context, email, passwordHash, role string, publicID uuid.UUID, createdBy int64) (*User, error) {
	if role != "manager" && role != "viewer" {
		return nil, fmt.Errorf("invalid role: %s", role)
	}
	email = strings.TrimSpace(strings.ToLower(email))
	var u User
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (public_id, email, password_hash, role, status)
		VALUES ($1, $2, $3, $4, 'active')
		RETURNING id, public_id, email, role, status, created_at, updated_at
	`, publicID, email, passwordHash, role).Scan(
		&u.ID, &u.PublicID, &u.Email, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailAlreadyExists
		}
		return nil, fmt.Errorf("create member: %w", err)
	}
	_ = createdBy
	return &u, nil
}

type UpdateMemberInput struct {
	Role         *string
	Status       *string
	PasswordHash *string
}

func (r *UserRepository) UpdateMember(ctx context.Context, publicID uuid.UUID, in UpdateMemberInput) (*User, error) {
	u, err := r.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if u.Role == "owner" {
		return nil, ErrCannotModifyOwner
	}
	if in.Role != nil && *in.Role != "manager" && *in.Role != "viewer" {
		return nil, fmt.Errorf("invalid role: %s", *in.Role)
	}
	err = r.pool.QueryRow(ctx, `
		UPDATE users SET
			role = COALESCE($2, role),
			status = COALESCE($3, status),
			password_hash = COALESCE($4, password_hash),
			updated_at = now()
		WHERE public_id = $1 AND role != 'owner'
		RETURNING id, public_id, email, role, status, created_at, updated_at
	`, publicID, in.Role, in.Status, in.PasswordHash).Scan(
		&u.ID, &u.PublicID, &u.Email, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update member: %w", err)
	}
	return u, nil
}

func clearUserReferences(ctx context.Context, tx pgx.Tx, userID int64) error {
	queries := []string{
		`UPDATE media_objects SET created_by = NULL WHERE created_by = $1`,
		`UPDATE media_objects SET updated_by = NULL WHERE updated_by = $1`,
		`UPDATE permissions SET granted_by = NULL WHERE granted_by = $1`,
		`UPDATE api_keys SET created_by = NULL WHERE created_by = $1`,
		`UPDATE convert_jobs SET created_by = NULL WHERE created_by = $1`,
		`UPDATE audit_logs SET actor_id = NULL WHERE actor_id = $1`,
		`UPDATE system_settings SET updated_by = NULL WHERE updated_by = $1`,
	}
	for _, q := range queries {
		if _, err := tx.Exec(ctx, q, userID); err != nil {
			return fmt.Errorf("clear user references: %w", err)
		}
	}
	return nil
}

func (r *UserRepository) DeleteMember(ctx context.Context, publicID uuid.UUID) (*User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var u User
	err = tx.QueryRow(ctx, `
		SELECT id, public_id, email, role, status, created_at, updated_at
		FROM users WHERE public_id = $1
	`, publicID).Scan(
		&u.ID, &u.PublicID, &u.Email, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("load member: %w", err)
	}
	if u.Role == "owner" {
		return nil, ErrCannotModifyOwner
	}
	if err := clearUserReferences(ctx, tx, u.ID); err != nil {
		return nil, err
	}
	err = tx.QueryRow(ctx, `
		DELETE FROM users WHERE id = $1 AND role != 'owner'
		RETURNING id, public_id, email, role, status, created_at, updated_at
	`, u.ID).Scan(
		&u.ID, &u.PublicID, &u.Email, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("delete member: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit delete member: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) CountActiveOwners(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM users WHERE role = 'owner' AND status = 'active'
	`).Scan(&n)
	return n, err
}
