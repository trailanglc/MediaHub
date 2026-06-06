package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"

	"github.com/anhtuanlc/mediahub/internal/auth"
	"github.com/anhtuanlc/mediahub/internal/integration"
	"github.com/anhtuanlc/mediahub/internal/platform/rediscache"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrAPIKeyAccessDenied    = errors.New("api key access denied")
	ErrAPIKeyInvalid         = errors.New("invalid api key")
	ErrAPIKeyInvalidScope    = errors.New("invalid api key scope")
	ErrAPIKeyInvalidStatus   = errors.New("invalid api key status")
	ErrAPIKeyInvalidRootFolder = errors.New("invalid root folder")
)

type APIKeyDTO struct {
	PublicID           string   `json:"public_id"`
	Name               string   `json:"name"`
	Scopes             []string `json:"scopes"`
	AllowedIPs         []string `json:"allowed_ips"`
	RootFolderPublicID *string  `json:"root_folder_public_id,omitempty"`
	Status             string   `json:"status"`
	LastUsedAt         *string  `json:"last_used_at,omitempty"`
	CreatedAt          string   `json:"created_at"`
}

type APIKeyService struct {
	keys    *repository.APIKeyRepository
	audit   *repository.AuditRepository
	objects *repository.MediaObjectRepository
	cache   *rediscache.APIKeyCache
}

func NewAPIKeyService(
	keys *repository.APIKeyRepository,
	audit *repository.AuditRepository,
	objects *repository.MediaObjectRepository,
	cache *rediscache.APIKeyCache,
) *APIKeyService {
	return &APIKeyService{keys: keys, audit: audit, objects: objects, cache: cache}
}

func (s *APIKeyService) invalidateKeyCache(ctx context.Context, keyHash string) {
	if s.cache != nil && s.cache.Store != nil {
		rediscache.InvalidateAPIKeyHash(ctx, s.cache.Store, keyHash)
	}
}

func validateScopes(scopes []string) error {
	if len(scopes) == 0 {
		return ErrAPIKeyInvalidScope
	}
	for _, sc := range scopes {
		if !integration.HasScope(integration.KnownScopes, sc) {
			return ErrAPIKeyInvalidScope
		}
	}
	return nil
}

func validateAPIKeyStatus(status string) error {
	switch status {
	case "active", "revoked":
		return nil
	default:
		return ErrAPIKeyInvalidStatus
	}
}

func (s *APIKeyService) validateRootFolder(ctx context.Context, pid *uuid.UUID) error {
	if pid == nil || s.objects == nil {
		return nil
	}
	obj, err := s.objects.GetByPublicID(ctx, *pid)
	if err != nil {
		if errors.Is(err, repository.ErrMediaObjectNotFound) {
			return ErrAPIKeyInvalidRootFolder
		}
		return err
	}
	if obj.Type != "folder" {
		return ErrAPIKeyInvalidRootFolder
	}
	return nil
}

func toAPIKeyDTO(k *repository.APIKey) APIKeyDTO {
	dto := APIKeyDTO{
		PublicID:   k.PublicID.String(),
		Name:       k.Name,
		Scopes:     k.Scopes,
		AllowedIPs: k.AllowedIPs,
		Status:     k.Status,
		CreatedAt:  k.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if k.RootFolderPublicID != nil {
		s := k.RootFolderPublicID.String()
		dto.RootFolderPublicID = &s
	}
	if k.LastUsedAt != nil {
		s := k.LastUsedAt.UTC().Format("2006-01-02T15:04:05Z")
		dto.LastUsedAt = &s
	}
	return dto
}

type CreateAPIKeyInput struct {
	Name               string
	Scopes             []string
	AllowedIPs         []string
	RootFolderPublicID *uuid.UUID
}

type CreateAPIKeyResult struct {
	Key    APIKeyDTO `json:"key"`
	Secret string    `json:"secret"`
}

func (s *APIKeyService) List(ctx context.Context) ([]APIKeyDTO, error) {
	list, err := s.keys.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]APIKeyDTO, 0, len(list))
	for i := range list {
		out = append(out, toAPIKeyDTO(&list[i]))
	}
	return out, nil
}

