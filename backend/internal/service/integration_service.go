package service

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/anhtuanlc/mediahub/internal/integration"
	integrationquota "github.com/anhtuanlc/mediahub/internal/platform/integrationquota"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrIntegrationAccessDenied  = errors.New("integration access denied")
	ErrIntegrationInvalidParent = errors.New("parent not in api key namespace")
	ErrIntegrationQuotaExceeded = errors.New("integration quota exceeded")
)

// IntegrationService exposes media operations scoped to an API key namespace.
type IntegrationService struct {
	objects  *repository.MediaObjectRepository
	upload   *UploadService
	media    *MediaObjectService
	videos   *VideoService
	delivery *DeliveryService
	quota    *integrationquota.Limiter
}

func NewIntegrationService(
	objects *repository.MediaObjectRepository,
	upload *UploadService,
	media *MediaObjectService,
	videos *VideoService,
	delivery *DeliveryService,
	quota *integrationquota.Limiter,
) *IntegrationService {
	return &IntegrationService{
		objects:  objects,
		upload:   upload,
		media:    media,
		videos:   videos,
		delivery: delivery,
		quota:    quota,
	}
}

func (s *IntegrationService) actor(key *repository.APIKey) (userID int64, role string) {
	return *key.CreatedBy, "owner"
}

func (s *IntegrationService) rootFolderPublicID(key *repository.APIKey) uuid.UUID {
	if key.RootFolderPublicID != nil {
		return *key.RootFolderPublicID
	}
	return integration.DefaultRootFolderPublicID
}

func (s *IntegrationService) rootFolderID(ctx context.Context, key *repository.APIKey) (int64, error) {
	root, err := s.objects.GetByPublicID(ctx, s.rootFolderPublicID(key))
	if err != nil {
		return 0, err
	}
	return root.ID, nil
}

func (s *IntegrationService) ObjectInNamespace(ctx context.Context, key *repository.APIKey, objectID int64) error {
	return s.objectInNamespace(ctx, key, objectID)
}

func (s *IntegrationService) objectInNamespace(ctx context.Context, key *repository.APIKey, objectID int64) error {
	rootID, err := s.rootFolderID(ctx, key)
	if err != nil {
		return err
	}
	ok, err := s.objects.IsDescendantOf(ctx, objectID, rootID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrIntegrationAccessDenied
	}
	return nil
}

func (s *IntegrationService) resolveUploadParent(ctx context.Context, key *repository.APIKey, parentPID *uuid.UUID) (uuid.UUID, error) {
	rootPub := s.rootFolderPublicID(key)
	var parent uuid.UUID
	if parentPID == nil || *parentPID == uuid.Nil {
		parent = rootPub
	} else {
		parent = *parentPID
	}
	parentObj, err := s.objects.GetByPublicID(ctx, parent)
	if err != nil {
		return uuid.Nil, err
	}
	if err := s.objectInNamespace(ctx, key, parentObj.ID); err != nil {
		return uuid.Nil, ErrIntegrationInvalidParent
	}
	if parentObj.Type != "folder" {
		return uuid.Nil, ErrMediaInvalidParent
	}
	return parent, nil
}

func (s *IntegrationService) GetObject(ctx context.Context, key *repository.APIKey, publicID uuid.UUID) (*MediaObjectDTO, error) {
	m, err := s.objects.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if err := s.objectInNamespace(ctx, key, m.ID); err != nil {
		return nil, ErrIntegrationAccessDenied
	}
	userID, role := s.actor(key)
	return s.media.Get(ctx, userID, role, publicID)
}

func (s *IntegrationService) DeleteObject(ctx context.Context, key *repository.APIKey, publicID uuid.UUID, ip, ua string) (*DeleteObjectResult, error) {
	m, err := s.objects.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if err := s.objectInNamespace(ctx, key, m.ID); err != nil {
		return nil, ErrIntegrationAccessDenied
	}
	userID, role := s.actor(key)
	return s.media.Delete(ctx, userID, role, publicID, ip, ua)
}

func (s *IntegrationService) InitUpload(ctx context.Context, key *repository.APIKey, in InitUploadInput, parentPID *uuid.UUID) (*InitUploadResult, error) {
	if s.quota != nil {
		ok, _ := s.quota.AllowUploadInit(ctx, key.ID)
		if !ok {
			return nil, ErrIntegrationQuotaExceeded
		}
	}
	parent, err := s.resolveUploadParent(ctx, key, parentPID)
	if err != nil {
		return nil, err
	}
	in.ParentPublicID = parent
	userID, role := s.actor(key)
	return s.upload.Init(ctx, userID, role, in)
}

