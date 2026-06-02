package handler

import (
	"context"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/platform"
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
	HostCache       *platform.TTLCache[*platform.HostStats]
}

type componentHealth struct {
	Status  string            `json:"status"`
	Error   string            `json:"error,omitempty"`
	Details map[string]string `json:"details,omitempty"`
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
	Host                    *platform.HostStats        `json:"host,omitempty"`
	Components              map[string]componentHealth `json:"components"`
}

func (h *HealthHandler) SystemHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	hostStats, _ := h.cachedHostStats(ctx)

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
		"worker":   {Status: "unknown", Details: map[string]string{"note": "worker heartbeat not configured"}},
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

	c.JSON(http.StatusOK, systemHealthResponse{
		Status:                 overall,
		Timestamp:              time.Now().UTC(),
		APIUptimeSeconds:       time.Since(apiStartedAt).Seconds(),
		GoVersion:              runtime.Version(),
		NumGoroutine:           runtime.NumGoroutine(),
		MetricsCacheTTLSeconds: ttlSec,
		MetricsCachedAt:        metricsCachedAt,
		UploadMetrics:          platform.Upload.Snapshot(),
		Host:                   hostStats,
		Components:             components,
	})
}

func (h *HealthHandler) cachedHostStats(ctx context.Context) (*platform.HostStats, error) {
	if h.HostCache == nil {
		return platform.CollectHostStats(ctx)
	}
	return h.HostCache.GetOrCompute(ctx, platform.CollectHostStats)
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
