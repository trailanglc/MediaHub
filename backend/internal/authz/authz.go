package authz

import (
	"context"
	"strings"

	"github.com/anhtuanlc/mediahub/internal/repository"
)

// IsOwnerRole reports whether the role has full workspace access (not subject to member listing filters).
func IsOwnerRole(role string) bool {
	return strings.EqualFold(strings.TrimSpace(role), "owner")
}

type Action string

const (
	ActionRead     Action = "read"
	ActionUpload   Action = "upload"
	ActionUpdate   Action = "update"
	ActionDelete   Action = "delete"
	ActionConvert  Action = "convert"
	ActionShare    Action = "share"
	ActionStream   Action = "stream"
	ActionDownload Action = "download"
	ActionManage   Action = "manage"
)

type Service struct {
	perms *repository.PermissionRepository
}

func NewService(perms *repository.PermissionRepository) *Service {
	return &Service{perms: perms}
}

func (s *Service) HasPermission(ctx context.Context, userID int64, role string, resourceID int64, action Action) (bool, error) {
	if IsOwnerRole(role) {
		return true, nil
	}
	perm := string(action)
	ok, err := s.perms.HasDirect(ctx, userID, resourceID, perm)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	if perm == string(ActionRead) {
		ok, err = s.perms.HasDirect(ctx, userID, resourceID, string(ActionManage))
		if err != nil || ok {
			return ok, err
		}
	}
	return s.perms.HasInherited(ctx, userID, resourceID, perm)
}

// CanListChildren allows opening a folder to see children when the user can read the folder
// or has a direct grant on something inside (shared subfolder without sharing parents).
func (s *Service) CanListChildren(ctx context.Context, userID int64, role string, folderID int64) (bool, error) {
	if IsOwnerRole(role) {
		return true, nil
	}
	ok, err := s.HasPermission(ctx, userID, role, folderID, ActionRead)
	if err != nil || ok {
		return ok, err
	}
	ok, err = s.perms.HasDirectAny(ctx, userID, folderID)
	if err != nil || ok {
		return ok, err
	}
	return s.perms.HasGrantedDescendantAny(ctx, userID, folderID)
}

// CanTraverseFolder is true when the user may open a folder to reach a shared subtree below it.
func (s *Service) CanTraverseFolder(ctx context.Context, userID int64, role string, folderID int64) (bool, error) {
	return s.CanListChildren(ctx, userID, role, folderID)
}

// VisibleForListing returns which object IDs may appear in a folder listing (members only).
func (s *Service) VisibleForListing(
	ctx context.Context,
	userID int64,
	role string,
	resourceIDs []int64,
) (map[int64]struct{}, error) {
	out := make(map[int64]struct{}, len(resourceIDs))
	if IsOwnerRole(role) {
		for _, id := range resourceIDs {
			out[id] = struct{}{}
		}
		return out, nil
	}
	return s.perms.ListingVisibleAmong(ctx, userID, resourceIDs)
}

// CapabilitiesMap resolves effective permissions for many resources in two DB round-trips.
func (s *Service) CapabilitiesMap(
	ctx context.Context,
	userID int64,
	role string,
	resourceIDs []int64,
) (map[int64]Capabilities, error) {
	out := make(map[int64]Capabilities, len(resourceIDs))
	if IsOwnerRole(role) {
		full := Capabilities{
			Read: true, Upload: true, Update: true, Delete: true, Manage: true, Share: true, Download: true,
			Convert: true, Stream: true,
		}
		for _, id := range resourceIDs {
			out[id] = full
		}
		return out, nil
	}
	perms, err := s.perms.EffectivePermissionsForResources(ctx, userID, resourceIDs)
	if err != nil {
		return nil, err
	}
	for _, id := range resourceIDs {
		out[id] = capabilitiesFromPermSet(perms[id])
	}
	return out, nil
}

// Capabilities is the authz view of actions on a resource (maps to service.ObjectCapabilities).
type Capabilities struct {
	Read     bool
	Upload   bool
	Update   bool
	Delete   bool
	Manage   bool
	Share    bool
	Download bool
	Convert  bool
	Stream   bool
}

func capabilitiesFromPermSet(perms map[string]struct{}) Capabilities {
	if perms == nil {
		return Capabilities{}
	}
	has := func(p string) bool {
		_, ok := perms[p]
		return ok
	}
	manage := has(string(ActionManage))
	share := has(string(ActionShare)) || manage
	return Capabilities{
		Read:     has(string(ActionRead)),
		Upload:   has(string(ActionUpload)),
		Update:   has(string(ActionUpdate)),
		Delete:   has(string(ActionDelete)),
		Manage:   manage,
		Share:    share,
		Download: has(string(ActionDownload)),
		Convert:  has(string(ActionConvert)),
		Stream:   has(string(ActionStream)),
	}
}
