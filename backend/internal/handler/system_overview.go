package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/anhtuanlc/mediahub/internal/observability"
	"github.com/anhtuanlc/mediahub/internal/platform/upload"
	"github.com/gin-gonic/gin"
)

const overviewJobLimit = 5

func sumVideosActive(hls map[string]int64) int64 {
	if hls == nil {
		return 0
	}
	var total int64
	for k, v := range hls {
		if k == "deleted" {
			continue
		}
		total += v
	}
	return total
}

func compactComponents(components map[string]componentHealth) gin.H {
	out := make(gin.H, len(components))
	for name, comp := range components {
		item := gin.H{"status": comp.Status}
		if comp.Error != "" {
			item["error"] = comp.Error
		}
		out[name] = item
	}
	return out
}

func compactHost(host *observability.HostStats) gin.H {
	if host == nil {
		return nil
	}
	disks := make([]gin.H, 0, len(host.Disks))
	for _, d := range host.Disks {
		disks = append(disks, gin.H{
			"path":         d.Path,
			"used_percent": d.UsedPercent,
		})
	}
	return gin.H{
		"cpu_percent": host.CPUPercent,
		"memory": gin.H{
			"used_percent": host.Memory.UsedPercent,
		},
		"disks": disks,
	}
}

func overallStatus(components map[string]componentHealth, host *observability.HostStats, storage componentHealth) string {
	overall := "healthy"
	for name, comp := range components {
		if name == "worker" {
			continue
		}
		if comp.Status == "unhealthy" {
			return "degraded"
		}
	}
	warnings := buildWarnings(host, storage)
	for _, w := range warnings {
		if w.Level == "critical" {
			return "degraded"
		}
	}
	return overall
}

func (h *SystemInfoHandler) buildStorageOverview(ctx context.Context) gin.H {
	if h.Health == nil || h.Health.Storage == nil {
		return gin.H{"status": "unhealthy", "error": "not configured"}
	}
	comp := h.Health.checkStorage(ctx)
	out := gin.H{"status": comp.Status}
	if comp.Error != "" {
		out["error"] = comp.Error
	}
	if comp.Details != nil {
		if comp.Details["stats_partial"] == "true" {
			out["stats_partial"] = true
		}
		for _, key := range []string{"used_bytes", "total_bytes", "used_percent", "object_count"} {
			if v := comp.Details[key]; v != "" {
				out[key] = v
			}
		}
	}

	quotaBytes := int64(0)
	if h.Settings != nil {
		if settings, err := h.Settings.Get(ctx); err == nil {
			quotaBytes = settings.Editable.Storage.QuotaBytes
		}
	}
	if quotaBytes <= 0 && h.Cfg != nil {
		quotaBytes = h.Cfg.Storage.QuotaBytes
	}
	if quotaBytes > 0 {
		out["quota_bytes"] = quotaBytes
		if comp.Details != nil {
			if usedStr := comp.Details["used_bytes"]; usedStr != "" {
				if used, err := strconv.ParseInt(usedStr, 10, 64); err == nil {
					remaining := quotaBytes - used
					if remaining < 0 {
						remaining = 0
					}
					out["quota_remaining_bytes"] = remaining
				}
			}
		}
	}
	return out
}

// Overview aggregates dashboard metrics in a single owner-only response.
func (h *SystemInfoHandler) Overview(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	if h.Health == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}

	hostStats, _ := h.Health.cachedHostStats(ctx)
	components := map[string]componentHealth{
		"database": h.Health.checkDB(ctx),
		"redis":    h.Health.checkRedis(ctx),
		"storage":  h.Health.checkStorage(ctx),
		"worker":   h.Health.checkWorker(ctx),
		"ffmpeg":   h.Health.checkFFmpeg(),
	}
	storageComp := components["storage"]

	resp := gin.H{
		"generated_at":   time.Now().UTC().Format(time.RFC3339),
		"status":         overallStatus(components, hostStats, storageComp),
		"warnings":       buildWarnings(hostStats, storageComp),
		"host":           compactHost(hostStats),
		"components":     compactComponents(components),
		"upload_metrics": upload.Default.Snapshot(),
		"storage":        h.buildStorageOverview(ctx),
	}

	if h.Health.Resources != nil && h.Health.Resources.Enabled {
		if _, lim, err := h.Health.Resources.Current(ctx); err == nil {
			resp["resource_limits"] = lim
		}
	}

	errors := gin.H{}
	partial := false

	if h.Queue != nil {
		snap, err := h.Queue.OverviewSnapshot(ctx, overviewJobLimit)
		if err != nil {
			partial = true
			errors["queue"] = err.Error()
		} else {
			resp["content"] = snap.Content
			resp["queue"] = snap.Queue
		}

		if h.Queue.Metrics != nil {
			stream := gin.H{"date": time.Now().UTC().Format("2006-01-02")}
			if analytics, err := h.Queue.Metrics.AnalyticsSnapshot(ctx); err != nil {
				partial = true
				errors["stream"] = err.Error()
			} else {
				for k, v := range analytics {
					stream[k] = v
				}
				resp["stream"] = stream
			}
		}
	}

	if partial {
		resp["partial"] = true
		resp["errors"] = errors
	}

	c.JSON(http.StatusOK, resp)
}
