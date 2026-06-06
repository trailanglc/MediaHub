package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
)

// SystemInfoHandler exposes owner-only operational summaries.
type SystemInfoHandler struct {
	Health   *HealthHandler
	Queue    *QueueHandler
	APIKeys  *repository.APIKeyRepository
	Settings *service.SettingsService
	Cfg      *config.Config
}

func (h *SystemInfoHandler) Storage(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	if h.Health == nil || h.Health.Storage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service_unavailable"})
		return
	}
	comp := h.Health.checkStorage(ctx)
	out := gin.H{
		"status":  comp.Status,
		"details": comp.Details,
	}
	if comp.Error != "" {
		out["error"] = comp.Error
	}
	if comp.Details != nil {
		if comp.Details["stats_partial"] == "true" {
			out["stats_partial"] = true
		}
	}
	if h.Cfg != nil {
		out["driver"] = h.Cfg.Storage.Driver
		out["bucket"] = h.Cfg.Storage.Bucket
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
	c.JSON(http.StatusOK, out)
}

func (h *SystemInfoHandler) Security(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	activeKeys := 0
	if h.APIKeys != nil {
		keys, err := h.APIKeys.List(ctx)
		if err == nil {
			for _, k := range keys {
				if k.Status == "active" {
					activeKeys++
				}
			}
		}
	}

	globalDomains := []string(nil)
	if h.Settings != nil {
		if domains, err := h.Settings.GlobalAllowedDomains(ctx); err == nil {
			globalDomains = domains
		}
	}

	resp := gin.H{
		"login_max_attempts":        5,
		"login_lockout_window_sec":  int((15 * time.Minute).Seconds()),
		"redis_fail_closed":         false,
		"stream_rate_limit_per_min": 120,
		"active_api_keys":           activeKeys,
		"global_stream_domains":     globalDomains,
		"password_transport": gin.H{
			"require_encrypted": false,
		},
		"cookie_secure": false,
	}
	if h.Cfg != nil {
		resp["login_max_attempts"] = h.Cfg.LoginMaxAttempts
		resp["login_lockout_window_sec"] = int(h.Cfg.LoginLockoutWindow.Seconds())
		resp["redis_fail_closed"] = h.Cfg.RedisFailClosed
		resp["stream_rate_limit_per_min"] = h.Cfg.StreamRateLimitPerMin
		resp["app_env"] = h.Cfg.AppEnv
		resp["password_transport"] = gin.H{
			"require_encrypted": h.Cfg.RequireEncryptedPassword,
		}
		resp["cookie_secure"] = h.Cfg.AppEnv == "production" || h.Cfg.AppEnv == "staging"
	}
	c.JSON(http.StatusOK, resp)
}

// ComponentHealth returns a single health component (database, redis, storage, worker, ffmpeg).
func (h *HealthHandler) ComponentHealth(c *gin.Context) {
	name := c.Param("component")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	var comp componentHealth
	switch name {
	case "database":
		comp = h.checkDB(ctx)
	case "redis":
		comp = h.checkRedis(ctx)
	case "storage":
		comp = h.checkStorage(ctx)
	case "worker":
		comp = h.checkWorker(ctx)
	case "ffmpeg":
		comp = h.checkFFmpeg()
	default:
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "unknown component"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"component": name, "health": comp})
}