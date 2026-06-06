package rediscache

import (
	"context"
	"errors"

	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

// CachedAPIKey holds fields required for API key auth (no secret plaintext).
type CachedAPIKey struct {
	ID                 int64      `json:"id"`
	PublicID           uuid.UUID  `json:"public_id"`
	KeyHash            string     `json:"key_hash"`
	Scopes             []string   `json:"scopes"`
	AllowedIPs         []string   `json:"allowed_ips"`
	RootFolderPublicID *uuid.UUID `json:"root_folder_public_id,omitempty"`
	Status             string     `json:"status"`
	CreatedBy          *int64     `json:"created_by,omitempty"`
}

// APIKeyCache resolves active API keys by hash with Redis backing.
type APIKeyCache struct {
	Store *Store
	Keys  *repository.APIKeyRepository
}

func (c *APIKeyCache) Enabled() bool {
	return c != nil && c.Store != nil && c.Store.Enabled() && c.Keys != nil
}

func APIKeyFromCached(cached CachedAPIKey) *repository.APIKey {
	return &repository.APIKey{
		ID:                 cached.ID,
		PublicID:           cached.PublicID,
		KeyHash:            cached.KeyHash,
		Scopes:             cached.Scopes,
		AllowedIPs:         cached.AllowedIPs,
		RootFolderPublicID: cached.RootFolderPublicID,
		Status:             cached.Status,
		CreatedBy:          cached.CreatedBy,
	}
}

func CachedAPIKeyFromRecord(k *repository.APIKey) CachedAPIKey {
	if k == nil {
		return CachedAPIKey{}
	}
	return CachedAPIKey{
		ID:                 k.ID,
		PublicID:           k.PublicID,
		KeyHash:            k.KeyHash,
		Scopes:             k.Scopes,
		AllowedIPs:         k.AllowedIPs,
		RootFolderPublicID: k.RootFolderPublicID,
		Status:             k.Status,
		CreatedBy:          k.CreatedBy,
	}
}

func (c *APIKeyCache) GetByHash(ctx context.Context, keyHash string) (*repository.APIKey, error) {
	if c == nil || c.Keys == nil {
		return nil, errors.New("api key cache not configured")
	}
	if keyHash == "" {
		return nil, repository.ErrAPIKeyNotFound
	}
	if !c.Enabled() {
		return c.Keys.GetActiveByHash(ctx, keyHash)
	}
	cacheKey := KeyAPIKeyHash(keyHash)
	var cached CachedAPIKey
	_, err := c.Store.GetOrLoadJSON(ctx, cacheKey, TTLAPIKey, &cached, func(ctx context.Context) (any, error) {
		k, err := c.Keys.GetActiveByHash(ctx, keyHash)
		if err != nil {
			return nil, err
		}
		return CachedAPIKeyFromRecord(k), nil
	})
	if err != nil {
		return nil, err
	}
	if cached.Status != "active" {
		_ = c.Store.Delete(ctx, cacheKey)
		return nil, repository.ErrAPIKeyNotFound
	}
	return APIKeyFromCached(cached), nil
}

func InvalidateAPIKeyHash(ctx context.Context, store *Store, keyHash string) {
	if store == nil || !store.Enabled() || keyHash == "" {
		return
	}
	_ = store.Delete(ctx, KeyAPIKeyHash(keyHash))
}

func (c *APIKeyCache) Set(ctx context.Context, k *repository.APIKey) {
	if !c.Enabled() || k == nil || k.KeyHash == "" || k.Status != "active" {
		return
	}
	_ = c.Store.SetJSON(ctx, KeyAPIKeyHash(k.KeyHash), CachedAPIKeyFromRecord(k), TTLAPIKey)
}
