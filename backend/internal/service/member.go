package service

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/anhtuanlc/mediahub/internal/auth"
	"github.com/anhtuanlc/mediahub/internal/platform"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrInvalidMemberRole   = errors.New("invalid member role")
	ErrInvalidMemberStatus = errors.New("invalid member status")
	ErrCannotDisableOwner  = errors.New("cannot disable owner")
	ErrNoMemberFields      = errors.New("no fields to update")
)

type MemberService struct {
	users    *repository.UserRepository
	perms    *repository.PermissionRepository
	refresh  *repository.RefreshTokenRepository
	audit    *repository.AuditRepository
	sessions *platform.SessionInvalidation
}

func NewMemberService(
	users *repository.UserRepository,
	perms *repository.PermissionRepository,
	refresh *repository.RefreshTokenRepository,
	audit *repository.AuditRepository,
	sessions *platform.SessionInvalidation,
) *MemberService {
	return &MemberService{users: users, perms: perms, refresh: refresh, audit: audit, sessions: sessions}
}

type CreateMemberInput struct {
	Email    string
	Password string
	Role     string
}

func (s *MemberService) List(ctx context.Context, cursor int64, limit int) ([]repository.User, error) {
	return s.users.ListMembers(ctx, repository.ListMembersParams{Cursor: cursor, Limit: limit})
}

func (s *MemberService) Get(ctx context.Context, publicID uuid.UUID) (*repository.User, []repository.Permission, error) {
	u, err := s.users.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, nil, err
	}
	if u.Role == "owner" {
		return nil, nil, repository.ErrUserNotFound
	}
	perms, err := s.perms.ListByUser(ctx, u.ID)
	if err != nil {
		return nil, nil, err
	}
	return u, perms, nil
}

func (s *MemberService) Create(ctx context.Context, in CreateMemberInput, actorID int64, ip, userAgent string) (*repository.User, error) {
	role := strings.ToLower(strings.TrimSpace(in.Role))
	if role != "manager" && role != "viewer" {
		return nil, ErrInvalidMemberRole
	}
	email := strings.TrimSpace(strings.ToLower(in.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, ErrInvalidEmail
	}
	if err := auth.ValidatePasswordStrength(in.Password); err != nil {
		return nil, err
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	u, err := s.users.CreateMember(ctx, email, hash, role, uuid.New(), actorID)
	if err != nil {
		return nil, err
	}
	targetID := u.ID
	_ = s.audit.Log(ctx, &actorID, "member.created", "user", &targetID, ip, userAgent, map[string]string{"email": email, "role": role})
	return u, nil
}

type UpdateMemberParams struct {
	Role     *string
	Status   *string
	Password *string
}

func (s *MemberService) Update(ctx context.Context, publicID uuid.UUID, p UpdateMemberParams, actorID int64, ip, userAgent string) (*repository.User, error) {
	in := repository.UpdateMemberInput{}

	if p.Role != nil {
		role := strings.ToLower(strings.TrimSpace(*p.Role))
		if role != "manager" && role != "viewer" {
			return nil, ErrInvalidMemberRole
		}
		in.Role = &role
	}
	if p.Status != nil {
		status := strings.ToLower(strings.TrimSpace(*p.Status))
		if status != "active" && status != "disabled" {
			return nil, ErrInvalidMemberStatus
		}
		in.Status = &status
	}
	if p.Password != nil && *p.Password != "" {
		if err := auth.ValidatePasswordStrength(*p.Password); err != nil {
			return nil, err
		}
		hash, err := auth.HashPassword(*p.Password)
		if err != nil {
			return nil, err
		}
		in.PasswordHash = &hash
	}

	if in.Role == nil && in.Status == nil && in.PasswordHash == nil {
		return nil, ErrNoMemberFields
	}

	u, err := s.users.UpdateMember(ctx, publicID, in)
	if err != nil {
		return nil, err
	}
	if in.Role != nil || in.Status != nil {
		_ = s.refresh.RevokeAllForUser(ctx, u.ID)
		if s.sessions != nil {
			_ = s.sessions.InvalidateUser(ctx, u.ID)
		}
	}
	targetID := u.ID
	_ = s.audit.Log(ctx, &actorID, "member.updated", "user", &targetID, ip, userAgent, nil)
	return u, nil
}

func (s *MemberService) Disable(ctx context.Context, publicID uuid.UUID, actorID int64, ip, userAgent string) error {
	u, err := s.users.GetByPublicID(ctx, publicID)
	if err != nil {
		return err
	}
	if u.Role == "owner" {
		return ErrCannotDisableOwner
	}
	status := "disabled"
	_, err = s.users.UpdateMember(ctx, publicID, repository.UpdateMemberInput{Status: &status})
	if err != nil {
		return err
	}
	_ = s.refresh.RevokeAllForUser(ctx, u.ID)
	if s.sessions != nil {
		_ = s.sessions.InvalidateUser(ctx, u.ID)
	}
	targetID := u.ID
	_ = s.audit.Log(ctx, &actorID, "member.disabled", "user", &targetID, ip, userAgent, nil)
	return nil
}

// Restore re-enables a disabled member (status = active).
func (s *MemberService) Restore(ctx context.Context, publicID uuid.UUID, actorID int64, ip, userAgent string) (*repository.User, error) {
	u, err := s.users.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if u.Role == "owner" {
		return nil, ErrCannotDisableOwner
	}
	if u.Status == "active" {
		return u, nil
	}
	active := "active"
	updated, err := s.users.UpdateMember(ctx, publicID, repository.UpdateMemberInput{Status: &active})
	if err != nil {
		return nil, err
	}
	targetID := updated.ID
	_ = s.audit.Log(ctx, &actorID, "member.restored", "user", &targetID, ip, userAgent, nil)
	return updated, nil
}

// Purge permanently deletes a member and cascades related rows (permissions, sessions, …).
func (s *MemberService) Purge(ctx context.Context, publicID uuid.UUID, actorID int64, ip, userAgent string) error {
	u, err := s.users.GetByPublicID(ctx, publicID)
	if err != nil {
		return err
	}
	if u.Role == "owner" {
		return ErrCannotDisableOwner
	}
	_ = s.refresh.RevokeAllForUser(ctx, u.ID)
	if s.sessions != nil {
		_ = s.sessions.InvalidateUser(ctx, u.ID)
	}
	deleted, err := s.users.DeleteMember(ctx, publicID)
	if err != nil {
		return err
	}
	targetID := deleted.ID
	_ = s.audit.Log(ctx, &actorID, "member.deleted", "user", &targetID, ip, userAgent, map[string]string{
		"email": deleted.Email,
	})
	return nil
}
