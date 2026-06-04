package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/authz"
	"github.com/anhtuanlc/mediahub/internal/mediautil"
	"github.com/anhtuanlc/mediahub/internal/platform/webhook"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/google/uuid"
)

var (
	ErrMediaAccessDenied   = errors.New("access denied")
	ErrMediaParentRequired = errors.New("parent folder required")
	ErrMediaInvalidName    = errors.New("invalid name")
	ErrMediaCannotMoveRoot  = errors.New("cannot move root folder")
	ErrMediaInvalidParent   = errors.New("invalid parent")
	ErrMediaParentDeleted   = errors.New("parent folder is deleted")
	ErrMediaNotDeleted      = errors.New("object is not in trash")
	ErrMediaSearchQueryReq  = errors.New("search query required")
	ErrMediaBulkRenameLimit = errors.New("bulk rename exceeds maximum objects")
	ErrMediaBulkRenameMode  = errors.New("invalid bulk rename mode")
	ErrMediaFolderNotEmpty  = errors.New("folder is not empty")
)

const MaxBulkRenameObjects = 20

type ObjectCapabilities struct {
	Read     bool `json:"read"`
	Upload   bool `json:"upload"`
	Update   bool `json:"update"`
	Delete   bool `json:"delete"`
	Manage   bool `json:"manage"`
	Download bool `json:"download"`
}

