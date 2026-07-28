package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/download"
	"github.com/anhtuanlc/mediahub/internal/mediautil"
	downloadprog "github.com/anhtuanlc/mediahub/internal/platform/download"
	"github.com/anhtuanlc/mediahub/internal/platform/rediscache"
	"github.com/anhtuanlc/mediahub/internal/platform/resource"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type DownloadRuntimeLimits struct {
	ChunkConcurrency int
	JobTimeout       time.Duration
	MaxConcurrent    int
}

type DownloadDeps struct {
	Log                *zap.Logger
	Pool               *pgxpool.Pool
	Store              storage.ObjectStorage
	YTDLPPath          string
	FFmpegPath         string
	ChunkConcurrency   int
	MaxBytes           int64
	JobTimeout         time.Duration
	Progress           *downloadprog.ProgressStore
	StreamCache        *rediscache.Store
	Gate               *resource.DynamicGate
	ResolveLimits      func(ctx context.Context) DownloadRuntimeLimits
	EnqueueConvert     func(ctx context.Context, jobPublicID, videoPublicID uuid.UUID, objectID int64, variants []string, maxAttempts int) error
	HeartbeatTouch     func(ctx context.Context) error
}

type DownloadProcessor struct {
	deps DownloadDeps
}

func NewDownloadProcessor(deps DownloadDeps) *DownloadProcessor {
	return &DownloadProcessor{deps: deps}
}

func (p *DownloadProcessor) emit(ctx context.Context, job *repository.DownloadJob, status, stage string, pct int, done, total int64, force bool) {
	if job == nil {
		return
	}

	// Persist to Postgres ~1/s (always on force/terminal). Redis snapshot is
	// refreshed on every Publish so reload still sees live progress without
	// hammering the DB on every read-buffer progress tick.
	if p.deps.Progress == nil || p.deps.Progress.ShouldPersistDB(ctx, job.PublicID.String(), force) {
		_ = repository.NewDownloadJobRepository(p.deps.Pool).UpdateProgress(ctx, job.ID, pct, done, total)
	}
	if p.deps.Progress == nil {
		return
	}
	createdBy := int64(0)
	if job.CreatedBy != nil {
		createdBy = *job.CreatedBy
	}
	ev := downloadprog.JobEvent{
		PublicID:      job.PublicID.String(),
		CreatedBy:     createdBy,
		Status:        status,
		Kind:          job.Kind,
		SourceURL:     job.SourceURL,
		Title:         job.Title,
		ThumbnailURL:  job.ThumbnailURL,
		ProgressPct:   pct,
		ProgressStage: stage,
		BytesDone:     done,
		BytesTotal:    total,
		CreatedAt:     job.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if job.VideoPublicID != nil {
		v := job.VideoPublicID.String()
		ev.VideoPublicID = &v
	}
	if job.LastError != nil {
		ev.LastError = job.LastError
	}
	_ = p.deps.Progress.Publish(ctx, ev, force)
}

func (p *DownloadProcessor) dbCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 15*time.Second)
}