func (s *IntegrationService) PutChunk(ctx context.Context, key *repository.APIKey, sessionID uuid.UUID, index int, body io.Reader, size int64) error {
	userID, _ := s.actor(key)
	return s.upload.PutChunk(ctx, userID, sessionID, index, body, size)
}

func (s *IntegrationService) CompleteUpload(ctx context.Context, key *repository.APIKey, sessionID uuid.UUID, ip, ua string) (*MediaObjectDTO, error) {
	userID, role := s.actor(key)
	return s.upload.Complete(ctx, userID, role, sessionID, ip, ua)
}

func (s *IntegrationService) AbortUpload(ctx context.Context, key *repository.APIKey, sessionID uuid.UUID) error {
	userID, _ := s.actor(key)
	return s.upload.Abort(ctx, userID, sessionID)
}

func (s *IntegrationService) UploadLimits(ctx context.Context) (*UploadLimitsResult, error) {
	return s.upload.Limits(ctx)
}

func (s *IntegrationService) StartConvert(ctx context.Context, key *repository.APIKey, publicID uuid.UUID, input StartConvertInput, ip, ua string) (*ConvertJobDTO, error) {
	if s.quota != nil {
		ok, _ := s.quota.AllowConvert(ctx, key.ID)
		if !ok {
			return nil, ErrIntegrationQuotaExceeded
		}
	}
	m, err := s.objects.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if err := s.objectInNamespace(ctx, key, m.ID); err != nil {
		return nil, ErrIntegrationAccessDenied
	}
	if m.Type != "video" {
		return nil, fmt.Errorf("%w: not a video", ErrVideoNotFound)
	}
	userID, role := s.actor(key)
	return s.videos.StartConvert(ctx, userID, role, ip, ua, publicID, input)
}

func (s *IntegrationService) RetryConvert(ctx context.Context, key *repository.APIKey, publicID uuid.UUID, input StartConvertInput, ip, ua string) (*ConvertJobDTO, error) {
	if s.quota != nil {
		ok, _ := s.quota.AllowConvert(ctx, key.ID)
		if !ok {
			return nil, ErrIntegrationQuotaExceeded
		}
	}
	m, err := s.objects.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if err := s.objectInNamespace(ctx, key, m.ID); err != nil {
		return nil, ErrIntegrationAccessDenied
	}
	if m.Type != "video" {
		return nil, fmt.Errorf("%w: not a video", ErrVideoNotFound)
	}
	userID, role := s.actor(key)
	return s.videos.RetryConvert(ctx, userID, role, ip, ua, publicID, input)
}

func (s *IntegrationService) CancelConvert(ctx context.Context, key *repository.APIKey, publicID uuid.UUID, ip, ua string) error {
	m, err := s.objects.GetByPublicID(ctx, publicID)
	if err != nil {
		return err
	}
	if err := s.objectInNamespace(ctx, key, m.ID); err != nil {
		return ErrIntegrationAccessDenied
	}
	if m.Type != "video" {
		return fmt.Errorf("%w: not a video", ErrVideoNotFound)
	}
	userID, role := s.actor(key)
	return s.videos.CancelConvert(ctx, userID, role, ip, ua, publicID)
}

func (s *IntegrationService) GetHLSAccess(ctx context.Context, key *repository.APIKey, publicID uuid.UUID) (*HLSAccessDTO, error) {
	m, err := s.objects.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if err := s.objectInNamespace(ctx, key, m.ID); err != nil {
		return nil, ErrIntegrationAccessDenied
	}
	userID, role := s.actor(key)
	access, err := s.videos.GetHLSAccess(ctx, userID, role, publicID)
	if err != nil {
		return nil, err
	}
	if s.delivery != nil {
		access.EmbedHTML = s.delivery.BuildEmbedHTML(publicID.String())
	}
	return access, nil
}

type IntegrationMediaDTO struct {
	PublicID     string              `json:"public_id"`
	Type         string              `json:"type"`
	Name         string              `json:"name"`
	MimeType     *string             `json:"mime_type,omitempty"`
	SizeBytes    int64               `json:"size_bytes"`
	HLSStatus    *string             `json:"hls_status,omitempty"`
	CreatedAt    string              `json:"created_at"`
	UpdatedAt    string              `json:"updated_at"`
	DeliveryURLs MediaDeliveryURLs   `json:"delivery_urls"`
}

