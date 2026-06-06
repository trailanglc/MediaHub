package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	convertprogress "github.com/anhtuanlc/mediahub/internal/platform/convert"
	"github.com/anhtuanlc/mediahub/internal/platform/rediscache"
	"github.com/anhtuanlc/mediahub/internal/platform/resource"
	"github.com/anhtuanlc/mediahub/internal/platform/webhook"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/anhtuanlc/mediahub/internal/transcode"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type ConvertDeps struct {
	Log              *zap.Logger
	Pool             *pgxpool.Pool
	Store            storage.ObjectStorage
	FFmpegPath       string
	FFprobePath      string
	JobTimeout       time.Duration
	DeletePrefix     func(ctx context.Context, prefix string) error
	HeartbeatTouch   func(ctx context.Context) error
	ConvertProgress  *convertprogress.ProgressStore
	StreamCache      *rediscache.Store
	GovernorEnabled bool
	ResourcePolicy  resource.PolicyConfig
	Gate            *resource.DynamicGate
	Webhooks        *webhook.Dispatcher
}

type ConvertProcessor struct {
	deps ConvertDeps
}

func NewConvertProcessor(deps ConvertDeps) *ConvertProcessor {
	return &ConvertProcessor{deps: deps}
}

func (p *ConvertProcessor) report(ctx context.Context, videoPID string, stage string, percent int) {
	if p.deps.ConvertProgress != nil {
		_ = p.deps.ConvertProgress.Set(ctx, videoPID, stage, percent)
	}
}

