package service

import (
	"context"
	"errors"

	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

var ErrInvalidPermission = errors.New("invalid permission")

var validPermissions = map[string]bool{
	"read": true, "upload": true, "update": true, "delete": true,
	"convert": true, "share": true, "stream": true, "download": true, "manage": true,
}

type PermissionService struct {
	perms   *repository.PermissionRepository
	users   *repository.UserRepository
	objects *repository.MediaObjectRepository
	audit   *repository.AuditRepository
}

func NewPermissionService(
	perms *repository.PermissionRepository,
	users *repository.UserRepository,
	objects *repository.MediaObjectRepository,
	audit *repository.AuditRepository,
) *PermissionService {
	return &PermissionService{perms: perms, users: users, objects: objects, audit: audit}
}

type GrantPermissionInput struct {
	UserPublicID     uuid.UUID
	ResourcePublicID uuid.UUID
	Permission       string
}

func (s *PermissionService) ListMine(ctx context.Context, userID int64) ([]repository.Permission, error) {
	return s.perms.ListByUser(ctx, userID)
}

func (s *PermissionService) ListByResource(ctx context.Context, resourcePublicID uuid.UUID) ([]repository.Permission, error) {
	obj, err := s.objects.GetByPublicID(ctx, resourcePublicID)
	if err != nil {
		return nil, err
	}
	return s.perms.ListByResource(ctx, obj.ID)
}

func (s *PermissionService) Grant(ctx context.Context, in GrantPermissionInput, grantedBy int64, ip, userAgent string) (*repository.Permission, error) {
	if !validPermissions[in.Permission] {
		return nil, ErrInvalidPermission
	}
	user, err := s.users.GetByPublicID(ctx, in.UserPublicID)
	if err != nil {
		return nil, err
	}
	if user.Role == "owner" {
		return nil, errors.New("owner has full access")
	}
	obj, err := s.objects.GetByPublicID(ctx, in.ResourcePublicID)
	if err != nil {
		return nil, err
	}
	p, err := s.perms.Grant(ctx, user.ID, obj.ID, grantedBy, obj.Type, in.Permission)
	if err != nil {
		return nil, err
	}
	targetID := p.ID
	_ = s.audit.Log(ctx, &grantedBy, "permission.granted", "permission", &targetID, ip, userAgent, map[string]string{
		"permission": in.Permission,
	})
	return p, nil
}

func (s *PermissionService) Revoke(ctx context.Context, id int64, actorID int64, ip, userAgent string) error {
	if err := s.perms.Revoke(ctx, id); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, &actorID, "permission.revoked", "permission", &id, ip, userAgent, nil)
	return nil
}
