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

var ErrAPIKeyNotFound = errors.New("api key not found")

type APIKey struct {
	ID                  int64
	PublicID            uuid.UUID
	Name                string
	KeyHash             string
	Scopes              []string
	AllowedDomains      []string
	AllowedIPs          []string
	RootFolderPublicID  *uuid.UUID
	Status              string
	CreatedBy           *int64
	LastUsedAt          *time.Time
	CreatedAt           time.Time
}

type APIKeyRepository struct {
	pool *pgxpool.Pool
}

func NewAPIKeyRepository(pool *pgxpool.Pool) *APIKeyRepository {
	return &APIKeyRepository{pool: pool}
}

func scanAPIKey(row pgx.Row) (*APIKey, error) {
	var k APIKey
	var scopesJSON, domainsJSON, ipsJSON []byte
	err := row.Scan(
		&k.ID, &k.PublicID, &k.Name, &k.KeyHash, &scopesJSON, &domainsJSON, &ipsJSON,
		&k.RootFolderPublicID, &k.Status, &k.CreatedBy, &k.LastUsedAt, &k.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}
	_ = json.Unmarshal(scopesJSON, &k.Scopes)
	_ = json.Unmarshal(domainsJSON, &k.AllowedDomains)
	_ = json.Unmarshal(ipsJSON, &k.AllowedIPs)
	if k.Scopes == nil {
		k.Scopes = []string{}
	}
	if k.AllowedDomains == nil {
		k.AllowedDomains = []string{}
	}
	if k.AllowedIPs == nil {
		k.AllowedIPs = []string{}
	}
	return &k, nil
}

func (r *APIKeyRepository) Create(ctx context.Context, publicID uuid.UUID, name, keyHash string, scopes, ips []string, rootFolderPublicID *uuid.UUID, createdBy int64) (*APIKey, error) {
	scopesJSON, _ := json.Marshal(scopes)
	domainsJSON, _ := json.Marshal([]string{})
	ipsJSON, _ := json.Marshal(ips)
	row := r.pool.QueryRow(ctx, `
		INSERT INTO api_keys (public_id, name, key_hash, scopes, allowed_domains, allowed_ips, root_folder_public_id, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, public_id, name, key_hash, scopes, allowed_domains, allowed_ips, root_folder_public_id, status, created_by, last_used_at, created_at
	`, publicID, name, keyHash, scopesJSON, domainsJSON, ipsJSON, rootFolderPublicID, createdBy)
	return scanAPIKey(row)
}

func (r *APIKeyRepository) List(ctx context.Context) ([]APIKey, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, public_id, name, key_hash, scopes, allowed_domains, allowed_ips, root_folder_public_id, status, created_by, last_used_at, created_at
		FROM api_keys ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *k)
	}
	return list, rows.Err()
}

func (r *APIKeyRepository) GetByPublicID(ctx context.Context, publicID uuid.UUID) (*APIKey, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, public_id, name, key_hash, scopes, allowed_domains, allowed_ips, root_folder_public_id, status, created_by, last_used_at, created_at
		FROM api_keys WHERE public_id = $1
	`, publicID)
	return scanAPIKey(row)
}

func (r *APIKeyRepository) ListActiveHashes(ctx context.Context) ([]APIKey, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, public_id, name, key_hash, scopes, allowed_domains, allowed_ips, root_folder_public_id, status, created_by, last_used_at, created_at
		FROM api_keys WHERE status = 'active'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *k)
	}
	return list, rows.Err()
}

func (r *APIKeyRepository) Update(ctx context.Context, publicID uuid.UUID, name *string, scopes, ips []string, rootFolderPublicID **uuid.UUID, status *string) (*APIKey, error) {
	cur, err := r.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	n := cur.Name
	if name != nil {
		n = *name
	}
	st := cur.Status
	if status != nil {
		st = *status
	}
	if scopes == nil {
		scopes = cur.Scopes
	}
	if ips == nil {
		ips = cur.AllowedIPs
	}
	rootFolder := cur.RootFolderPublicID
	if rootFolderPublicID != nil {
		rootFolder = *rootFolderPublicID
	}
	scopesJSON, _ := json.Marshal(scopes)
	domainsJSON, _ := json.Marshal([]string{})
	ipsJSON, _ := json.Marshal(ips)
	row := r.pool.QueryRow(ctx, `
		UPDATE api_keys SET name = $2, scopes = $3, allowed_domains = $4, allowed_ips = $5, root_folder_public_id = $6, status = $7
		WHERE public_id = $1
		RETURNING id, public_id, name, key_hash, scopes, allowed_domains, allowed_ips, root_folder_public_id, status, created_by, last_used_at, created_at
	`, publicID, n, scopesJSON, domainsJSON, ipsJSON, rootFolder, st)
	return scanAPIKey(row)
}

func (r *APIKeyRepository) Revoke(ctx context.Context, publicID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE api_keys SET status = 'revoked' WHERE public_id = $1`, publicID)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	return nil
}

func (r *APIKeyRepository) TouchLastUsed(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE api_keys SET last_used_at = now() WHERE id = $1`, id)
	return err
}