func (p *DownloadProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	if p.deps.Gate != nil {
		if err := p.deps.Gate.Acquire(ctx); err != nil {
			return err
		}
		defer p.deps.Gate.Release()
	}
	if p.deps.HeartbeatTouch != nil {
		_ = p.deps.HeartbeatTouch(ctx)
	}
	var payload DownloadPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}
	jobPID, err := uuid.Parse(payload.JobPublicID)
	if err != nil {
		return err
	}
	jobs := repository.NewDownloadJobRepository(p.deps.Pool)
	job, err := jobs.GetByPublicID(ctx, jobPID)
	if err != nil {
		return err
	}
	if job.Status == "cancelled" || job.Status == "succeeded" {
		return nil
	}

	limits := p.resolveLimits(ctx)
	timeout := limits.JobTimeout
	if timeout <= 0 {
		timeout = 2 * time.Hour
	}
	jobCtx, jobCancel := context.WithTimeout(ctx, timeout)
	defer jobCancel()

	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		p.watchCancel(jobCtx, jobCancel, jobPID.String())
	}()
	defer func() { jobCancel(); <-watchDone }()

	if err := jobs.MarkRunning(jobCtx, job.ID); err != nil {
		if errors.Is(err, repository.ErrDownloadJobNotRunnable) {
			return nil
		}
		return err
	}
	job.Status = "running"
	p.emit(jobCtx, job, "running", "starting", 5, 0, 0, true)

	if aborted, err := p.abortIfCancelled(jobCtx, jobs, job); err != nil {
		return err
	} else if aborted {
		p.emit(jobCtx, job, "cancelled", "cancelled", job.ProgressPct, job.BytesDone, job.BytesTotal, true)
		return nil
	}

	// Resume after partial ingest (object already created).
	if job.ObjectID != nil && *job.ObjectID > 0 && job.VideoPublicID != nil {
		var runErr error
		if job.Kind == download.KindHLS {
			runErr = p.resumeLinkedJob(jobCtx, jobs, job)
		} else {
			runErr = p.resumeIngestConvert(jobCtx, jobs, job)
		}
		return p.finishRun(jobCtx, jobs, job, jobPID.String(), runErr)
	}

	tmpDir, err := os.MkdirTemp("", "mediahub-dl-*")
	if err != nil {
		return p.fail(jobs, job, err.Error())
	}
	defer os.RemoveAll(tmpDir)

	var runErr error
	switch job.Kind {
	case download.KindHLS:
		runErr = p.runHLS(jobCtx, jobs, job, tmpDir, limits.ChunkConcurrency)
	case download.KindYTDLP:
		runErr = p.runYTDLP(jobCtx, jobs, job, tmpDir)
	default:
		runErr = p.runDirect(jobCtx, jobs, job, tmpDir)
	}
	return p.finishRun(jobCtx, jobs, job, jobPID.String(), runErr)
}

func (p *DownloadProcessor) finishRun(jobCtx context.Context, jobs *repository.DownloadJobRepository, job *repository.DownloadJob, jobPID string, runErr error) error {
	if runErr == nil {
		p.clearProgress(jobPID)
		return nil
	}
	if p.isUserCancel(jobCtx, jobPID) {
		dbCtx, cancel := p.dbCtx()
		defer cancel()
		_ = jobs.MarkCancelled(dbCtx, job.ID)
		p.emit(dbCtx, job, "cancelled", "cancelled", job.ProgressPct, job.BytesDone, job.BytesTotal, true)
		p.clearProgress(jobPID)
		return nil
	}
	if errors.Is(runErr, context.DeadlineExceeded) || errors.Is(jobCtx.Err(), context.DeadlineExceeded) {
		return p.fail(jobs, job, "timeout")
	}
	msg := runErr.Error()
	if errors.Is(runErr, context.Canceled) || errors.Is(jobCtx.Err(), context.Canceled) {
		msg = "interrupted"
	}
	return p.fail(jobs, job, msg)
}

func (p *DownloadProcessor) watchCancel(ctx context.Context, cancel context.CancelFunc, jobPID string) {
	if p.deps.Progress == nil {
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkCtx, checkCancel := context.WithTimeout(context.Background(), 2*time.Second)
			ok, err := p.deps.Progress.IsCancelRequested(checkCtx, jobPID)
			checkCancel()
			if err == nil && ok {
				cancel()
				return
			}
		}
	}
}

func (p *DownloadProcessor) abortIfCancelled(ctx context.Context, jobs *repository.DownloadJobRepository, job *repository.DownloadJob) (bool, error) {
	if !p.isUserCancel(ctx, job.PublicID.String()) {
		return false, nil
	}
	dbCtx, cancel := p.dbCtx()
	defer cancel()
	_ = jobs.MarkCancelled(dbCtx, job.ID)
	p.clearProgress(job.PublicID.String())
	return true, nil
}

func (p *DownloadProcessor) clearProgress(jobPID string) {
	if p.deps.Progress == nil {
		return
	}
	dbCtx, cancel := p.dbCtx()
	defer cancel()
	_ = p.deps.Progress.Clear(dbCtx, jobPID)
	_ = p.deps.Progress.ClearCancel(dbCtx, jobPID)
}

func (p *DownloadProcessor) fail(jobs *repository.DownloadJobRepository, job *repository.DownloadJob, msg string) error {
	dbCtx, cancel := p.dbCtx()
	defer cancel()
	_ = jobs.MarkFailed(dbCtx, job.ID, msg)
	job.LastError = &msg
	p.emit(dbCtx, job, "failed", "failed", job.ProgressPct, job.BytesDone, job.BytesTotal, true)
	p.clearProgress(job.PublicID.String())
	if p.deps.Log != nil {
		p.deps.Log.Warn("download.failed",
			zap.String("job", job.PublicID.String()),
			zap.String("url", download.RedactURL(job.SourceURL)),
			zap.String("error", msg),
		)
	}
	return fmt.Errorf("%s", msg)
}

