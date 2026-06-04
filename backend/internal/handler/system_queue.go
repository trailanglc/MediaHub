package handler

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/anhtuanlc/mediahub/internal/repository"
	streamplat "github.com/anhtuanlc/mediahub/internal/platform/stream"
	workerhb "github.com/anhtuanlc/mediahub/internal/platform/worker"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

type QueueHandler struct {
	Videos    *repository.VideoRepository
	Objects   *repository.MediaObjectRepository
	RedisAddr string
	Metrics   *streamplat.Metrics
	Heartbeat *workerhb.Heartbeat
	Inspector *asynq.Inspector

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
	objects *repository.MediaObjectRepository,
	redisAddr string,
	metrics *streamplat.Metrics,
) *QueueHandler {
	return &QueueHandler{
		Videos:    videos,
		Objects:   objects,
		RedisAddr: redisAddr,
		Metrics:   metrics,
		Inspector: asynq.NewInspector(asynq.RedisClientOpt{Addr: redisAddr}),
	}
}

func (h *QueueHandler) Close() error {
	if h.Inspector == nil {
		return nil
	}
	return h.Inspector.Close()
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

	if h.Inspector == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	pending, _ := h.Inspector.GetQueueInfo("default")
	queueDepth := 0
	if pending != nil {
		queueDepth = pending.Pending + pending.Active + pending.Scheduled
	}

	resp := gin.H{
		"convert_jobs": jobs,
		"video_hls":    videos,
		"queue_depth":  queueDepth,
	}
	if mediaObjects != nil {
		resp["media_objects"] = mediaObjects
	}
	c.JSON(http.StatusOK, resp)
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
	out := gin.H{}
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