type MediaObjectDTO struct {
	PublicID       string              `json:"public_id"`
	ParentPublicID *string             `json:"parent_public_id"`
	Type           string              `json:"type"`
	Name           string              `json:"name"`
	MimeType       *string             `json:"mime_type"`
	SizeBytes      int64               `json:"size_bytes"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
	Capabilities   ObjectCapabilities  `json:"capabilities"`
	PreviewURL     *string             `json:"preview_url,omitempty"`
	DownloadURL    *string             `json:"download_url,omitempty"`
	Breadcrumbs    []repository.BreadcrumbItem `json:"breadcrumbs,omitempty"`
}

// ObjectAccessURLs holds presigned URLs for grid/preview (not included in list).
type ObjectAccessURLs struct {
	PreviewURL   *string `json:"preview_url,omitempty"`
	ThumbnailURL *string `json:"thumbnail_url,omitempty"`
	DownloadURL  *string `json:"download_url,omitempty"`
}

const maxBatchAccessURLs = 24

type MediaObjectService struct {
	objects        *repository.MediaObjectRepository
	settings       *SettingsService
	authz          *authz.Service
	audit          *repository.AuditRepository
	store          storage.ObjectStorage
	storageCleanup *StorageCleanupService
	thumbnails     *ThumbnailService
	webhooks       *webhook.Dispatcher
}

func NewMediaObjectService(
	objects *repository.MediaObjectRepository,
	settings *SettingsService,
	authzSvc *authz.Service,
	audit *repository.AuditRepository,
	store storage.ObjectStorage,
	storageCleanup *StorageCleanupService,
	thumbnails *ThumbnailService,
) *MediaObjectService {
	return &MediaObjectService{
		objects:        objects,
		settings:       settings,
		authz:          authzSvc,
		audit:          audit,
		store:          store,
		storageCleanup: storageCleanup,
		thumbnails:     thumbnails,
	}
}

func (s *MediaObjectService) SetWebhooks(d *webhook.Dispatcher) {
	s.webhooks = d
}

type ListObjectsInput struct {
	ParentPublicID *uuid.UUID
	Types          []string
	Query          string
	Cursor         int64
	Limit          int
}

func (s *MediaObjectService) resolveParent(ctx context.Context, parentPID *uuid.UUID) (*repository.MediaObject, error) {
	if parentPID != nil {
		parent, err := s.objects.GetByPublicID(ctx, *parentPID)
		if err != nil {
			return nil, err
		}
		if parent.Type != "folder" {
			return nil, fmt.Errorf("%w: parent must be a folder", ErrMediaInvalidParent)
		}
		return parent, nil
	}
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	rootID, err := uuid.Parse(settings.Editable.Media.DefaultRootFolderPublicID)
	if err != nil {
		return nil, err
	}
	return s.objects.GetByPublicID(ctx, rootID)
}

func (s *MediaObjectService) checkPerm(ctx context.Context, userID int64, role string, resourceID int64, action authz.Action) error {
	if authz.IsOwnerRole(role) {
		return nil
	}
	ok, err := s.authz.HasPermission(ctx, userID, role, resourceID, action)
	if err != nil {
		return err
	}
	if !ok {
		return ErrMediaAccessDenied
	}
	return nil
}

func (s *MediaObjectService) checkFolderListAccess(ctx context.Context, userID int64, role string, folderID int64) error {
	if authz.IsOwnerRole(role) {
		return nil
	}
	ok, err := s.authz.CanListChildren(ctx, userID, role, folderID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrMediaAccessDenied
	}
	return nil
}

func (s *MediaObjectService) checkFolderGetAccess(
	ctx context.Context,
	userID int64,
	role string,
	m *repository.MediaObject,
) error {
	if m.Type != "folder" {
		return s.checkPerm(ctx, userID, role, m.ID, authz.ActionRead)
	}
	return s.checkFolderListAccess(ctx, userID, role, m.ID)
}

func (s *MediaObjectService) capabilities(ctx context.Context, userID int64, role string, resourceID int64) ObjectCapabilities {
	if authz.IsOwnerRole(role) {
		return ObjectCapabilities{Read: true, Upload: true, Update: true, Delete: true, Manage: true, Download: true}
	}
	cap := ObjectCapabilities{}
	checks := []struct {
		action authz.Action
		dest   *bool
	}{
		{authz.ActionRead, &cap.Read},
		{authz.ActionUpload, &cap.Upload},
		{authz.ActionUpdate, &cap.Update},
		{authz.ActionDelete, &cap.Delete},
		{authz.ActionManage, &cap.Manage},
		{authz.ActionDownload, &cap.Download},
	}
	for _, c := range checks {
		ok, _ := s.authz.HasPermission(ctx, userID, role, resourceID, c.action)
		*c.dest = ok
	}
	return cap
}

func toDTO(m *repository.MediaObject, caps ObjectCapabilities) MediaObjectDTO {
	dto := MediaObjectDTO{
		PublicID:     m.PublicID.String(),
		Type:         m.Type,
		Name:         m.Name,
		MimeType:     m.MimeType,
		SizeBytes:    m.SizeBytes,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
		Capabilities: caps,
	}
	if m.ParentPublic != nil {
		s := m.ParentPublic.String()
		dto.ParentPublicID = &s
	}
	return dto
}

func (s *MediaObjectService) presignURLs(ctx context.Context, m *repository.MediaObject, caps ObjectCapabilities) (*string, *string) {
	if m.StorageKey == nil || *m.StorageKey == "" {
		return nil, nil
	}
	var preview, download *string
	ttl := 15 * time.Minute
	if caps.Read && (m.Type == "image" || m.Type == "file") {
		if u, err := s.store.PresignGetObject(ctx, *m.StorageKey, ttl); err == nil {
			preview = &u
		}
	}
	if caps.Download || (caps.Read && m.Type != "folder") {
		if u, err := s.store.PresignGetObject(ctx, *m.StorageKey, ttl); err == nil {
			download = &u
		}
	}
	if m.Type == "image" && preview == nil && download != nil {
		preview = download
	}
	return preview, download
}

func (s *MediaObjectService) List(ctx context.Context, userID int64, role string, in ListObjectsInput) ([]MediaObjectDTO, *int64, error) {
	parent, err := s.resolveParent(ctx, in.ParentPublicID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.checkFolderListAccess(ctx, userID, role, parent.ID); err != nil {
		return nil, nil, err
	}

	limit := in.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	filter := repository.ObjectListFilter{
		Types:  in.Types,
		Query:  in.Query,
		Cursor: in.Cursor,
		Limit:  limit,
	}
	var items []repository.MediaObject
	if q := strings.TrimSpace(in.Query); q != "" {
		items, err = s.objects.SearchActiveInSubtree(ctx, parent.ID, filter)
	} else {
		items, err = s.objects.ListChildren(ctx, parent.ID, filter)
	}
	if err != nil {
		return nil, nil, err
	}

	var next *int64
	if len(items) > limit {
		last := items[limit-1]
		next = &last.ID
		items = items[:limit]
	}

	if strings.TrimSpace(in.Query) != "" {
		return s.filterListDTOWithBreadcrumbs(ctx, userID, role, items, next)
	}

	return s.filterListDTO(ctx, userID, role, items, next)
}

func objectCapabilitiesFromAuthz(c authz.Capabilities) ObjectCapabilities {
	return ObjectCapabilities{
		Read:     c.Read,
		Upload:   c.Upload,
		Update:   c.Update,
		Delete:   c.Delete,
		Manage:   c.Manage,
		Download: c.Download,
	}
}

// BatchAccessURLs returns presigned URLs for up to maxBatchAccessURLs objects (read permission required).
func (s *MediaObjectService) BatchAccessURLs(
	ctx context.Context,
	userID int64,
	role string,
	publicIDs []uuid.UUID,
) (map[string]ObjectAccessURLs, error) {
	if len(publicIDs) == 0 {
		return map[string]ObjectAccessURLs{}, nil
	}
	if len(publicIDs) > maxBatchAccessURLs {
		return nil, fmt.Errorf("too many ids: max %d", maxBatchAccessURLs)
	}

	objects, err := s.objects.ListByPublicIDs(ctx, publicIDs)
	if err != nil {
		return nil, err
	}

	ids := make([]int64, len(objects))
	byPID := make(map[string]*repository.MediaObject, len(objects))
	for i := range objects {
		ids[i] = objects[i].ID
		pid := objects[i].PublicID.String()
		byPID[pid] = &objects[i]
	}

	capsMap, err := s.authz.CapabilitiesMap(ctx, userID, role, ids)
	if err != nil {
		return nil, err
	}

	out := make(map[string]ObjectAccessURLs, len(objects))
	ttl := 15 * time.Minute
	for pid, m := range byPID {
		caps := objectCapabilitiesFromAuthz(capsMap[m.ID])
		if !caps.Read {
			continue
		}
		urls := ObjectAccessURLs{}
		if m.ThumbnailKey != nil && *m.ThumbnailKey != "" {
			if u, err := s.store.PresignGetObject(ctx, *m.ThumbnailKey, ttl); err == nil {
				urls.ThumbnailURL = &u
			}
		}
		preview, download := s.presignURLs(ctx, m, caps)
		if urls.ThumbnailURL != nil && m.Type == "image" {
			urls.PreviewURL = urls.ThumbnailURL
		} else {
			urls.PreviewURL = preview
		}
		urls.DownloadURL = download
		out[pid] = urls
	}
	return out, nil
}

func (s *MediaObjectService) Get(ctx context.Context, userID int64, role string, publicID uuid.UUID) (*MediaObjectDTO, error) {
	m, err := s.objects.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if err := s.checkFolderGetAccess(ctx, userID, role, m); err != nil {
		return nil, err
	}
	caps := s.capabilities(ctx, userID, role, m.ID)
	dto := toDTO(m, caps)
	preview, download := s.presignURLs(ctx, m, caps)
	dto.PreviewURL = preview
	dto.DownloadURL = download
	crumbs, err := s.objects.GetAncestors(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	dto.Breadcrumbs = crumbs
	return &dto, nil
}

type CreateFolderInput struct {
	ParentPublicID uuid.UUID
	Name           string
}

func (s *MediaObjectService) CreateFolder(ctx context.Context, userID int64, role string, in CreateFolderInput, ip, ua string) (*MediaObjectDTO, error) {
	parent, err := s.objects.GetByPublicID(ctx, in.ParentPublicID)
	if err != nil {
		return nil, err
	}
	if parent.Type != "folder" {
		return nil, ErrMediaInvalidParent
	}
	if err := s.checkPerm(ctx, userID, role, parent.ID, authz.ActionUpload); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > 255 {
		return nil, ErrMediaInvalidName
	}

	pid := uuid.New()
	m, err := s.objects.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:  pid,
		ParentID:  &parent.ID,
		Type:      "folder",
		Name:      name,
		SizeBytes: 0,
		CreatedBy: userID,
	})
	if err != nil {
		return nil, err
	}
	tid := m.ID
	_ = s.audit.Log(ctx, &userID, "folder.create", "folder", &tid, ip, ua, map[string]string{"name": name})
	caps := s.capabilities(ctx, userID, role, m.ID)
	dto := toDTO(m, caps)
	return &dto, nil
}

type PatchObjectInput struct {
	Name           *string
	ParentPublicID *uuid.UUID
}

func (s *MediaObjectService) Patch(ctx context.Context, userID int64, role string, publicID uuid.UUID, in PatchObjectInput, ip, ua string) (*MediaObjectDTO, error) {
	m, err := s.objects.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if err := s.checkPerm(ctx, userID, role, m.ID, authz.ActionUpdate); err != nil {
		return nil, err
	}

	rootPID, _ := uuid.Parse(repository.DefaultRootFolderPublicID)
	if m.PublicID == rootPID {
		if in.ParentPublicID != nil {
			return nil, ErrMediaCannotMoveRoot
		}
	}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" || len(name) > 255 {
			return nil, ErrMediaInvalidName
		}
		if err := s.objects.UpdateName(ctx, m.ID, userID, name); err != nil {
			return nil, err
		}
	}
	if in.ParentPublicID != nil {
		newParent, err := s.objects.GetByPublicID(ctx, *in.ParentPublicID)
		if err != nil {
			return nil, err
		}
		if newParent.Type != "folder" {
			return nil, ErrMediaInvalidParent
		}
		if err := s.checkPerm(ctx, userID, role, newParent.ID, authz.ActionUpload); err != nil {
			return nil, err
		}
		if err := s.objects.Move(ctx, m.ID, &newParent.ID, userID); err != nil {
			return nil, err
		}
	}

	updated, err := s.objects.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	tid := updated.ID
	_ = s.audit.Log(ctx, &userID, "object.update", updated.Type, &tid, ip, ua, nil)
	caps := s.capabilities(ctx, userID, role, updated.ID)
	dto := toDTO(updated, caps)
	return &dto, nil
}

type BulkRenameInput struct {
	ObjectPublicIDs []uuid.UUID
	Mode            string
	Value           string
}

type BulkRenameResult struct {
	RenamedCount int              `json:"renamed_count"`
	Items        []MediaObjectDTO `json:"items"`
}

func (s *MediaObjectService) BulkRename(ctx context.Context, userID int64, role string, in BulkRenameInput, ip, ua string) (*BulkRenameResult, error) {
	if len(in.ObjectPublicIDs) == 0 {
		return &BulkRenameResult{Items: []MediaObjectDTO{}}, nil
	}
	if len(in.ObjectPublicIDs) > MaxBulkRenameObjects {
		return nil, ErrMediaBulkRenameLimit
	}
	mode := strings.TrimSpace(in.Mode)
	if mode != "prefix" && mode != "suffix" {
		return nil, ErrMediaBulkRenameMode
	}
	value := strings.TrimSpace(in.Value)
	if value == "" {
		return nil, ErrMediaInvalidName
	}

	type planned struct {
		obj     *repository.MediaObject
		newName string
	}
	plannedRenames := make([]planned, 0, len(in.ObjectPublicIDs))

	for _, pid := range in.ObjectPublicIDs {
		m, err := s.objects.GetByPublicID(ctx, pid)
		if err != nil {
			return nil, err
		}
		rootPID, _ := uuid.Parse(repository.DefaultRootFolderPublicID)
		if m.PublicID == rootPID {
			return nil, ErrMediaCannotMoveRoot
		}
		if err := s.checkPerm(ctx, userID, role, m.ID, authz.ActionUpdate); err != nil {
			return nil, err
		}
		newName := value + m.Name
		if mode == "suffix" {
			newName = m.Name + value
		}
		newName = strings.TrimSpace(newName)
		if newName == "" || len(newName) > 255 {
			return nil, ErrMediaInvalidName
		}
		plannedRenames = append(plannedRenames, planned{obj: m, newName: newName})
	}

	tx, err := s.objects.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, p := range plannedRenames {
		if err := s.objects.UpdateNameTx(ctx, tx, p.obj.ID, userID, p.newName); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	out := make([]MediaObjectDTO, 0, len(plannedRenames))
	for _, p := range plannedRenames {
		updated, err := s.objects.GetByPublicID(ctx, p.obj.PublicID)
		if err != nil {
			return nil, err
		}
		tid := updated.ID
		_ = s.audit.Log(ctx, &userID, "object.update", updated.Type, &tid, ip, ua, map[string]string{
			"bulk_rename": "1",
			"mode":        mode,
		})
		caps := s.capabilities(ctx, userID, role, updated.ID)
		out = append(out, toDTO(updated, caps))
	}
	_ = s.audit.Log(ctx, &userID, "object.bulk_rename", "batch", nil, ip, ua, map[string]string{
		"count": fmt.Sprintf("%d", len(out)),
		"mode":  mode,
	})
	return &BulkRenameResult{RenamedCount: len(out), Items: out}, nil
}

type DeleteObjectResult struct {
	DeletedCount int64 `json:"deleted_count"`
}

func (s *MediaObjectService) ListTrash(ctx context.Context, userID int64, role string, in ListObjectsInput) ([]MediaObjectDTO, *int64, error) {
	limit := in.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	items, err := s.objects.ListDeletedRoots(ctx, repository.ObjectListFilter{
		Types:  in.Types,
		Query:  in.Query,
		Cursor: in.Cursor,
		Limit:  limit,
	})
	if err != nil {
		return nil, nil, err
	}
	var next *int64
	if len(items) > limit {
		last := items[limit-1]
		next = &last.ID
		items = items[:limit]
	}
	return s.filterListDTO(ctx, userID, role, items, next)
}

func (s *MediaObjectService) Search(ctx context.Context, userID int64, role string, in ListObjectsInput) ([]MediaObjectDTO, *int64, error) {
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return nil, nil, ErrMediaSearchQueryReq
	}
	limit := in.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	items, err := s.objects.SearchActiveByName(ctx, repository.ObjectListFilter{
		Types:  in.Types,
		Query:  q,
		Cursor: in.Cursor,
		Limit:  limit,
	})
	if err != nil {
		return nil, nil, err
	}
	var next *int64
	if len(items) > limit {
		last := items[limit-1]
		next = &last.ID
		items = items[:limit]
	}
	return s.filterListDTOWithBreadcrumbs(ctx, userID, role, items, next)
}

func (s *MediaObjectService) filterListDTOWithBreadcrumbs(
	ctx context.Context,
	userID int64,
	role string,
	items []repository.MediaObject,
	next *int64,
) ([]MediaObjectDTO, *int64, error) {
	if len(items) == 0 {
		return []MediaObjectDTO{}, next, nil
	}
	if authz.IsOwnerRole(role) {
		return s.ownerListDTOWithBreadcrumbs(ctx, userID, role, items, next)
	}
	ids := make([]int64, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	visible, capsMap, crumbsMap, err := s.listingContext(ctx, userID, role, ids)
	if err != nil {
		return nil, nil, err
	}
	out := make([]MediaObjectDTO, 0, len(items))
	for i := range items {
		if _, ok := visible[items[i].ID]; !ok {
			continue
		}
		caps := objectCapabilitiesFromAuthz(capsMap[items[i].ID])
		dto := toDTO(&items[i], caps)
		dto.Breadcrumbs = crumbsMap[items[i].ID]
		out = append(out, dto)
	}
	return out, next, nil
}

func (s *MediaObjectService) filterListDTO(
	ctx context.Context,
	userID int64,
	role string,
	items []repository.MediaObject,
	next *int64,
) ([]MediaObjectDTO, *int64, error) {
	if len(items) == 0 {
		return []MediaObjectDTO{}, next, nil
	}
	if authz.IsOwnerRole(role) {
		return s.ownerListDTO(ctx, userID, role, items, next)
	}
	ids := make([]int64, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	visible, capsMap, _, err := s.listingContext(ctx, userID, role, ids)
	if err != nil {
		return nil, nil, err
	}
	out := make([]MediaObjectDTO, 0, len(items))
	for i := range items {
		if _, ok := visible[items[i].ID]; !ok {
			continue
		}
		caps := objectCapabilitiesFromAuthz(capsMap[items[i].ID])
		out = append(out, toDTO(&items[i], caps))
	}
	return out, next, nil
}

func (s *MediaObjectService) ownerListDTO(
	ctx context.Context,
	userID int64,
	role string,
	items []repository.MediaObject,
	next *int64,
) ([]MediaObjectDTO, *int64, error) {
	ids := make([]int64, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	capsMap, err := s.authz.CapabilitiesMap(ctx, userID, role, ids)
	if err != nil {
		return nil, nil, err
	}
	out := make([]MediaObjectDTO, 0, len(items))
	for i := range items {
		out = append(out, toDTO(&items[i], objectCapabilitiesFromAuthz(capsMap[items[i].ID])))
	}
	return out, next, nil
}

func (s *MediaObjectService) ownerListDTOWithBreadcrumbs(
	ctx context.Context,
	userID int64,
	role string,
	items []repository.MediaObject,
	next *int64,
) ([]MediaObjectDTO, *int64, error) {
	ids := make([]int64, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	capsMap, err := s.authz.CapabilitiesMap(ctx, userID, role, ids)
	if err != nil {
		return nil, nil, err
	}
	crumbsMap, err := s.objects.GetAncestorsBatch(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	out := make([]MediaObjectDTO, 0, len(items))
	for i := range items {
		dto := toDTO(&items[i], objectCapabilitiesFromAuthz(capsMap[items[i].ID]))
		dto.Breadcrumbs = crumbsMap[items[i].ID]
		out = append(out, dto)
	}
	return out, next, nil
}

func (s *MediaObjectService) listingContext(
	ctx context.Context,
	userID int64,
	role string,
	ids []int64,
) (map[int64]struct{}, map[int64]authz.Capabilities, map[int64][]repository.BreadcrumbItem, error) {
	visible, err := s.authz.VisibleForListing(ctx, userID, role, ids)
	if err != nil {
		return nil, nil, nil, err
	}
	capsMap, err := s.authz.CapabilitiesMap(ctx, userID, role, ids)
	if err != nil {
		return nil, nil, nil, err
	}
	crumbsMap, err := s.objects.GetAncestorsBatch(ctx, ids)
	if err != nil {
		return nil, nil, nil, err
	}
	return visible, capsMap, crumbsMap, nil
}

type RestoreObjectResult struct {
	RestoredCount int64          `json:"restored_count"`
	Object        MediaObjectDTO `json:"object"`
}

type PurgeObjectResult struct {
	PurgedCount int64 `json:"purged_count"`
}

type EmptyTrashResult struct {
	PurgedCount  int64 `json:"purged_count"`
	RootsPurged  int   `json:"roots_purged"`
	RootsSkipped int   `json:"roots_skipped"`
}

func (s *MediaObjectService) Restore(ctx context.Context, userID int64, role string, publicID uuid.UUID, ip, ua string) (*RestoreObjectResult, error) {
	m, err := s.objects.GetByPublicIDAny(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if m.DeletedAt == nil {
		return nil, ErrMediaNotDeleted
	}
	if err := s.checkPerm(ctx, userID, role, m.ID, authz.ActionUpdate); err != nil {
		return nil, err
	}
	if m.ParentID != nil {
		parent, err := s.objects.GetByIDAny(ctx, *m.ParentID)
		if err != nil {
			return nil, ErrMediaInvalidParent
		}
		if parent.DeletedAt != nil {
			return nil, ErrMediaParentDeleted
		}
	}
	restoredCount, err := s.objects.RestoreSubtree(ctx, m.ID, userID)
	if err != nil {
		return nil, err
	}
	if restoredCount == 0 {
		return nil, repository.ErrMediaObjectNotFound
	}
	restored, err := s.objects.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	tid := restored.ID
	meta := map[string]string{
		"restored_count": fmt.Sprintf("%d", restoredCount),
	}
	_ = s.audit.Log(ctx, &userID, "object.restore", restored.Type, &tid, ip, ua, meta)
	caps := s.capabilities(ctx, userID, role, restored.ID)
	dto := toDTO(restored, caps)
	return &RestoreObjectResult{RestoredCount: restoredCount, Object: dto}, nil
}

func (s *MediaObjectService) Purge(ctx context.Context, userID int64, role string, publicID uuid.UUID, ip, ua string) (*PurgeObjectResult, error) {
	m, err := s.objects.GetByPublicIDAny(ctx, publicID)
	if err != nil {
		return nil, err
	}
	rootPID, _ := uuid.Parse(repository.DefaultRootFolderPublicID)
	if m.PublicID == rootPID {
		return nil, ErrMediaCannotMoveRoot
	}
	if m.DeletedAt == nil {
		return nil, ErrMediaNotDeleted
	}
	if err := s.checkPerm(ctx, userID, role, m.ID, authz.ActionDelete); err != nil {
		return nil, err
	}

	purgedCount, err := s.purgeSubtree(ctx, m)
	if err != nil {
		return nil, err
	}

	tid := m.ID
	meta := map[string]string{
		"purged_count": fmt.Sprintf("%d", purgedCount),
	}
	_ = s.audit.Log(ctx, &userID, "object.purge", m.Type, &tid, ip, ua, meta)
	return &PurgeObjectResult{PurgedCount: purgedCount}, nil
}

func (s *MediaObjectService) purgeSubtree(ctx context.Context, m *repository.MediaObject) (int64, error) {
	if err := s.scheduleDeletedSubtreeDeletions(ctx, m.ID); err != nil {
		return 0, err
	}

	purgedCount, err := s.objects.HardDeleteSubtree(ctx, m.ID)
	if err != nil {
		return 0, err
	}
	if purgedCount == 0 {
		return 0, repository.ErrMediaObjectNotFound
	}
	return purgedCount, nil
}

// PurgeExpiredTrash permanently removes soft-deleted objects older than maintenance.trash_retention_days.
func (s *MediaObjectService) PurgeExpiredTrash(ctx context.Context) (int, error) {
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return 0, err
	}
	days := settings.Editable.Maintenance.TrashRetentionDays
	if days <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	total := 0
	for {
		roots, err := s.objects.ListExpiredDeletedRoots(ctx, cutoff, 50)
		if err != nil {
			return total, err
		}
		if len(roots) == 0 {
			break
		}
		for i := range roots {
			n, err := s.purgeSubtree(ctx, &roots[i])
			if err != nil {
				if errors.Is(err, repository.ErrMediaObjectNotFound) {
					continue
				}
				return total, err
			}
			total += int(n)
		}
		if len(roots) < 50 {
			break
		}
	}
	return total, nil
}

// EmptyTrash permanently removes all soft-deleted roots the user may delete.
func (s *MediaObjectService) EmptyTrash(ctx context.Context, userID int64, role string, ip, ua string) (*EmptyTrashResult, error) {
	res := &EmptyTrashResult{}
	for {
		roots, err := s.objects.ListDeletedRoots(ctx, repository.ObjectListFilter{Limit: 50})
		if err != nil {
			return nil, err
		}
		if len(roots) == 0 {
			break
		}
		progress := false
		for i := range roots {
			m := &roots[i]
			if err := s.checkPerm(ctx, userID, role, m.ID, authz.ActionDelete); err != nil {
				res.RootsSkipped++
				continue
			}
			n, err := s.purgeSubtree(ctx, m)
			if err != nil {
				if errors.Is(err, repository.ErrMediaObjectNotFound) {
					continue
				}
				return res, err
			}
			res.PurgedCount += n
			res.RootsPurged++
			progress = true
			tid := m.ID
			meta := map[string]string{"purged_count": fmt.Sprintf("%d", n)}
			_ = s.audit.Log(ctx, &userID, "object.purge", m.Type, &tid, ip, ua, meta)
		}
		if !progress {
			break
		}
		if len(roots) < 50 {
			break
		}
	}
	if res.RootsPurged > 0 {
		meta := map[string]string{
			"purged_count":  fmt.Sprintf("%d", res.PurgedCount),
			"roots_purged":  fmt.Sprintf("%d", res.RootsPurged),
			"roots_skipped": fmt.Sprintf("%d", res.RootsSkipped),
		}
		_ = s.audit.Log(ctx, &userID, "trash.empty", "trash", nil, ip, ua, meta)
	}
	return res, nil
}

func (s *MediaObjectService) Delete(ctx context.Context, userID int64, role string, publicID uuid.UUID, ip, ua string) (*DeleteObjectResult, error) {
	m, err := s.objects.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	rootPID, _ := uuid.Parse(repository.DefaultRootFolderPublicID)
	if m.PublicID == rootPID {
		return nil, ErrMediaCannotMoveRoot
	}
	if err := s.checkPerm(ctx, userID, role, m.ID, authz.ActionDelete); err != nil {
		return nil, err
	}

	if m.Type == "folder" && s.settings.DeleteEmptyFoldersOnly(ctx) {
		n, err := s.objects.CountActiveChildren(ctx, m.ID)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			return nil, ErrMediaFolderNotEmpty
		}
	}

	deletedCount, err := s.objects.SoftDeleteSubtree(ctx, m.ID, userID)
	if err != nil {
		return nil, err
	}
	if deletedCount == 0 {
		return nil, repository.ErrMediaObjectNotFound
	}

	if s.storageCleanup != nil {
		if err := s.scheduleDeletedSubtreeDeletions(ctx, m.ID); err != nil {
			return nil, fmt.Errorf("schedule storage deletion: %w", err)
		}
	}

	tid := m.ID
	meta := map[string]string{
		"deleted_count": fmt.Sprintf("%d", deletedCount),
	}
	_ = s.audit.Log(ctx, &userID, "object.delete", m.Type, &tid, ip, ua, meta)
	if s.webhooks != nil {
		s.webhooks.Emit(ctx, webhook.EventMediaDeleted, map[string]any{
			"public_id":     m.PublicID.String(),
			"type":          m.Type,
			"name":          m.Name,
			"deleted_count": deletedCount,
		})
	}
	return &DeleteObjectResult{DeletedCount: deletedCount}, nil
}

const subtreeDeletionBatch = 200

func (s *MediaObjectService) scheduleDeletedSubtreeDeletions(ctx context.Context, ancestorID int64) error {
	if s.storageCleanup == nil {
		return nil
	}
	afterID := int64(0)
	for {
		batch, err := s.objects.ListDeletedInSubtreePage(ctx, ancestorID, afterID, subtreeDeletionBatch)
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return repository.ErrMediaObjectNotFound
		}
		ptrs := make([]*repository.MediaObject, len(batch))
		for i := range batch {
			ptrs[i] = &batch[i]
		}
		if err := s.storageCleanup.ScheduleDeletions(ctx, ptrs); err != nil {
			return err
		}
		afterID = batch[len(batch)-1].ID
		if len(batch) < subtreeDeletionBatch {
			return nil
		}
	}
}

// Exported for upload service
func (s *MediaObjectService) Objects() *repository.MediaObjectRepository {
	return s.objects
}

func (s *MediaObjectService) Store() storage.ObjectStorage {
	return s.store
}

func (s *MediaObjectService) Settings() *SettingsService {
	return s.settings
}

func (s *MediaObjectService) Authz() *authz.Service {
	return s.authz
}

func (s *MediaObjectService) Audit() *repository.AuditRepository {
	return s.audit
}

func (s *MediaObjectService) CheckUploadPerm(ctx context.Context, userID int64, role string, parentID int64) error {
	return s.checkPerm(ctx, userID, role, parentID, authz.ActionUpload)
}

func (s *MediaObjectService) BuildCommittedObject(
	ctx context.Context,
	userID int64,
	parentID int64,
	originalName string,
	mimeType *string,
	size int64,
	storageKey string,
	checksum *string,
) (*repository.MediaObject, error) {
	name := mediautil.SanitizeName(originalName)
	objType := mediautil.DetectObjectType(ptrStr(mimeType), originalName)
	pid := uuid.New()
	m, err := s.objects.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:     pid,
		ParentID:     &parentID,
		Type:         objType,
		Name:         name,
		OriginalName: &originalName,
		MimeType:     mimeType,
		SizeBytes:    size,
		StorageKey:   &storageKey,
		Checksum:     checksum,
		CreatedBy:    userID,
	})
	if err != nil {
		return nil, err
	}
	if objType == "video" {
		if err := s.objects.CreateVideoAsset(ctx, m.ID); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
