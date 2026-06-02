package background

import (
	"context"
	"time"

	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"go.uber.org/zap"
)

// Scheduler runs periodic maintenance jobs (storage deletions, upload expiry, trash purge, …).
type Scheduler struct {
	Log            *zap.Logger
	Upload         *service.UploadService
	StorageCleanup *service.StorageCleanupService
	DeletionRepo   *repository.StorageDeletionRepository
	Media          *service.MediaObjectService
	MediaRepo      *repository.MediaObjectRepository
}

// Run blocks until ctx is cancelled. Jobs run once on start where the API previously did.
func (s *Scheduler) Run(ctx context.Context) {
	s.runStorageDeletions(ctx)
	s.runTrashPurge(ctx)
	s.runClosureRepair(ctx)

	go s.loop(ctx, 15*time.Minute, "expire stale uploads", func(ctx context.Context) {
		n, err := s.Upload.ExpireStaleSessions(ctx)
		if err != nil {
			return 0, err
		}
		return n, nil
	})

	go s.loop(ctx, 30*time.Second, "storage deletions", func(ctx context.Context) {
		if n, err := s.DeletionRepo.RequeueStaleProcessing(ctx, 3*time.Minute); err != nil {
			s.Log.Warn("requeue stale storage deletion jobs", zap.Error(err))
		} else if n > 0 {
			s.Log.Info("requeued stale storage deletion jobs", zap.Int64("count", n))
		}
		n, err := s.StorageCleanup.ProcessPendingDeletions(ctx, 50)
		return n, err
	})

	go s.loop(ctx, 24*time.Hour, "purge expired trash", func(ctx context.Context) {
		n, err := s.Media.PurgeExpiredTrash(ctx)
		return n, err
	})

	go s.loop(ctx, 24*time.Hour, "cleanup stale temp objects", func(ctx context.Context) {
		n, err := s.StorageCleanup.CleanupStaleTempObjects(ctx)
		return n, err
	})

	go s.loop(ctx, 24*time.Hour, "repair closure paths", func(ctx context.Context) {
		n, err := s.MediaRepo.RepairClosurePaths(ctx)
		return int(n), err
	})

	<-ctx.Done()
}

func (s *Scheduler) runStorageDeletions(ctx context.Context) {
	if n, err := s.DeletionRepo.RequeueStaleProcessing(ctx, 3*time.Minute); err != nil {
		s.Log.Warn("requeue stale storage deletion jobs", zap.Error(err))
	} else if n > 0 {
		s.Log.Info("requeued stale storage deletion jobs", zap.Int64("count", n))
	}
	if n, err := s.StorageCleanup.ProcessPendingDeletions(ctx, 50); err != nil {
		s.Log.Warn("storage deletion jobs", zap.Error(err))
	} else if n > 0 {
		s.Log.Info("processed storage deletions", zap.Int("count", n))
	}
}

func (s *Scheduler) runTrashPurge(ctx context.Context) {
	n, err := s.Media.PurgeExpiredTrash(ctx)
	if err != nil {
		s.Log.Warn("purge expired trash", zap.Error(err))
	} else if n > 0 {
		s.Log.Info("purged expired trash objects", zap.Int("count", n))
	}
}

func (s *Scheduler) runClosureRepair(ctx context.Context) {
	n, err := s.MediaRepo.RepairClosurePaths(ctx)
	if err != nil {
		s.Log.Warn("closure path repair on startup", zap.Error(err))
	} else if n > 0 {
		s.Log.Info("repaired closure paths on startup", zap.Int64("removed", n))
	}
}

func (s *Scheduler) loop(
	ctx context.Context,
	interval time.Duration,
	name string,
	fn func(context.Context) (int, error),
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := fn(ctx)
			if err != nil {
				s.Log.Warn(name, zap.Error(err))
			} else if n > 0 {
				s.Log.Info(name, zap.Int("count", n))
			}
		}
	}
}
