package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"github.com/anhtuanlc/mediahub/internal/auth"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrAPIKeyAccessDenied = errors.New("api key access denied")
	ErrAPIKeyInvalid      = errors.New("invalid api key")
)

type APIKeyDTO struct {
	PublicID       string   `json:"public_id"`
	Name           string   `json:"name"`
	Scopes         []string `json:"scopes"`
	AllowedDomains []string `json:"allowed_domains"`
	AllowedIPs     []string `json:"allowed_ips"`
	Status         string   `json:"status"`
	LastUsedAt     *string  `json:"last_used_at,omitempty"`
	CreatedAt      string   `json:"created_at"`
}

type APIKeyService struct {
	keys  *repository.APIKeyRepository
	audit *repository.AuditRepository
}

func NewAPIKeyService(keys *repository.APIKeyRepository, audit *repository.AuditRepository) *APIKeyService {
	return &APIKeyService{keys: keys, audit: audit}
}

func toAPIKeyDTO(k *repository.APIKey) APIKeyDTO {
	dto := APIKeyDTO{
		PublicID:       k.PublicID.String(),
		Name:           k.Name,
		Scopes:         k.Scopes,
		AllowedDomains: k.AllowedDomains,
		AllowedIPs:     k.AllowedIPs,
		Status:         k.Status,
		CreatedAt:      k.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if k.LastUsedAt != nil {
		s := k.LastUsedAt.UTC().Format("2006-01-02T15:04:05Z")
		dto.LastUsedAt = &s
	}
	return dto
}

type CreateAPIKeyInput struct {
	Name           string
	Scopes         []string
	AllowedDomains []string
	AllowedIPs     []string
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
		scopes = []string{"stream"}
	}
	k, err := s.keys.Create(ctx, uuid.New(), in.Name, hash, scopes, in.AllowedDomains, in.AllowedIPs, createdBy)
	if err != nil {
		return nil, err
	}
	actorID := createdBy
	_ = s.audit.Log(ctx, &actorID, "api_key.create", "api_key", &k.ID, ip, ua, map[string]string{"public_id": k.PublicID.String()})
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
	aid := actorID
	_ = s.audit.Log(ctx, &aid, "api_key.revoke", "api_key", &k.ID, ip, ua, nil)
	return nil
}

func (s *APIKeyService) Update(ctx context.Context, publicID uuid.UUID, name *string, scopes, domains, ips []string, status *string) (*APIKeyDTO, error) {
	k, err := s.keys.Update(ctx, publicID, name, scopes, domains, ips, status)
	if err != nil {
		return nil, err
	}
	dto := toAPIKeyDTO(k)
	return &dto, nil
}

// VerifyAPIKey checks plain key against active keys (linear scan; acceptable for small key counts).
func (s *APIKeyService) VerifyAPIKey(ctx context.Context, plain string) (*repository.APIKey, error) {
	if plain == "" {
		return nil, ErrAPIKeyInvalid
	}
	hash := auth.HashToken(plain)
	list, err := s.keys.ListActiveHashes(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].KeyHash == hash {
			_ = s.keys.TouchLastUsed(ctx, list[i].ID)
			return &list[i], nil
		}
	}
	return nil, ErrAPIKeyInvalid
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

func (s *APIKeyService) MatchDomain(k *repository.APIKey, origin string) bool {
	return MatchDomainAllowlist(origin, k.AllowedDomains, nil)
}