func (s *APIKeyService) Create(ctx context.Context, createdBy int64, ip, ua string, in CreateAPIKeyInput) (*CreateAPIKeyResult, error) {
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, err
	}
	plain := "mh_" + base64.RawURLEncoding.EncodeToString(secretBytes)
	hash := auth.HashToken(plain)
	scopes := in.Scopes
	if len(scopes) == 0 {
		scopes = []string{integration.ScopeStream}
	} else if err := validateScopes(scopes); err != nil {
		return nil, err
	}
	if err := s.validateRootFolder(ctx, in.RootFolderPublicID); err != nil {
		return nil, err
	}
	k, err := s.keys.Create(ctx, uuid.New(), in.Name, hash, scopes, in.AllowedIPs, in.RootFolderPublicID, createdBy)
	if err != nil {
		return nil, err
	}
	actorID := createdBy
	_ = s.audit.Log(ctx, &actorID, "api_key.create", "api_key", &k.ID, ip, ua, map[string]string{"public_id": k.PublicID.String()})
	if s.cache != nil {
		s.cache.Set(ctx, k)
	}
	return &CreateAPIKeyResult{Key: toAPIKeyDTO(k), Secret: plain}, nil
}

func (s *APIKeyService) Revoke(ctx context.Context, actorID int64, ip, ua string, publicID uuid.UUID) error {
	k, err := s.keys.GetByPublicID(ctx, publicID)
	if err != nil {
		return err
	}
	if err := s.keys.Revoke(ctx, publicID); err != nil {
		return err
	}
	s.invalidateKeyCache(ctx, k.KeyHash)
	aid := actorID
	_ = s.audit.Log(ctx, &aid, "api_key.revoke", "api_key", &k.ID, ip, ua, nil)
	return nil
}

func (s *APIKeyService) Update(ctx context.Context, actorID int64, ip, ua string, publicID uuid.UUID, name *string, scopes, ips []string, rootFolderPublicID **uuid.UUID, status *string) (*APIKeyDTO, error) {
	if scopes != nil {
		if err := validateScopes(scopes); err != nil {
			return nil, err
		}
	}
	if status != nil {
		if err := validateAPIKeyStatus(*status); err != nil {
			return nil, err
		}
	}
	if rootFolderPublicID != nil && *rootFolderPublicID != nil {
		if err := s.validateRootFolder(ctx, *rootFolderPublicID); err != nil {
			return nil, err
		}
	}
	k, err := s.keys.Update(ctx, publicID, name, scopes, ips, rootFolderPublicID, status)
	if err != nil {
		return nil, err
	}
	s.invalidateKeyCache(ctx, k.KeyHash)
	if k.Status == "active" && s.cache != nil {
		s.cache.Set(ctx, k)
	}
	aid := actorID
	_ = s.audit.Log(ctx, &aid, "api_key.update", "api_key", &k.ID, ip, ua, map[string]string{"public_id": k.PublicID.String()})
	dto := toAPIKeyDTO(k)
	return &dto, nil
}

// VerifyAPIKey resolves an active key by hash (Redis cache → single DB lookup on miss).
func (s *APIKeyService) VerifyAPIKey(ctx context.Context, plain string) (*repository.APIKey, error) {
	if plain == "" {
		return nil, ErrAPIKeyInvalid
	}
	hash := auth.HashToken(plain)
	var k *repository.APIKey
	var err error
	if s.cache != nil && s.cache.Enabled() {
		k, err = s.cache.GetByHash(ctx, hash)
	} else {
		k, err = s.keys.GetActiveByHash(ctx, hash)
	}
	if err != nil {
		if errors.Is(err, repository.ErrAPIKeyNotFound) {
			return nil, ErrAPIKeyInvalid
		}
		return nil, err
	}
	_ = s.keys.TouchLastUsed(ctx, k.ID)
	return k, nil
}

func (s *APIKeyService) HasScope(k *repository.APIKey, scope string) bool {
	for _, sc := range k.Scopes {
		if sc == scope {
			return true
		}
	}
	return false
}

func (s *APIKeyService) MatchIP(clientIP string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, ip := range allowed {
		if ip == clientIP {
			return true
		}
	}
	return false
}

// CheckIPRestriction enforces IP allowlist (empty list = no restriction).
func (s *APIKeyService) CheckIPRestriction(clientIP string, key *repository.APIKey) bool {
	return s.MatchIP(clientIP, key.AllowedIPs)
}