func (p *DownloadProcessor) isUserCancel(ctx context.Context, jobPID string) bool {
	if p.deps.Progress == nil {
		return false
	}
	checkCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ok, err := p.deps.Progress.IsCancelRequested(checkCtx, jobPID)
	if err != nil {
		return false
	}
	if ok {
		return true
	}
	_ = ctx
	return false
}

func (p *DownloadProcessor) runDirect(ctx context.Context, jobs *repository.DownloadJobRepository, job *repository.DownloadJob, tmpDir string) error {
	url := job.SourceURL
	if job.ResolvedURL != nil && *job.ResolvedURL != "" {
		url = *job.ResolvedURL
	}
	dest := filepath.Join(tmpDir, "file.bin")
	// No overall Client.Timeout — large MP4s exceed 90s; job ctx enforces DOWNLOAD_JOB_TIMEOUT.
	client := download.SafeHTTPClient(0)
	// Progressive MP4/direct: single connection only (CDN often resets multi-range).
	const concurrency = 1
	p.emit(ctx, job, "running", "download", 10, 0, 0, true)

	ct, size, err := download.FetchToFile(ctx, client, url, dest, concurrency, p.deps.MaxBytes, func(done, total int64) {
		pct := 10
		if total > 0 {
			pct = 10 + int(done*70/total)
		}
		p.emit(ctx, job, "running", "download", pct, done, total, false)
	})
	if err != nil {
		return err
	}
	name := pickFilename(job, ct, url)
	return p.ingestFileAndConvert(ctx, jobs, job, dest, name, ct, size)
}

func (p *DownloadProcessor) runYTDLP(ctx context.Context, jobs *repository.DownloadJobRepository, job *repository.DownloadJob, tmpDir string) error {
	var sel struct {
		FormatID string `json:"format_id"`
		URL      string `json:"url"`
		IsHLS    bool   `json:"is_hls"`
		Ext      string `json:"ext"`
	}
	_ = json.Unmarshal(job.SelectedFormat, &sel)

	if sel.IsHLS || (sel.URL != "" && strings.Contains(strings.ToLower(sel.URL), ".m3u8")) {
		url := sel.URL
		if url == "" && job.ResolvedURL != nil {
			url = *job.ResolvedURL
		}
		if url == "" {
			return fmt.Errorf("missing hls url")
		}
		job.ResolvedURL = &url
		return p.runHLS(ctx, jobs, job, tmpDir, p.resolveLimits(ctx).ChunkConcurrency)
	}

	// Prefer direct URL multi-chunk when we have a progressive media URL.
	if sel.URL != "" && !sel.IsHLS {
		job.ResolvedURL = &sel.URL
		return p.runDirect(ctx, jobs, job, tmpDir)
	}

	p.emit(ctx, job, "running", "ytdlp", 15, 0, 0, true)
	dest := filepath.Join(tmpDir, "ytdlp.%(ext)s")
	outTpl := filepath.Join(tmpDir, "out.%(ext)s")
	_ = dest
	if err := download.DownloadYTDLPFile(ctx, p.deps.YTDLPPath, job.SourceURL, sel.FormatID, outTpl, p.deps.MaxBytes); err != nil {
		return err
	}
	matches, _ := filepath.Glob(filepath.Join(tmpDir, "out.*"))
	if len(matches) == 0 {
		return fmt.Errorf("yt-dlp produced no file")
	}
	path := matches[0]
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if p.deps.MaxBytes > 0 && st.Size() > p.deps.MaxBytes {
		return fmt.Errorf("file exceeds max size (%d > %d)", st.Size(), p.deps.MaxBytes)
	}
	name := filepath.Base(path)
	if job.Title != nil && *job.Title != "" {
		ext := filepath.Ext(path)
		name = mediautil.SanitizeName(*job.Title) + ext
	}
	ct := mime.TypeByExtension(filepath.Ext(path))
	return p.ingestFileAndConvert(ctx, jobs, job, path, name, ct, st.Size())
}

