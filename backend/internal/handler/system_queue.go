package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	streamplat "github.com/anhtuanlc/mediahub/internal/platform/stream"
	workerhb "github.com/anhtuanlc/mediahub/internal/platform/worker"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

type QueueHandler struct {
	Videos         *repository.VideoRepository
	VideoSvc       *service.VideoService
	Objects        *repository.MediaObjectRepository
	DeletionJobs   *repository.StorageDeletionRepository
	ConvertEnqueue *service.ConvertEnqueue
	Audit          *repository.AuditRepository
	RedisAddr      string
	Metrics        *streamplat.Metrics
	Heartbeat      *workerhb.Heartbeat
	Inspector      *asynq.Inspector

	mediaSummaryCache struct {
		mu sync.RWMutex
		at time.Time
		v  gin.H
	}
}

const queueMediaSummaryTTL = 30 * time.Second

// NewQueueHandler wires a long-lived asynq inspector (closed via Close on API shutdown).
func NewQueueHandler(
	videos *repository.VideoRepository,
	videoSvc *service.VideoService,
	objects *repository.MediaObjectRepository,
	deletionJobs *repository.StorageDeletionRepository,
	convertEnqueue *service.ConvertEnqueue,
	audit *repository.AuditRepository,
	redisAddr string,
	metrics *streamplat.Metrics,
) *QueueHandler {
	return &QueueHandler{
		Videos:         videos,
		VideoSvc:       videoSvc,
		Objects:        objects,
		DeletionJobs:   deletionJobs,
		ConvertEnqueue: convertEnqueue,
		Audit:          audit,
		RedisAddr:      redisAddr,
		Metrics:        metrics,
		Inspector:      asynq.NewInspector(asynq.RedisClientOpt{Addr: redisAddr}),
	}
}

func (h *QueueHandler) Close() error {
	if h.Inspector == nil {
		return nil
	}
	return h.Inspector.Close()
}

