package rediscache

import (
	"context"
	"errors"

	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

// CachedAuthUser is a minimal user record for auth middleware.
type CachedAuthUser struct {
	ID       int64     `json:"id"`
	PublicID uuid.UUID `json:"public_id"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	Status   string    `json:"status"`
}

// AuthUserCache loads users by public id with Redis backing.
type AuthUserCache struct {
	Store *Store
	Users *repository.UserRepository
}

func (c *AuthUserCache) Enabled() bool {
	return c != nil && c.Store != nil && c.Store.Enabled() && c.Users != nil
}

func (c *AuthUserCache) GetByPublicID(ctx context.Context, publicID uuid.UUID) (*repository.User, error) {
	if c == nil || c.Users == nil {
		return nil, errors.New("auth user cache not configured")
	}
	if !c.Enabled() {
		return c.Users.GetByPublicID(ctx, publicID)
	}
	key := KeyAuthUser(publicID.String())
	var cached CachedAuthUser
	_, err := c.Store.GetOrLoadJSON(ctx, key, TTLAuthUser, &cached, func(ctx context.Context) (any, error) {
		u, err := c.Users.GetByPublicID(ctx, publicID)
		if err != nil {
			return nil, err
		}
		return CachedAuthUser{
			ID:       u.ID,
			PublicID: u.PublicID,
			Email:    u.Email,
			Role:     u.Role,
			Status:   u.Status,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return &repository.User{
		ID:       cached.ID,
		PublicID: cached.PublicID,
		Email:    cached.Email,
		Role:     cached.Role,
		Status:   cached.Status,
	}, nil
}

func InvalidateAuthUser(ctx context.Context, store *Store, publicID uuid.UUID) {
	if store == nil || !store.Enabled() {
		return
	}
	_ = store.Delete(ctx, KeyAuthUser(publicID.String()))
}
