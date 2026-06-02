package service

import (
	"context"
	"fmt"
	"time"

	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/storage"
)

const (
	tempCleanupAge = 7 * 24 * time.Hour

	// API path only enqueues jobs; S3 work runs in background.
	deletionImmediateSlots   = 8
	deletionImmediateTimeout = 20 * time.Second
	deletionObjectJobTimeout = 30 * time.Second
	deletionPrefixJobTimeout = 10 * time.Minute
)

// StorageCleanupService removes objects from S3/MinIO without blocking HTTP handlers.
//
// Flow on delete:
//  1. Enqueue jobs (single keys + optional prefix for HLS).
//  2. Fire-and-forget goroutine: delete single-key jobs only (fast for any file size).
//  3. Periodic worker: prefix / failed / saturated immediate queue.
type StorageCleanupService struct {
	jobs    *repository.StorageDeletionRepository
	store   storage.ObjectStorage
	bgSlots chan struct{}
}

func NewStorageCleanupService(
	jobs *repository.StorageDeletionRepository,
	store storage.ObjectStorage,
) *StorageCleanupService {
	return &StorageCleanupService{
		jobs:    jobs,
		store:   store,
		bgSlots: make(chan struct{}, deletionImmediateSlots),
	}
}

// ScheduleDeletion enqueues storage cleanup and kicks a non-blocking fast path for single keys.
func (s *StorageCleanupService) ScheduleDeletion(ctx context.Context, m *repository.MediaObject) error {
	return s.ScheduleDeletions(ctx, []*repository.MediaObject{m})
}

// ScheduleDeletions enqueues S3 cleanup for many deleted objects (e.g. folder subtree).
// Immediate per-object deletes run only for small batches; large trees rely on the periodic worker.
func (s *StorageCleanupService) ScheduleDeletions(ctx context.Context, objects []*repository.MediaObject) error {
	for _, m := range objects {
		if m == nil {
			continue
		}
		if err := s.EnqueueForDeletedObject(ctx, m); err != nil {
			return err
		}
	}
	const immediateMax = 5
	if len(objects) <= immediateMax {
		for _, m := range objects {
			if m != nil {
				go s.runImmediateDeletion(m.ID)
			}
		}
	}
	return nil
}

func (s *StorageCleanupService) runImmediateDeletion(objectID int64) {
	select {
	case s.bgSlots <- struct{}{}:
	default:
		// Worker pool saturated; periodic worker will process jobs.
		return
	}
	defer func() { <-s.bgSlots }()

	ctx, cancel := context.WithTimeout(context.Background(), deletionImmediateTimeout)
	defer cancel()
	_, _ = s.processFastJobsForObject(ctx, objectID)
}

func (s *StorageCleanupService) EnqueueForDeletedObject(ctx context.Context, m *repository.MediaObject) error {
	if m.StorageKey != nil && *m.StorageKey != "" {
		if err := s.jobs.Enqueue(ctx, m.ID, *m.StorageKey, false); err != nil {
			return err
		}
	}
	if m.ThumbnailKey != nil && *m.ThumbnailKey != "" {
		if err := s.jobs.Enqueue(ctx, m.ID, *m.ThumbnailKey, false); err != nil {
			return err
		}
	} else if m.Type == "image" {
		if err := s.jobs.Enqueue(ctx, m.ID, storage.ThumbnailObjectKey(m.PublicID), false); err != nil {
			return err
		}
	}
	// Prefix deletes (HLS segments) can be large; never run on the request path.
	if m.Type == "video" {
		prefix := fmt.Sprintf("%s%s/", storage.PrefixHLS, m.PublicID.String())
		if err := s.jobs.Enqueue(ctx, m.ID, prefix, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *StorageCleanupService) processFastJobsForObject(ctx context.Context, objectID int64) (int, error) {
	jobs, err := s.jobs.ClaimFastJobsForObject(ctx, objectID, 8)
	if err != nil {
		return 0, err
	}
	return s.runJobs(ctx, jobs, deletionObjectJobTimeout)
}

func (s *StorageCleanupService) ProcessPendingDeletions(ctx context.Context, limit int) (int, error) {
	jobs, err := s.jobs.ClaimPending(ctx, limit)
	if err != nil {
		return 0, err
	}
	var done int
	for _, job := range jobs {
		timeout := deletionObjectJobTimeout
		if job.IsPrefix {
			timeout = deletionPrefixJobTimeout
		}
		n, err := s.runJobs(ctx, []repository.StorageDeletionJob{job}, timeout)
		done += n
		if err != nil {
			return done, err
		}
	}
	return done, nil
}

func (s *StorageCleanupService) runJobs(ctx context.Context, jobs []repository.StorageDeletionJob, timeout time.Duration) (int, error) {
	var done int
	for _, job := range jobs {
		jobCtx, cancel := context.WithTimeout(ctx, timeout)
		execErr := s.executeJob(jobCtx, job)
		cancel()
		if execErr != nil {
			_ = s.jobs.MarkFailed(ctx, job.ID, execErr.Error())
			continue
		}
		if err := s.jobs.MarkDone(ctx, job.ID); err != nil {
			return done, err
		}
		done++
	}
	return done, nil
}

func (s *StorageCleanupService) executeJob(ctx context.Context, job repository.StorageDeletionJob) error {
	if job.IsPrefix {
		deleter, ok := s.store.(*storage.S3Storage)
		if !ok {
			return fmt.Errorf("prefix delete not supported for storage driver")
		}
		return deleter.DeletePrefix(ctx, job.StorageKey)
	}
	return s.store.DeleteObject(ctx, job.StorageKey)
}

// CleanupStaleTempObjects removes objects under temp/ older than tempCleanupAge.
func (s *StorageCleanupService) CleanupStaleTempObjects(ctx context.Context) (int, error) {
	cutoff := time.Now().Add(-tempCleanupAge)
	return s.store.DeleteObjectsOlderThan(ctx, storage.PrefixTemp, cutoff)
}