func (h *QueueHandler) queueDepthAndPaused(ctx context.Context) (depth int, paused bool, err error) {
	_ = ctx
	if h.ConvertEnqueue != nil {
		depth, err = h.ConvertEnqueue.PendingDepth(ctx)
		if err != nil {
			return 0, false, err
		}
		paused, err = h.ConvertEnqueue.IsQueuePaused(ctx)
		return depth, paused, err
	}
	if h.Inspector == nil {
		return 0, false, nil
	}
	info, err := h.Inspector.GetQueueInfo("default")
	if err != nil {
		// Same as ConvertEnqueue: empty Asynq queue is not a hard failure.
		if service.IsAsynqQueueMissing(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if info == nil {
		return 0, false, nil
	}
	return info.Pending + info.Active + info.Scheduled + info.Retry, info.Paused, nil
}

func (h *QueueHandler) QueueStatus(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	jobs, err := h.Videos.CountJobsByStatus(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	videos, _ := h.Videos.CountByHLSStatus(ctx)

	var mediaObjects gin.H
	if h.Objects != nil {
		if summary := h.cachedMediaSummary(ctx); summary != nil {
			mediaObjects = summary
		}
	}

	queueDepth, queuePaused, err := h.queueDepthAndPaused(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	queueMax := 0
	if h.ConvertEnqueue != nil {
		queueMax = h.ConvertEnqueue.MaxDepth()
	}

	resp := gin.H{
		"convert_jobs": jobs,
		"video_hls":    videos,
		"queue_depth":  queueDepth,
		"queue_paused": queuePaused,
	}
	if queueMax > 0 {
		resp["queue_max_depth"] = queueMax
	}

	if active, err := h.Videos.ListActiveConvertJobs(ctx, 50); err == nil {
		if items := buildRunningJobItems(active); len(items) > 0 {
			resp["running_jobs"] = items
		}
	}

	if failed, err := h.Videos.ListRecentFailedJobs(ctx, 50); err == nil {
		if items := buildFailedJobItems(failed); len(items) > 0 {
			resp["failed_jobs"] = items
		}
	}
	if mediaObjects != nil {
		resp["media_objects"] = mediaObjects
	}
	if h.DeletionJobs != nil {
		if counts, err := h.DeletionJobs.CountByStatus(ctx); err == nil && len(counts) > 0 {
			resp["storage_deletion_jobs"] = counts
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *QueueHandler) PauseQueue(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if h.ConvertEnqueue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	actor, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	if err := h.ConvertEnqueue.PauseQueue(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}
	if h.Audit != nil {
		_ = h.Audit.Log(ctx, &actor.ID, "system.queue.pause", "queue", nil, c.ClientIP(), c.Request.UserAgent(), nil)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "queue_paused": true})
}

func (h *QueueHandler) ResumeQueue(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if h.ConvertEnqueue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	actor, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	if err := h.ConvertEnqueue.ResumeQueue(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}
	if h.Audit != nil {
		_ = h.Audit.Log(ctx, &actor.ID, "system.queue.resume", "queue", nil, c.ClientIP(), c.Request.UserAgent(), nil)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "queue_paused": false})
}

func (h *QueueHandler) DeleteFailedJob(c *gin.Context) {
	if h.VideoSvc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	actor, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	jobPID, err := uuid.Parse(c.Param("job_public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid job_public_id"})
		return
	}
	if err := h.VideoSvc.DeleteFailedConvertJob(c.Request.Context(), actor.ID, actor.Role, c.ClientIP(), c.Request.UserAgent(), jobPID); err != nil {
		switch {
		case errors.Is(err, service.ErrVideoAccessDenied):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		case errors.Is(err, service.ErrConvertJobNotDismissible):
			c.JSON(http.StatusConflict, gin.H{"error": "conflict", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type overviewQueueSnapshot struct {
	Content gin.H
	Queue   gin.H
}

func buildRunningJobItems(rows []repository.ActiveConvertJobRow) []gin.H {
	if len(rows) == 0 {
		return nil
	}
	items := make([]gin.H, 0, len(rows))
	for i := range rows {
		row := rows[i]
		item := gin.H{
			"job_public_id":   row.JobPublicID.String(),
			"video_public_id": row.VideoPublicID.String(),
			"video_name":      row.VideoName,
			"status":          row.Status,
			"attempts":        row.Attempts,
			"max_attempts":    row.MaxAttempts,
		}
		if row.StartedAt != nil {
			item["started_at"] = row.StartedAt.UTC().Format(time.RFC3339)
		}
		items = append(items, item)
	}
	return items
}

func buildFailedJobItems(rows []repository.FailedConvertJobRow) []gin.H {
	if len(rows) == 0 {
		return nil
	}
	items := make([]gin.H, 0, len(rows))
	for i := range rows {
		row := rows[i]
		item := gin.H{
			"job_public_id":   row.JobPublicID.String(),
			"video_public_id": row.VideoPublicID.String(),
			"video_name":      row.VideoName,
			"attempts":        row.Attempts,
			"max_attempts":    row.MaxAttempts,
		}
		if row.Error != nil {
			item["error"] = *row.Error
		}
		if row.FinishedAt != nil {
			item["finished_at"] = row.FinishedAt.UTC().Format(time.RFC3339)
		}
		items = append(items, item)
	}
	return items
}

// OverviewSnapshot builds a compact queue + content snapshot for the dashboard overview API.
func (h *QueueHandler) OverviewSnapshot(ctx context.Context, jobLimit int) (*overviewQueueSnapshot, error) {
	jobs, err := h.Videos.CountJobsByStatus(ctx)
	if err != nil {
		return nil, err
	}
	videos, _ := h.Videos.CountByHLSStatus(ctx)

	var mediaObjects gin.H
	if h.Objects != nil {
		if summary := h.cachedMediaSummary(ctx); summary != nil {
			mediaObjects = summary
		}
	}

	queueDepth, queuePaused, err := h.queueDepthAndPaused(ctx)
	if err != nil {
		return nil, err
	}

	queueMax := 0
	if h.ConvertEnqueue != nil {
		queueMax = h.ConvertEnqueue.MaxDepth()
	}

	queue := gin.H{
		"depth":         queueDepth,
		"paused":        queuePaused,
		"convert_jobs":  jobs,
	}
	if queueMax > 0 {
		queue["max_depth"] = queueMax
	}

	if active, err := h.Videos.ListActiveConvertJobs(ctx, jobLimit); err == nil {
		if items := buildRunningJobItems(active); len(items) > 0 {
			queue["running_jobs"] = items
		}
	}
	if failed, err := h.Videos.ListRecentFailedJobs(ctx, jobLimit); err == nil {
		if items := buildFailedJobItems(failed); len(items) > 0 {
			queue["failed_jobs"] = items
		}
	}
	if h.DeletionJobs != nil {
		if counts, err := h.DeletionJobs.CountByStatus(ctx); err == nil && len(counts) > 0 {
			queue["storage_deletion_jobs"] = counts
		}
	}

	content := gin.H{
		"video_hls":     videos,
		"videos_active": sumVideosActive(videos),
	}
	if mediaObjects != nil {
		content["media_objects"] = mediaObjects
	}

	return &overviewQueueSnapshot{Content: content, Queue: queue}, nil
}

func (h *QueueHandler) cachedMediaSummary(ctx context.Context) gin.H {
	h.mediaSummaryCache.mu.RLock()
	if v := h.mediaSummaryCache.v; v != nil && time.Since(h.mediaSummaryCache.at) < queueMediaSummaryTTL {
		h.mediaSummaryCache.mu.RUnlock()
		return v
	}
	h.mediaSummaryCache.mu.RUnlock()

	summary, err := h.Objects.CountActiveSummary(ctx)
	if err != nil || summary == nil {
		return nil
	}
	out := gin.H{
		"total":   summary.Total,
		"folders": summary.Folders,
		"files":   summary.Files,
	}
	h.mediaSummaryCache.mu.Lock()
	h.mediaSummaryCache.v = out
	h.mediaSummaryCache.at = time.Now()
	h.mediaSummaryCache.mu.Unlock()
	return out
}

func (h *QueueHandler) StreamAnalytics(c *gin.Context) {
	ctx := c.Request.Context()
	now := time.Now().UTC()
	out := gin.H{"date": now.Format("2006-01-02")}
	if h.Metrics != nil {
		snap, err := h.Metrics.AnalyticsSnapshot(ctx)
		if err == nil {
			for k, v := range snap {
				out[k] = v
			}
		}
	}
	c.JSON(http.StatusOK, out)
}

func CheckWorkerHeartbeat(ctx context.Context, r *redis.Client) componentHealth {
	hb := workerhb.NewHeartbeat(r)
	if workers, ok := hb.ActiveWorkers(ctx); ok && len(workers) > 0 {
		details := map[string]string{
			"active_workers": strconv.Itoa(len(workers)),
		}
		if len(workers) == 1 {
			details["last_seen"] = workers[0].LastSeen
		}
		return componentHealth{
			Status:  "healthy",
			Details: details,
		}
	}
	if at, ok := hb.LastSeen(ctx); ok {
		return componentHealth{
			Status:  "healthy",
			Details: map[string]string{"last_seen": at},
		}
	}
	return componentHealth{
		Status:  "unknown",
		Details: map[string]string{"note": "no worker heartbeat — is make worker running?"},
	}
}