func (p *ConvertProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	if p.deps.GovernorEnabled && p.deps.Gate != nil {
		if err := p.deps.Gate.Acquire(ctx); err != nil {
			return err
		}
		defer p.deps.Gate.Release()
	}
	lim := resource.LatestLimits(p.deps.ResourcePolicy)
	if p.deps.GovernorEnabled && lim.DeferConvert {
		if p.deps.Log != nil {
			p.deps.Log.Debug("convert.deferred", zap.String("reason", "low memory"))
		}
		return fmt.Errorf("%w: low memory", resource.ErrDeferred)
	}
	if p.deps.HeartbeatTouch != nil {
		_ = p.deps.HeartbeatTouch(ctx)
	}
	var payload ConvertPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}
	jobPID, err := uuid.Parse(payload.JobPublicID)
	if err != nil {
		return err
	}
	videoPID, err := uuid.Parse(payload.VideoPublicID)
	if err != nil {
		return err
	}
	videoID := videoPID.String()

	videos := repository.NewVideoRepository(p.deps.Pool)
	objects := repository.NewMediaObjectRepository(p.deps.Pool)

	job, err := videos.GetConvertJobByPublicID(ctx, jobPID)
	if err != nil {
		return err
	}
	if job.Status == "cancelled" {
		return nil
	}
	row, err := videos.GetVideoRowByAssetID(ctx, job.VideoAssetID)
	if err != nil {
		return err
	}
	if aborted, err := p.abortIfCancelled(ctx, payload, videos, job, row.Asset.ID, videoPID, videoID); err != nil {
		return err
	} else if aborted {
		return nil
	}
	if row.Media.StorageKey == nil || *row.Media.StorageKey == "" {
		return p.fail(ctx, payload, videos, job, row.Asset.ID, videoPID, videoID, "missing storage key")
	}

	startedAt := time.Now()
	if p.deps.Log != nil {
		p.deps.Log.Info("convert.started", p.convertLogFields(payload, job, videoPID)...)
	}

	_ = videos.UpdateJobStatus(ctx, job.ID, "running", nil)
	_ = videos.UpdateHLSStatus(ctx, row.Asset.ID, "converting")
	rediscache.InvalidateStreamVideo(ctx, p.deps.StreamCache, videoPID)
	p.report(ctx, videoID, "starting", 10)

	tmpDir, err := os.MkdirTemp("", "mediahub-convert-*")
	if err != nil {
		return p.fail(ctx, payload, videos, job, row.Asset.ID, videoPID, videoID, err.Error())
	}
	defer os.RemoveAll(tmpDir)

	p.report(ctx, videoID, "download", 15)
	sourcePath := filepath.Join(tmpDir, "source")
	if err := p.downloadSource(ctx, *row.Media.StorageKey, sourcePath); err != nil {
		return p.fail(ctx, payload, videos, job, row.Asset.ID, videoPID, videoID, err.Error())
	}
	if aborted, err := p.abortIfCancelled(ctx, payload, videos, job, row.Asset.ID, videoPID, videoID); err != nil {
		return err
	} else if aborted {
		return nil
	}

	cfg := transcode.Config{
		FFmpegPath:  p.deps.FFmpegPath,
		FFprobePath: p.deps.FFprobePath,
		Timeout:     p.deps.JobTimeout,
	}
	if p.deps.GovernorEnabled {
		cfg.Threads = lim.FFmpegThreads
	}
	if probe, err := transcode.Probe(ctx, cfg, sourcePath); err == nil {
		_ = videos.SetProbeMetadata(ctx, row.Asset.ID, &probe.DurationSeconds, &probe.Width, &probe.Height, &probe.Codec, &probe.Bitrate)
	} else if p.deps.Log != nil {
		p.deps.Log.Debug("convert.probe_failed", zap.Error(err), zap.String("video_public_id", videoPID.String()))
	}

	outDir := filepath.Join(tmpDir, "hls")
	if err := transcode.ConvertToHLS(ctx, cfg, sourcePath, outDir, payload.Variants, func(stage string, percent int) {
		p.report(ctx, videoID, stage, percent)
	}); err != nil {
		p.cleanupHLSPrefix(ctx, videoPID)
		return p.fail(ctx, payload, videos, job, row.Asset.ID, videoPID, videoID, err.Error())
	}
	if aborted, err := p.abortIfCancelled(ctx, payload, videos, job, row.Asset.ID, videoPID, videoID); err != nil {
		return err
	} else if aborted {
		return nil
	}
	if err := transcode.ValidateHLSOutput(outDir); err != nil {
		p.cleanupHLSPrefix(ctx, videoPID)
		return p.fail(ctx, payload, videos, job, row.Asset.ID, videoPID, videoID, err.Error())
	}

	p.report(ctx, videoID, "upload", 80)
	prefix := storage.HLSPrefix(videoPID)
	if err := p.uploadTree(ctx, outDir, prefix); err != nil {
		p.cleanupHLSPrefix(ctx, videoPID)
		return p.fail(ctx, payload, videos, job, row.Asset.ID, videoPID, videoID, err.Error())
	}

	p.report(ctx, videoID, "thumbnail", 92)
	thumbPath := filepath.Join(tmpDir, "thumb.jpg")
	thumbKey := storage.ThumbnailObjectKey(videoPID)
	if err := transcode.ExtractThumbnail(ctx, cfg, sourcePath, thumbPath); err == nil {
		if f, err := os.Open(thumbPath); err == nil {
			st, _ := f.Stat()
			_ = p.deps.Store.PutObject(ctx, thumbKey, f, st.Size(), "image/jpeg")
			f.Close()
			_ = videos.SetThumbnailKey(ctx, row.Asset.ID, thumbKey)
			_ = objects.SetThumbnailKey(ctx, row.Media.ID, thumbKey)
		}
	}

	p.report(ctx, videoID, "finalize", 98)
	masterKey := storage.HLSMasterKey(videoPID)
	if err := videos.SetHLSReady(ctx, row.Asset.ID, masterKey, prefix, thumbKey); err != nil {
		return err
	}
	rediscache.InvalidateStreamVideo(ctx, p.deps.StreamCache, videoPID)
	errMsg := (*string)(nil)
	if err := videos.UpdateJobStatus(ctx, job.ID, "succeeded", errMsg); err != nil {
		return err
	}
	p.report(ctx, videoID, "done", 100)
	if p.deps.ConvertProgress != nil {
		_ = p.deps.ConvertProgress.Clear(ctx, videoID)
	}
	if p.deps.Webhooks != nil {
		p.deps.Webhooks.Emit(ctx, webhook.EventConvertCompleted, map[string]any{
			"public_id": videoPID.String(),
			"job_id":    jobPID.String(),
		})
	}
	if p.deps.Log != nil {
		fields := p.convertLogFields(payload, job, videoPID)
		fields = append(fields, zap.Duration("duration", time.Since(startedAt)))
		p.deps.Log.Info("convert.succeeded", fields...)
	}
	return nil
}

