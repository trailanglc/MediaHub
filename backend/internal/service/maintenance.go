package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/storage"
)

const MaxOrphanDeletesPerRun = 500

type MaintenanceService struct {
	storageCleanup *StorageCleanupService
	upload         *UploadService
	objects        *repository.MediaObjectRepository
	deletionJobs   *repository.StorageDeletionRepository
	uploadSessions *repository.UploadSessionRepository
	store          storage.ObjectStorage
	audit          *repository.AuditRepository
}

func NewMaintenanceService(
	storageCleanup *StorageCleanupService,
	upload *UploadService,
	objects *repository.MediaObjectRepository,
	deletionJobs *repository.StorageDeletionRepository,
	uploadSessions *repository.UploadSessionRepository,
	store storage.ObjectStorage,
	audit *repository.AuditRepository,
) *MaintenanceService {
	return &MaintenanceService{
		storageCleanup: storageCleanup,
		upload:         upload,
		objects:        objects,
		deletionJobs:   deletionJobs,
		uploadSessions: uploadSessions,
		store:          store,
		audit:          audit,
	}
}

type CleanupTempResult struct {
	ExpiredUploadSessions int `json:"expired_upload_sessions"`
	RemovedTempObjects    int `json:"removed_temp_objects"`
}

func (s *MaintenanceService) CleanupTemp(ctx context.Context) (*CleanupTempResult, error) {
	res := &CleanupTempResult{}
	if s.upload != nil {
		n, err := s.upload.ExpireStaleSessions(ctx)
		if err != nil {
			return nil, fmt.Errorf("expire upload sessions: %w", err)
		}
		res.ExpiredUploadSessions = n
	}
	if s.storageCleanup != nil {
		n, err := s.storageCleanup.CleanupStaleTempObjects(ctx)
		if err != nil {
			return nil, fmt.Errorf("cleanup temp objects: %w", err)
		}
		res.RemovedTempObjects = n
	}
	return res, nil
}

type CleanupOrphansInput struct {
	DryRun    bool
	MaxDelete int
	Prefixes  []string
}

type CleanupOrphansResult struct {
	DryRun      bool     `json:"dry_run"`
	Scanned     int      `json:"scanned"`
	OrphanCount int      `json:"orphan_count"`
	Deleted     int      `json:"deleted"`
	SampleKeys  []string `json:"sample_keys,omitempty"`
	Truncated   bool     `json:"truncated,omitempty"`
}

func (s *MaintenanceService) CleanupOrphans(ctx context.Context, in CleanupOrphansInput) (*CleanupOrphansResult, error) {
	s3, ok := s.store.(*storage.S3Storage)
	if !ok {
		return nil, fmt.Errorf("orphan cleanup requires S3 storage")
	}

	maxDelete := in.MaxDelete
	if maxDelete <= 0 {
		maxDelete = MaxOrphanDeletesPerRun
	}

	prefixes := in.Prefixes
	if len(prefixes) == 0 {
		prefixes = []string{storage.PrefixThumbnails, storage.PrefixHLS, storage.PrefixOriginals}
	}

	refs, err := s.objects.ListReferencedStorageKeys(ctx)
	if err != nil {
		return nil, err
	}
	publicIDs, err := s.objects.ListKnownPublicIDs(ctx)
	if err != nil {
		return nil, err
	}
	guardKeys, guardPrefixes, err := s.buildStorageGuards(ctx)
	if err != nil {
		return nil, err
	}
	for _, k := range guardKeys {
		refs[k] = struct{}{}
	}

	res := &CleanupOrphansResult{DryRun: in.DryRun}
	var orphans []string

	for _, prefix := range prefixes {
		keys, err := s3.ListObjectKeys(ctx, prefix)
		if err != nil {
			return nil, err
		}
		res.Scanned += len(keys)
		for _, key := range keys {
			if isProtectedStorageKey(key, refs, guardPrefixes, publicIDs) {
				continue
			}
			orphans = append(orphans, key)
		}
	}

	res.OrphanCount = len(orphans)
	if len(orphans) > maxDelete {
		res.Truncated = true
		orphans = orphans[:maxDelete]
	}
	if len(orphans) > 20 {
		res.SampleKeys = append([]string(nil), orphans[:20]...)
	} else {
		res.SampleKeys = orphans
	}

	if in.DryRun {
		return res, nil
	}

	for _, key := range orphans {
		if err := s.store.DeleteObject(ctx, key); err != nil {
			return res, fmt.Errorf("delete orphan %s: %w", key, err)
		}
		res.Deleted++
	}
	return res, nil
}

func (s *MaintenanceService) CleanupOrphansWithAudit(ctx context.Context, in CleanupOrphansInput, actorID int64, ip, ua string) (*CleanupOrphansResult, error) {
	res, err := s.CleanupOrphans(ctx, in)
	if err != nil {
		return nil, err
	}
	if s.audit != nil && !in.DryRun && res.Deleted > 0 {
		meta := map[string]string{
			"deleted": fmt.Sprintf("%d", res.Deleted),
			"scanned": fmt.Sprintf("%d", res.Scanned),
		}
		_ = s.audit.Log(ctx, &actorID, "system.cleanup.orphans", "storage", nil, ip, ua, meta)
	}
	return res, nil
}

func (s *MaintenanceService) CleanupTempWithAudit(ctx context.Context, actorID int64, ip, ua string) (*CleanupTempResult, error) {
	res, err := s.CleanupTemp(ctx)
	if err != nil {
		return nil, err
	}
	if s.audit != nil && (res.ExpiredUploadSessions > 0 || res.RemovedTempObjects > 0) {
		meta := map[string]string{
			"expired_uploads": fmt.Sprintf("%d", res.ExpiredUploadSessions),
			"removed_temp":    fmt.Sprintf("%d", res.RemovedTempObjects),
		}
		_ = s.audit.Log(ctx, &actorID, "system.cleanup.temp", "storage", nil, ip, ua, meta)
	}
	return res, nil
}

func (s *MaintenanceService) buildStorageGuards(ctx context.Context) (keys []string, prefixes []string, err error) {
	if s.uploadSessions != nil {
		p, k, err := s.uploadSessions.ListPendingStorageGuards(ctx)
		if err != nil {
			return nil, nil, err
		}
		prefixes = append(prefixes, p...)
		keys = append(keys, k...)
	}
	if s.deletionJobs != nil {
		k, p, err := s.deletionJobs.ListActiveDeletionGuards(ctx)
		if err != nil {
			return nil, nil, err
		}
		keys = append(keys, k...)
		prefixes = append(prefixes, p...)
	}
	return keys, prefixes, nil
}

func isProtectedStorageKey(key string, refs map[string]struct{}, guardPrefixes []string, publicIDs map[string]struct{}) bool {
	if _, ok := refs[key]; ok {
		return true
	}
	for _, p := range guardPrefixes {
		if p != "" && strings.HasPrefix(key, p) {
			return true
		}
	}
	if id := publicIDFromStorageKey(key); id != "" {
		if _, ok := publicIDs[id]; ok {
			return true
		}
	}
	return false
}

func publicIDFromStorageKey(key string) string {
	switch {
	case strings.HasPrefix(key, storage.PrefixThumbnails):
		rest := strings.TrimPrefix(key, storage.PrefixThumbnails)
		rest = strings.TrimSuffix(rest, ".jpg")
		if len(rest) == 36 {
			return rest
		}
	case strings.HasPrefix(key, storage.PrefixHLS):
		rest := strings.TrimPrefix(key, storage.PrefixHLS)
		if i := strings.Index(rest, "/"); i == 36 {
			return rest[:36]
		}
	}
	return ""
}