func (s *IntegrationService) GetMedia(ctx context.Context, key *repository.APIKey, publicID uuid.UUID) (*IntegrationMediaDTO, error) {
	m, err := s.objects.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if err := s.objectInNamespace(ctx, key, m.ID); err != nil {
		return nil, ErrIntegrationAccessDenied
	}
	dto := &IntegrationMediaDTO{
		PublicID:  m.PublicID.String(),
		Type:      m.Type,
		Name:      m.Name,
		MimeType:  m.MimeType,
		SizeBytes: m.SizeBytes,
		CreatedAt: m.CreatedAt.UTC().Format(timeRFC3339),
		UpdatedAt: m.UpdatedAt.UTC().Format(timeRFC3339),
	}
	var hlsMaster string
	if m.Type == "video" {
		userID, role := s.actor(key)
		if access, err := s.videos.GetHLSAccess(ctx, userID, role, publicID); err == nil {
			hlsMaster = access.MasterURL
		}
		if st, err := s.videos.HLSStatus(ctx, publicID); err == nil {
			dto.HLSStatus = &st
		}
	}
	allowDL, _ := s.videos.AllowSourceDownload(ctx, m)
	if s.delivery != nil {
		dto.DeliveryURLs = s.delivery.BuildForObject(m, hlsMaster != "", hlsMaster, allowDL)
	}
	return dto, nil
}

const timeRFC3339 = "2006-01-02T15:04:05Z"

func (s *IntegrationService) DeliveryURLs(ctx context.Context, key *repository.APIKey, ids []uuid.UUID) (map[string]MediaDeliveryURLs, error) {
	if len(ids) == 0 {
		return map[string]MediaDeliveryURLs{}, nil
	}
	if len(ids) > maxBatchAccessURLs {
		return nil, fmt.Errorf("too many ids: max %d", maxBatchAccessURLs)
	}
	out := make(map[string]MediaDeliveryURLs, len(ids))
	rootID, err := s.rootFolderID(ctx, key)
	if err != nil {
		return nil, err
	}
	for _, pid := range ids {
		m, err := s.objects.GetByPublicID(ctx, pid)
		if err != nil {
			continue
		}
		ok, err := s.objects.IsDescendantOf(ctx, m.ID, rootID)
		if err != nil || !ok {
			continue
		}
		var hlsMaster string
		if m.Type == "video" {
			userID, role := s.actor(key)
			if access, err := s.videos.GetHLSAccess(ctx, userID, role, pid); err == nil {
				hlsMaster = access.MasterURL
			}
		}
		if s.delivery != nil {
			allowDL, _ := s.videos.AllowSourceDownload(ctx, m)
			out[pid.String()] = s.delivery.BuildForObject(m, hlsMaster != "", hlsMaster, allowDL)
		}
	}
	return out, nil
}

func contentTypeFromMime(mime *string) string {
	if mime != nil && *mime != "" {
		return *mime
	}
	return "application/octet-stream"
}

// StorageKeyForVariant maps a delivery variant to an object storage key.
func StorageKeyForVariant(m *repository.MediaObject, variant string) (string, string, error) {
	switch variant {
	case integration.AssetVariantThumbnail:
		if m.ThumbnailKey != nil && *m.ThumbnailKey != "" {
			return *m.ThumbnailKey, "image/jpeg", nil
		}
		if m.Type == "image" && m.StorageKey != nil && *m.StorageKey != "" {
			return *m.StorageKey, contentTypeFromMime(m.MimeType), nil
		}
		return "", "", ErrMediaAccessDenied
	case integration.AssetVariantImage:
		if m.Type != "image" || m.StorageKey == nil || *m.StorageKey == "" {
			return "", "", ErrMediaAccessDenied
		}
		return *m.StorageKey, contentTypeFromMime(m.MimeType), nil
	case integration.AssetVariantFile:
		if m.StorageKey == nil || *m.StorageKey == "" {
			return "", "", ErrMediaAccessDenied
		}
		return *m.StorageKey, contentTypeFromMime(m.MimeType), nil
	default:
		return "", "", ErrMediaAccessDenied
	}
}