func (p *DownloadProcessor) resolveLimits(ctx context.Context) DownloadRuntimeLimits {
	lim := DownloadRuntimeLimits{
		ChunkConcurrency: p.deps.ChunkConcurrency,
		JobTimeout:       p.deps.JobTimeout,
	}
	if p.deps.ResolveLimits != nil {
		resolved := p.deps.ResolveLimits(ctx)
		if resolved.ChunkConcurrency > 0 {
			lim.ChunkConcurrency = resolved.ChunkConcurrency
		}
		if resolved.JobTimeout > 0 {
			lim.JobTimeout = resolved.JobTimeout
		}
		if resolved.MaxConcurrent > 0 {
			lim.MaxConcurrent = resolved.MaxConcurrent
			if p.deps.Gate != nil {
				p.deps.Gate.SetLimit(resolved.MaxConcurrent)
			}
		}
	}
	if lim.ChunkConcurrency <= 0 {
		lim.ChunkConcurrency = 2
	}
	if lim.JobTimeout <= 0 {
		lim.JobTimeout = 2 * time.Hour
	}
	return lim
}

func (p *DownloadProcessor) runHLS(ctx context.Context, jobs *repository.DownloadJobRepository, job *repository.DownloadJob, tmpDir string, chunkConcurrency int) error {
	url := job.SourceURL
	if job.ResolvedURL != nil && *job.ResolvedURL != "" {
		url = *job.ResolvedURL
	}
	var sel struct {
		URL   string `json:"url"`
		IsHLS bool   `json:"is_hls"`
	}
	_ = json.Unmarshal(job.SelectedFormat, &sel)
	if sel.URL != "" {
		url = sel.URL
	}

	hlsDir := filepath.Join(tmpDir, "hls")
	p.emit(ctx, job, "running", "hls_mirror", 10, 0, 0, true)
	client := download.SafeHTTPClient(0)
	hlsConc := chunkConcurrency
	if hlsConc <= 0 {
		hlsConc = 2
	}
	res, err := download.MirrorHLS(ctx, client, url, hlsDir, hlsConc, p.deps.MaxBytes, func(done, total int64) {
		pct := 10
		if total > 0 {
			pct = 10 + int(done*60/total)
		} else if done > 0 {
			pct = 10 + int(done%60)
			if pct > 70 {
				pct = 70
			}
		}
		p.emit(ctx, job, "running", "hls_mirror", pct, done, total, false)
	})
	if err != nil {
		return err
	}

	name := "hls-stream"
	if job.Title != nil && *job.Title != "" {
		name = mediautil.SanitizeName(*job.Title)
	}
	userID := int64(0)
	if job.CreatedBy != nil {
		userID = *job.CreatedBy
	}
	parentID := int64(0)
	if job.ParentFolderID != nil {
		parentID = *job.ParentFolderID
	}
	if parentID == 0 {
		return fmt.Errorf("missing parent folder")
	}
	if aborted, err := p.abortIfCancelled(ctx, jobs, job); err != nil {
		return err
	} else if aborted {
		return context.Canceled
	}

	videoPID := uuid.New()
	mimeType := "application/vnd.apple.mpegurl"
	objects := repository.NewMediaObjectRepository(p.deps.Pool)
	m, err := objects.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:     videoPID,
		ParentID:     &parentID,
		Type:         "video",
		Name:         name,
		OriginalName: &name,
		MimeType:     &mimeType,
		SizeBytes:    res.BytesTotal,
		CreatedBy:    userID,
	})
	if err != nil {
		return err
	}
	if err := objects.CreateVideoAsset(ctx, m.ID); err != nil {
		return err
	}
	if err := jobs.LinkMedia(ctx, job.ID, videoPID, m.ID); err != nil {
		if errors.Is(err, repository.ErrDownloadJobNotRunnable) {
			return context.Canceled
		}
		return err
	}
	job.VideoPublicID = &videoPID
	oid := m.ID
	job.ObjectID = &oid

	p.emit(ctx, job, "running", "upload", 75, res.BytesTotal, res.BytesTotal, true)
	prefix := storage.HLSPrefix(videoPID)
	masterKey := storage.HLSMasterKey(videoPID)
	if err := uploadDir(ctx, p.deps.Store, hlsDir, prefix); err != nil {
		return err
	}
	// Ensure master key matches layout (mirror writes master.m3u8 at root).
	if res.MasterRel != "master.m3u8" {
		src := filepath.Join(hlsDir, res.MasterRel)
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		st, _ := f.Stat()
		size := int64(0)
		if st != nil {
			size = st.Size()
		}
		err = p.deps.Store.PutObject(ctx, masterKey, f, size, "application/vnd.apple.mpegurl")
		f.Close()
		if err != nil {
			return err
		}
	}

	videos := repository.NewVideoRepository(p.deps.Pool)
	asset, err := videos.GetByObjectID(ctx, m.ID)
	if err != nil {
		return err
	}
	thumbKey := saveVideoThumbnail(
		ctx,
		p.deps.Log,
		p.deps.Store,
		videos,
		objects,
		p.deps.FFmpegPath,
		videoPID,
		asset.ID,
		m.ID,
		job.ThumbnailURL,
		hlsDir,
	)
	if err := videos.SetHLSReady(ctx, asset.ID, masterKey, prefix, thumbKey); err != nil {
		return err
	}
	rediscache.InvalidateStreamVideo(ctx, p.deps.StreamCache, videoPID)

	if err := jobs.MarkSucceeded(ctx, job.ID, videoPID, m.ID); err != nil {
		if errors.Is(err, repository.ErrDownloadJobNotRunnable) {
			return context.Canceled
		}
		return err
	}
	job.VideoPublicID = &videoPID
	p.emit(ctx, job, "succeeded", "done", 100, res.BytesTotal, res.BytesTotal, true)
	return nil
}

