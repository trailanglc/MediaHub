package background

import (
	"context"
	"sync"
	"time"

	"github.com/anhtuanlc/mediahub/internal/platform/resource"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"go.uber.org/zap"
)

const schedulerTickerShutdownTimeout = 30 * time.Second

// Scheduler runs periodic maintenance jobs (storage deletions, upload expiry, trash purge, …).
type Scheduler struct {
	Log               *zap.Logger
	Upload            *service.UploadService
	StorageCleanup    *service.StorageCleanupService
	DeletionRepo      *repository.StorageDeletionRepository
	Media             *service.MediaObjectService
	MediaRepo         *repository.MediaObjectRepository
	Settings          *service.SettingsService
	Videos            *repository.VideoRepository
	VideoSvc          *service.VideoService
	Refresh           *repository.RefreshTokenRepository
	Resources         *resource.Reader
	ConvertJobTimeout time.Duration
}

// Run blocks until ctx is cancelled. Jobs run once on start where the API previously did.
func (s *Scheduler) Run(ctx context.Context) {
	s.runStorageDeletions(ctx)
	s.runTrashPurge(ctx)
	s.runAuditPurge(ctx)
	s.runRetentionPurge(ctx)
	s.runClosureRepair(ctx)

	var wg sync.WaitGroup

	wg.Add(1)
	go s.runTicker(&wg, ctx, 15*time.Minute, "expire stale uploads", func(ctx context.Context) (int, error) {
		n, err := s.Upload.ExpireStaleSessions(ctx)
		if err != nil {
			return 0, err
		}
		return n, nil
	})

	wg.Add(1)
	go s.runTicker(&wg, ctx, 30*time.Second, "storage deletions", func(ctx context.Context) (int, error) {
		if n, err := s.DeletionRepo.RequeueStaleProcessing(ctx, 3*time.Minute); err != nil {
			s.Log.Warn("requeue stale storage deletion jobs", zap.Error(err))
		} else if n > 0 {
			s.Log.Info("requeued stale storage deletion jobs", zap.Int64("count", n))
		}
		n, err := s.StorageCleanup.ProcessPendingDeletions(ctx, s.deletionBatch(ctx))
		return n, err
	})

	wg.Add(1)
	go s.runTicker(&wg, ctx, 5*time.Minute, "stale convert jobs", func(ctx context.Context) (int, error) {
		if s.VideoSvc == nil {
			return 0, nil
		}
		olderThan := s.ConvertJobTimeout + 5*time.Minute
		if olderThan <= 0 {
			olderThan = 2*time.Hour + 5*time.Minute
		}
		return s.VideoSvc.FailStaleRunningJobs(ctx, olderThan)
	})

	wg.Add(1)
	go s.runTicker(&wg, ctx, 24*time.Hour, "purge expired trash", func(ctx context.Context) (int, error) {
		n, err := s.Media.PurgeExpiredTrash(ctx)
		return n, err
	})

	wg.Add(1)
	go s.runTicker(&wg, ctx, 24*time.Hour, "cleanup stale temp objects", func(ctx context.Context) (int, error) {
		n, err := s.StorageCleanup.CleanupStaleTempObjects(ctx)
		return n, err
	})

	wg.Add(1)
	go s.runTicker(&wg, ctx, 24*time.Hour, "purge retention data", func(ctx context.Context) (int, error) {
		return s.purgeRetentionData(ctx)
	})

	wg.Add(1)
	go s.runTicker(&wg, ctx, 24*time.Hour, "purge expired audit logs", func(ctx context.Context) (int, error) {
		return s.purgeExpiredAuditLogs(ctx)
	})

	wg.Add(1)
	go s.runTicker(&wg, ctx, 24*time.Hour, "repair closure paths", func(ctx context.Context) (int, error) {
		n, err := s.MediaRepo.RepairClosurePaths(ctx)
		return int(n), err
	})

	<-ctx.Done()

	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
		s.Log.Info("scheduler tickers stopped")
	case <-time.After(schedulerTickerShutdownTimeout):
		s.Log.Warn("scheduler shutdown timed out waiting for tickers",
			zap.Duration("timeout", schedulerTickerShutdownTimeout))
	}
}

func (s *Scheduler) deletionBatch(ctx context.Context) int {
	const fallback = 50
	if s.Resources == nil || !s.Resources.Enabled {
		return fallback
	}
	_, lim, err := s.Resources.Current(ctx)
	if err != nil {
		return fallback
	}
	if lim.SchedulerDeletionBatch < 1 {
		return fallback
	}
	return lim.SchedulerDeletionBatch
}

func (s *Scheduler) runStorageDeletions(ctx context.Context) {
	if n, err := s.DeletionRepo.RequeueStaleProcessing(ctx, 3*time.Minute); err != nil {
		s.Log.Warn("requeue stale storage deletion jobs", zap.Error(err))
	} else if n > 0 {
		s.Log.Info("requeued stale storage deletion jobs", zap.Int64("count", n))
	}
	if n, err := s.StorageCleanup.ProcessPendingDeletions(ctx, s.deletionBatch(ctx)); err != nil {
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

func (s *Scheduler) runAuditPurge(ctx context.Context) {
	n, err := s.purgeExpiredAuditLogs(ctx)
	if err != nil {
		s.Log.Warn("purge expired audit logs", zap.Error(err))
	} else if n > 0 {
		s.Log.Info("purged expired audit logs", zap.Int("count", n))
	}
}

func (s *Scheduler) purgeExpiredAuditLogs(ctx context.Context) (int, error) {
	if s.Settings == nil {
		return 0, nil
	}
	return s.Settings.PurgeExpiredAuditLogs(ctx)
}

func (s *Scheduler) runRetentionPurge(ctx context.Context) {
	n, err := s.purgeRetentionData(ctx)
	if err != nil {
		s.Log.Warn("purge retention data", zap.Error(err))
	} else if n > 0 {
		s.Log.Info("purged retention data", zap.Int("count", n))
	}
}

func (s *Scheduler) purgeRetentionData(ctx context.Context) (int, error) {
	if s.Settings == nil {
		return 0, nil
	}
	total := 0
	if n, err := s.Settings.PurgeOldJobRecords(ctx, s.Videos, s.DeletionRepo); err != nil {
		return total, err
	} else {
		total += n
	}
	if n, err := s.Settings.PurgeStaleRefreshTokens(ctx, s.Refresh); err != nil {
		return total, err
	} else {
		total += n
	}
	return total, nil
}

func (s *Scheduler) runClosureRepair(ctx context.Context) {
	n, err := s.MediaRepo.RepairClosurePaths(ctx)
	if err != nil {
		s.Log.Warn("closure path repair on startup", zap.Error(err))
	} else if n > 0 {
		s.Log.Info("repaired closure paths on startup", zap.Int64("removed", n))
	}
}

func (s *Scheduler) runTicker(
	wg *sync.WaitGroup,
	ctx context.Context,
	interval time.Duration,
	name string,
	fn func(context.Context) (int, error),
) {
	defer wg.Done()
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
