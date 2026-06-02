package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/auth"
	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/platform"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrTooManyAttempts    = errors.New("too many login attempts")
	ErrInvalidRefresh     = errors.New("invalid refresh token")
)

type AuthUserDTO struct {
	PublicID string `json:"public_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

type AuthTokens struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
	AccessJTI        string
}

type AuthService struct {
	users     *repository.UserRepository
	refresh   *repository.RefreshTokenRepository
	audit     *repository.AuditRepository
	issuer    *auth.TokenIssuer
	revoke    *platform.TokenRevocation
	ratelimit *platform.LoginRateLimiter
	cfg       *config.Config
}

func NewAuthService(
	users *repository.UserRepository,
	refresh *repository.RefreshTokenRepository,
	audit *repository.AuditRepository,
	revoke *platform.TokenRevocation,
	ratelimit *platform.LoginRateLimiter,
	cfg *config.Config,
) *AuthService {
	secret := cfg.JWTSecret
	if secret == "" {
		secret = "dev-insecure-jwt-secret-change-me"
	}
	return &AuthService{
		users:     users,
		refresh:   refresh,
		audit:     audit,
		issuer:    auth.NewTokenIssuer(secret, cfg.JWTAccessTTL),
		revoke:    revoke,
		ratelimit: ratelimit,
		cfg:       cfg,
	}
}

func (s *AuthService) Issuer() *auth.TokenIssuer {
	return s.issuer
}

func (s *AuthService) CookieSecure() bool {
	return s.cfg.AppEnv == "production"
}

func (s *AuthService) AccessTTL() time.Duration {
	return s.cfg.JWTAccessTTL
}

func (s *AuthService) RefreshTTL() time.Duration {
	return s.cfg.JWTRefreshTTL
}

func (s *AuthService) Login(ctx context.Context, email, password, ip, userAgent string) (*AuthUserDTO, *AuthTokens, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	locked, err := s.ratelimit.IsLocked(ctx, email, ip)
	if err != nil {
		return nil, nil, err
	}
	if locked {
		return nil, nil, ErrTooManyAttempts
	}

	u, hash, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			_ = s.ratelimit.RecordFailure(ctx, email, ip)
			_ = s.audit.Log(ctx, nil, "auth.login_failed", "user", nil, ip, userAgent, map[string]string{"email": email})
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, err
	}
	if u.Status != "active" {
		return nil, nil, ErrAccountDisabled
	}
	if err := auth.ComparePassword(hash, password); err != nil {
		_ = s.ratelimit.RecordFailure(ctx, email, ip)
		actorID := u.ID
		_ = s.audit.Log(ctx, &actorID, "auth.login_failed", "user", &u.ID, ip, userAgent, map[string]string{"email": email})
		return nil, nil, ErrInvalidCredentials
	}

	_ = s.ratelimit.Clear(ctx, email, ip)
	tokens, err := s.issueTokenPair(ctx, u)
	if err != nil {
		return nil, nil, err
	}
	actorID := u.ID
	_ = s.audit.Log(ctx, &actorID, "auth.login", "user", &u.ID, ip, userAgent, map[string]string{"email": email})
	return userDTO(u), tokens, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshPlain, ip, userAgent string) (*AuthUserDTO, *AuthTokens, error) {
	if refreshPlain == "" {
		return nil, nil, ErrInvalidRefresh
	}
	rt, err := s.refresh.GetByHash(ctx, auth.HashToken(refreshPlain))
	if err != nil {
		if errors.Is(err, repository.ErrRefreshTokenNotFound) {
			return nil, nil, ErrInvalidRefresh
		}
		return nil, nil, err
	}
	if rt.RevokedAt != nil || time.Now().After(rt.ExpiresAt) {
		_ = s.refresh.RevokeFamily(ctx, rt.FamilyID)
		return nil, nil, ErrInvalidRefresh
	}

	u, err := s.users.GetByID(ctx, rt.UserID)
	if err != nil {
		return nil, nil, err
	}
	if u.Status != "active" {
		return nil, nil, ErrAccountDisabled
	}

	newTokens, err := s.rotateRefresh(ctx, rt, u)
	if err != nil {
		return nil, nil, err
	}
	actorID := u.ID
	_ = s.audit.Log(ctx, &actorID, "auth.refresh", "user", &u.ID, ip, userAgent, nil)
	return userDTO(u), newTokens, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshPlain, accessJTI string, accessTTL time.Duration, ip, userAgent string, userID *int64) error {
	if refreshPlain != "" {
		if rt, err := s.refresh.GetByHash(ctx, auth.HashToken(refreshPlain)); err == nil {
			_ = s.refresh.Revoke(ctx, rt.ID, nil)
		}
	}
	if accessJTI != "" && accessTTL > 0 {
		_ = s.revoke.RevokeJTI(ctx, accessJTI, accessTTL)
	}
	_ = s.audit.Log(ctx, userID, "auth.logout", "user", userID, ip, userAgent, nil)
	return nil
}

func (s *AuthService) Me(ctx context.Context, userID int64) (*AuthUserDTO, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.Status != "active" {
		return nil, ErrAccountDisabled
	}
	return userDTO(u), nil
}

func (s *AuthService) issueTokenPair(ctx context.Context, u *repository.User) (*AuthTokens, error) {
	plain, hash, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	familyID := uuid.New()
	expiresAt := time.Now().Add(s.cfg.JWTRefreshTTL)
	if _, err := s.refresh.Create(ctx, u.ID, hash, familyID, expiresAt); err != nil {
		return nil, err
	}
	access, jti, accessExp, err := s.issuer.IssueAccess(u.PublicID, u.Role)
	if err != nil {
		return nil, err
	}
	return &AuthTokens{
		AccessToken:      access,
		RefreshToken:     plain,
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: expiresAt,
		AccessJTI:        jti,
	}, nil
}

func (s *AuthService) rotateRefresh(ctx context.Context, old *repository.RefreshToken, u *repository.User) (*AuthTokens, error) {
	plain, hash, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(s.cfg.JWTRefreshTTL)
	newRT, err := s.refresh.Create(ctx, u.ID, hash, old.FamilyID, expiresAt)
	if err != nil {
		return nil, err
	}
	replaced := newRT.ID
	if err := s.refresh.Revoke(ctx, old.ID, &replaced); err != nil {
		return nil, err
	}
	access, jti, accessExp, err := s.issuer.IssueAccess(u.PublicID, u.Role)
	if err != nil {
		return nil, err
	}
	return &AuthTokens{
		AccessToken:      access,
		RefreshToken:     plain,
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: expiresAt,
		AccessJTI:        jti,
	}, nil
}

func userDTO(u *repository.User) *AuthUserDTO {
	return &AuthUserDTO{
		PublicID: u.PublicID.String(),
		Email:    u.Email,
		Role:     u.Role,
	}
}