func (p *ConvertProcessor) downloadSource(ctx context.Context, key, dest string) error {
	body, err := p.deps.Store.GetObject(ctx, key)
	if err != nil {
		return err
	}
	defer body.Close()
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, body)
	return err
}

func (p *ConvertProcessor) uploadTree(ctx context.Context, localDir, storagePrefix string) error {
	return filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}
		key := storagePrefix + strings.ReplaceAll(rel, "\\", "/")
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		ct := "application/octet-stream"
		if strings.HasSuffix(path, ".m3u8") {
			ct = "application/vnd.apple.mpegurl"
		} else if strings.HasSuffix(path, ".ts") {
			ct = "video/mp2t"
		}
		return p.deps.Store.PutObject(ctx, key, f, info.Size(), ct)
	})
}

func (p *ConvertProcessor) cleanupHLSPrefix(ctx context.Context, videoPID uuid.UUID) {
	if p.deps.DeletePrefix != nil {
		_ = p.deps.DeletePrefix(ctx, storage.HLSPrefix(videoPID))
	}
}

func (p *ConvertProcessor) abortIfCancelled(
	ctx context.Context,
	payload ConvertPayload,
	videos *repository.VideoRepository,
	job *repository.ConvertJob,
	assetID int64,
	videoPID uuid.UUID,
	videoID string,
) (bool, error) {
	cancelled := job.Status == "cancelled"
	if !cancelled && p.deps.ConvertProgress != nil {
		req, err := p.deps.ConvertProgress.IsCancelRequested(ctx, payload.JobPublicID)
		if err != nil {
			return false, err
		}
		cancelled = req
	}
	if !cancelled {
		return false, nil
	}
	p.cleanupHLSPrefix(ctx, videoPID)
	_ = videos.UpdateHLSStatus(ctx, assetID, "failed")
	if p.deps.ConvertProgress != nil {
		_ = p.deps.ConvertProgress.Clear(ctx, videoID)
		_ = p.deps.ConvertProgress.ClearCancel(ctx, payload.JobPublicID)
	}
	return true, nil
}

func (p *ConvertProcessor) convertLogFields(payload ConvertPayload, job *repository.ConvertJob, videoPID uuid.UUID) []zap.Field {
	fields := []zap.Field{
		zap.String("job_id", job.PublicID.String()),
		zap.String("video_public_id", videoPID.String()),
		zap.Int("attempt", job.Attempts+1),
	}
	if payload.RequestID != "" {
		fields = append(fields, zap.String("request_id", payload.RequestID))
	}
	if len(payload.Variants) > 0 {
		fields = append(fields, zap.Strings("variants", payload.Variants))
	}
	return fields
}

func (p *ConvertProcessor) fail(ctx context.Context, payload ConvertPayload, videos *repository.VideoRepository, job *repository.ConvertJob, assetID int64, videoPID uuid.UUID, videoID, msg string) error {
	rediscache.InvalidateStreamVideo(ctx, p.deps.StreamCache, videoPID)
	p.cleanupHLSPrefix(ctx, videoPID)
	_ = videos.SetLastError(ctx, assetID, msg)
	errMsg := msg
	_ = videos.UpdateJobStatus(ctx, job.ID, "failed", &errMsg)
	if p.deps.ConvertProgress != nil {
		_ = p.deps.ConvertProgress.Clear(ctx, videoID)
	}
	if p.deps.Webhooks != nil {
		p.deps.Webhooks.Emit(ctx, webhook.EventConvertFailed, map[string]any{
			"public_id": videoPID.String(),
			"job_id":    job.PublicID.String(),
			"error":     msg,
		})
	}
	if p.deps.Log != nil {
		fields := p.convertLogFields(payload, job, videoPID)
		fields = append(fields, zap.String("error", msg))
		if job.Attempts+1 < job.MaxAttempts {
			p.deps.Log.Warn("convert.retry", fields...)
			return fmt.Errorf("convert failed (retryable): %s", msg)
		}
		p.deps.Log.Error("convert.failed", fields...)
		return nil
	}
	if job.Attempts+1 < job.MaxAttempts {
		return fmt.Errorf("convert failed (retryable): %s", msg)
	}
	return nil
}