func (p *DownloadProcessor) resumeLinkedJob(ctx context.Context, jobs *repository.DownloadJobRepository, job *repository.DownloadJob) error {
	if aborted, err := p.abortIfCancelled(ctx, jobs, job); err != nil {
		return err
	} else if aborted {
		return context.Canceled
	}
	if job.VideoPublicID == nil || job.ObjectID == nil {
		return fmt.Errorf("missing linked media")
	}
	if err := jobs.MarkSucceeded(ctx, job.ID, *job.VideoPublicID, *job.ObjectID); err != nil {
		if errors.Is(err, repository.ErrDownloadJobNotRunnable) {
			return context.Canceled
		}
		return err
	}
	p.emit(ctx, job, "succeeded", "done", 100, job.BytesDone, job.BytesTotal, true)
	return nil
}

func (p *DownloadProcessor) resumeIngestConvert(ctx context.Context, jobs *repository.DownloadJobRepository, job *repository.DownloadJob) error {
	if aborted, err := p.abortIfCancelled(ctx, jobs, job); err != nil {
		return err
	} else if aborted {
		return context.Canceled
	}
	if job.ObjectID == nil || job.VideoPublicID == nil {
		return fmt.Errorf("missing linked media")
	}
	videos := repository.NewVideoRepository(p.deps.Pool)
	asset, err := videos.GetByObjectID(ctx, *job.ObjectID)
	if err != nil {
		return err
	}
	userID := int64(0)
	if job.CreatedBy != nil {
		userID = *job.CreatedBy
	}
	return p.enqueueConvertAndSucceed(ctx, jobs, job, videos, asset.ID, *job.VideoPublicID, *job.ObjectID, userID, job.BytesTotal)
}

func (p *DownloadProcessor) ingestFileAndConvert(ctx context.Context, jobs *repository.DownloadJobRepository, job *repository.DownloadJob, localPath, name, contentType string, size int64) error {
	userID := int64(0)
	if job.CreatedBy != nil {
		userID = *job.CreatedBy
	}
	parentID := int64(0)
	if job.ParentFolderID != nil {
		parentID = *job.ParentFolderID
	}
	if parentID == 0 {
		return fmt.Errorf("missing parent folder")
	}
	if name == "" {
		name = "download.mp4"
	}
	name = mediautil.SanitizeName(name)

	if aborted, err := p.abortIfCancelled(ctx, jobs, job); err != nil {
		return err
	} else if aborted {
		return context.Canceled
	}

	p.emit(ctx, job, "running", "upload", 80, size, size, true)
	key := storage.NewObjectKey(storage.PrefixOriginals)
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	ct := contentType
	if ct == "" {
		ct = mime.TypeByExtension(filepath.Ext(name))
	}
	if ct == "" {
		ct = "video/mp4"
	}
	if err := p.deps.Store.PutObject(ctx, key, f, size, ct); err != nil {
		f.Close()
		return err
	}
	f.Close()

	objects := repository.NewMediaObjectRepository(p.deps.Pool)
	mimePtr := &ct
	m, err := objects.CreateWithClosure(ctx, repository.CreateMediaObjectInput{
		PublicID:     uuid.New(),
		ParentID:     &parentID,
		Type:         "video",
		Name:         name,
		OriginalName: &name,
		MimeType:     mimePtr,
		SizeBytes:    size,
		StorageKey:   &key,
		CreatedBy:    userID,
	})
	if err != nil {
		return err
	}
	if err := objects.CreateVideoAsset(ctx, m.ID); err != nil {
		return err
	}
	if err := jobs.LinkMedia(ctx, job.ID, m.PublicID, m.ID); err != nil {
		if errors.Is(err, repository.ErrDownloadJobNotRunnable) {
			return context.Canceled
		}
		return err
	}
	job.VideoPublicID = &m.PublicID
	oid := m.ID
	job.ObjectID = &oid

	videos := repository.NewVideoRepository(p.deps.Pool)
	asset, err := videos.GetByObjectID(ctx, m.ID)
	if err != nil {
		return err
	}
	_ = saveVideoThumbnail(
		ctx,
		p.deps.Log,
		p.deps.Store,
		videos,
		objects,
		p.deps.FFmpegPath,
		m.PublicID,
		asset.ID,
		m.ID,
		job.ThumbnailURL,
		localPath,
	)

	return p.enqueueConvertAndSucceed(ctx, jobs, job, videos, asset.ID, m.PublicID, m.ID, userID, size)
}

