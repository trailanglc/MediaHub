package handler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/observability"
	"github.com/anhtuanlc/mediahub/internal/platform/capacity"
	"github.com/anhtuanlc/mediahub/internal/platform/resource"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/anhtuanlc/mediahub/internal/platform/cache"
	"github.com/anhtuanlc/mediahub/internal/platform/upload"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var apiStartedAt = time.Now()

type HealthHandler struct {
	DB              *pgxpool.Pool
	Redis           *redis.Client
	Storage         storage.ObjectStorage
	MetricsCacheTTL time.Duration
	HostCache       *cache.TTLCache[*observability.HostStats]
	Resources       *resource.Reader
	ConvertQueue    *service.ConvertEnqueue
}

type componentHealth struct {
	Status  string            `json:"status"`
	Error   string            `json:"error,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

type HealthWarning struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type systemHealthResponse struct {
	Status                  string                     `json:"status"`
	Timestamp               time.Time                  `json:"timestamp"`
	APIUptimeSeconds        float64                    `json:"api_uptime_seconds"`
	GoVersion               string                     `json:"go_version"`
	NumGoroutine            int                        `json:"num_goroutine"`
	MetricsCacheTTLSeconds  int                        `json:"metrics_cache_ttl_seconds,omitempty"`
	MetricsCachedAt         *time.Time                 `json:"metrics_cached_at,omitempty"`
	UploadMetrics           map[string]uint64          `json:"upload_metrics,omitempty"`
	Host                    *observability.HostStats   `json:"host,omitempty"`
	ResourceSnapshot        *resource.Snapshot         `json:"resource_snapshot,omitempty"`
	ResourceLimits          *resource.Limits           `json:"resource_limits,omitempty"`
	QueueDepth              int                        `json:"queue_depth,omitempty"`
	QueueMaxDepth           int                        `json:"queue_max_depth,omitempty"`
	Warnings                []HealthWarning            `json:"warnings,omitempty"`
	Components              map[string]componentHealth `json:"components"`
}

func (h *HealthHandler) SystemHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	hostStats, _ := h.cachedHostStats(ctx)

	var resSnap *resource.Snapshot
	var resLimits *resource.Limits
	if h.Resources != nil && h.Resources.Enabled {
		if snap, lim, err := h.Resources.Current(ctx); err == nil {
			resSnap = snap
			resLimits = &lim
		}
	}

	queueDepth := 0
	queueMax := 0
	if h.ConvertQueue != nil {
		queueMax = h.ConvertQueue.MaxDepth()
		if depth, err := h.ConvertQueue.PendingDepth(ctx); err == nil {
			queueDepth = depth
		}
	}

	var metricsCachedAt *time.Time
	if h.HostCache != nil {
		if at := h.HostCache.CachedAt(); !at.IsZero() {
			t := at.UTC()
			metricsCachedAt = &t
		}
	}

	components := map[string]componentHealth{
		"database": h.checkDB(ctx),
		"redis":    h.checkRedis(ctx),
		"storage":  h.checkStorage(ctx),
		"worker":   h.checkWorker(ctx),
		"ffmpeg":   h.checkFFmpeg(),
	}

	overall := "healthy"
	for name, comp := range components {
		if name == "worker" {
			continue
		}
		if comp.Status == "unhealthy" {
			overall = "degraded"
			break
		}
	}

	ttlSec := 0
	if h.MetricsCacheTTL > 0 {
		ttlSec = int(h.MetricsCacheTTL.Seconds())
	}

	warnings := buildWarnings(hostStats, components["storage"])
	if overall == "healthy" {
		for _, w := range warnings {
			if w.Level == "critical" {
				overall = "degraded"
				break
			}
		}
	}

	c.JSON(http.StatusOK, systemHealthResponse{
		Status:                 overall,
		Timestamp:              time.Now().UTC(),
		APIUptimeSeconds:       time.Since(apiStartedAt).Seconds(),
		GoVersion:              runtime.Version(),
		NumGoroutine:           runtime.NumGoroutine(),
		MetricsCacheTTLSeconds: ttlSec,
		MetricsCachedAt:        metricsCachedAt,
		UploadMetrics:          upload.Default.Snapshot(),
		Host:                   hostStats,
		ResourceSnapshot:       resSnap,
		ResourceLimits:         resLimits,
		QueueDepth:             queueDepth,
		QueueMaxDepth:          queueMax,
		Warnings:               warnings,
		Components:             components,
	})
}

func buildWarnings(host *observability.HostStats, storage componentHealth) []HealthWarning {
	var out []HealthWarning

	if host != nil {
		if lvl := capacity.UsageLevel(host.Memory.UsedPercent); lvl != "" {
			msg := "RAM máy chủ đang cao — hệ thống sẽ giảm tải khi vượt 90%."
			if lvl == "critical" {
				msg = "RAM máy chủ vượt 90% — đã giảm convert và tải nền."
			}
			out = append(out, HealthWarning{Level: lvl, Code: "host_ram_high", Message: msg})
		}
		if lvl := capacity.UsageLevel(host.CPUPercent); lvl != "" {
			msg := "CPU máy chủ đang cao — theo dõi thêm hoặc giảm CONVERT_MAX_CONCURRENT."
			if lvl == "critical" {
				msg = "CPU máy chủ vượt 90% — đã giảm convert và tải nền."
			}
			out = append(out, HealthWarning{Level: lvl, Code: "host_cpu_high", Message: msg})
		}
		for _, d := range host.Disks {
			if lvl := capacity.UsageLevel(d.UsedPercent); lvl != "" {
				msg := fmt.Sprintf("Ổ đĩa %s đang cao (%.1f%%) — dọn dẹp hoặc mở rộng dung lượng.", d.Path, d.UsedPercent)
				if lvl == "critical" {
					msg = fmt.Sprintf("Ổ đĩa %s vượt 90%% — nguy cơ lỗi ghi file/convert.", d.Path)
				}
				out = append(out, HealthWarning{Level: lvl, Code: "host_disk_high", Message: msg})
			}
		}
	}

	if storage.Details != nil {
		if raw := storage.Details["used_percent"]; raw != "" {
			if pct, err := strconv.ParseFloat(raw, 64); err == nil {
				if lvl := capacity.UsageLevel(pct); lvl != "" {
					bucket := storage.Details["bucket"]
					msg := "Object storage (MinIO) đang cao — cân nhắc dọn media cũ."
					if bucket != "" {
						msg = fmt.Sprintf("Bucket %s đang cao (%.1f%%).", bucket, pct)
					}
					if lvl == "critical" {
						msg = "Object storage gần đầy — upload/convert có thể thất bại."
					}
					out = append(out, HealthWarning{Level: lvl, Code: "storage_disk_high", Message: msg})
				}
			}
		}
	}

	return out
}

func (h *HealthHandler) cachedHostStats(ctx context.Context) (*observability.HostStats, error) {
	if h.HostCache == nil {
		return observability.CollectHostStats(ctx)
	}
	return h.HostCache.GetOrCompute(ctx, observability.CollectHostStats)
}

func (h *HealthHandler) checkDB(ctx context.Context) componentHealth {
	if h.DB == nil {
		return componentHealth{Status: "unhealthy", Error: "not configured"}
	}
	if err := h.DB.Ping(ctx); err != nil {
		return componentHealth{Status: "unhealthy", Error: err.Error()}
	}
	stat := h.DB.Stat()
	return componentHealth{
		Status: "healthy",
		Details: map[string]string{
			"acquired_conns": strconv.Itoa(int(stat.AcquiredConns())),
			"idle_conns":     strconv.Itoa(int(stat.IdleConns())),
			"max_conns":      strconv.Itoa(int(stat.MaxConns())),
			"total_conns":    strconv.Itoa(int(stat.TotalConns())),
		},
	}
}

func (h *HealthHandler) checkRedis(ctx context.Context) componentHealth {
	if h.Redis == nil {
		return componentHealth{Status: "unhealthy", Error: "not configured"}
	}
	if err := h.Redis.Ping(ctx).Err(); err != nil {
		return componentHealth{Status: "unhealthy", Error: err.Error()}
	}
	info, err := h.Redis.Info(ctx, "memory", "clients").Result()
	details := map[string]string{}
	if err == nil {
		for _, line := range strings.Split(info, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "used_memory_human:") {
				details["used_memory_human"] = strings.TrimPrefix(line, "used_memory_human:")
			}
			if strings.HasPrefix(line, "used_memory:") {
				details["used_memory_bytes"] = strings.TrimPrefix(line, "used_memory:")
			}
			if strings.HasPrefix(line, "connected_clients:") {
				details["connected_clients"] = strings.TrimPrefix(line, "connected_clients:")
			}
		}
	}
	return componentHealth{Status: "healthy", Details: details}
}

func (h *HealthHandler) checkStorage(ctx context.Context) componentHealth {
	if h.Storage == nil {
		return componentHealth{Status: "unhealthy", Error: "not configured"}
	}
	if err := h.Storage.Ping(ctx); err != nil {
		return componentHealth{Status: "unhealthy", Error: err.Error()}
	}

	statsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	stats, err := h.Storage.Stats(statsCtx)
	if err != nil {
		return componentHealth{
			Status: "healthy",
			Details: map[string]string{
				"note": "connected; usage stats unavailable: " + err.Error(),
			},
		}
	}

	var cachedAt time.Time
	var cacheTTL time.Duration
	if cs, ok := h.Storage.(*storage.S3Storage); ok {
		cachedAt = cs.StatsCacheAt()
		cacheTTL = cs.StatsCacheTTL()
	}

	return componentHealth{
		Status:  "healthy",
		Details: storage.StatsDetails(stats, cachedAt, cacheTTL),
	}
}

func (h *HealthHandler) checkWorker(ctx context.Context) componentHealth {
	if h.Redis == nil {
		return componentHealth{Status: "unknown", Details: map[string]string{"note": "redis not configured"}}
	}
	return CheckWorkerHeartbeat(ctx, h.Redis)
}

func (h *HealthHandler) checkFFmpeg() componentHealth {
	path := os.Getenv("FFMPEG_PATH")
	if path == "" {
		path = "/usr/bin/ffmpeg"
	}
	if _, err := os.Stat(path); err != nil {
		return componentHealth{Status: "unhealthy", Error: "ffmpeg not found"}
	}
	return componentHealth{
		Status: "healthy",
		Details: map[string]string{"path": path},
	}
}

func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