func (p *DownloadProcessor) enqueueConvertAndSucceed(
	ctx context.Context,
	jobs *repository.DownloadJobRepository,
	job *repository.DownloadJob,
	videos *repository.VideoRepository,
	assetID int64,
	videoPID uuid.UUID,
	objectID int64,
	userID int64,
	size int64,
) error {
	p.emit(ctx, job, "running", "convert_queue", 95, size, size, true)

	if p.deps.EnqueueConvert != nil {
		latest, err := videos.LatestJobForAsset(ctx, assetID)
		if err != nil {
			return err
		}
		needEnqueue := latest == nil || (latest.Status != "pending" && latest.Status != "running" && latest.Status != "succeeded")
		if needEnqueue {
			convertJobPID := uuid.New()
			_, err := p.deps.Pool.Exec(ctx, `
				INSERT INTO convert_jobs (public_id, video_asset_id, status, created_by)
				VALUES ($1, $2, 'pending', $3)
			`, convertJobPID, assetID, userID)
			if err != nil {
				return err
			}
			_, _ = p.deps.Pool.Exec(ctx, `UPDATE video_assets SET hls_status = 'pending', updated_at = now() WHERE id = $1`, assetID)
			if err := p.deps.EnqueueConvert(ctx, convertJobPID, videoPID, objectID, nil, 3); err != nil {
				_ = videos.UpdateHLSStatus(ctx, assetID, "none")
				return fmt.Errorf("enqueue convert: %w", err)
			}
		}
	}

	if err := jobs.MarkSucceeded(ctx, job.ID, videoPID, objectID); err != nil {
		if errors.Is(err, repository.ErrDownloadJobNotRunnable) {
			return context.Canceled
		}
		return err
	}
	job.VideoPublicID = &videoPID
	p.emit(ctx, job, "succeeded", "done", 100, size, size, true)
	return nil
}

func pickFilename(job *repository.DownloadJob, ct, url string) string {
	if job.Title != nil && *job.Title != "" {
		ext := filepath.Ext(url)
		if ext == "" || len(ext) > 5 {
			ext = ".mp4"
			if strings.Contains(ct, "webm") {
				ext = ".webm"
			}
		}
		return mediautil.SanitizeName(*job.Title) + ext
	}
	base := filepath.Base(strings.Split(url, "?")[0])
	if base != "" && base != "." && base != "/" {
		return mediautil.SanitizeName(base)
	}
	return "download.mp4"
}

func uploadDir(ctx context.Context, store storage.ObjectStorage, localDir, keyPrefix string) error {
	return filepath.WalkDir(localDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		key := keyPrefix + rel
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil {
			return err
		}
		ct := "application/octet-stream"
		switch {
		case strings.HasSuffix(rel, ".m3u8"):
			ct = "application/vnd.apple.mpegurl"
		case strings.HasSuffix(rel, ".ts"):
			ct = "video/mp2t"
		case strings.HasSuffix(rel, ".m4s"):
			ct = "video/iso.segment"
		}
		return store.PutObject(ctx, key, f, st.Size(), ct)
	})
}
