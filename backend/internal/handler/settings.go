package handler

import (
	"errors"
	"net/http"

	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type SettingsHandler struct {
	settings *service.SettingsService
	logger   *zap.Logger
}

func NewSettingsHandler(settings *service.SettingsService, logger *zap.Logger) *SettingsHandler {
	return &SettingsHandler{settings: settings, logger: logger}
}

func (h *SettingsHandler) Get(c *gin.Context) {
	resp, err := h.settings.Get(c.Request.Context())
	if err != nil {
		if h.logger != nil {
			h.logger.Error("settings get", zap.Error(err))
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "failed to load settings",
		})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *SettingsHandler) Update(c *gin.Context) {
	var patch service.SettingsPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}
	actor, ok := middleware.GetAuthUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "authentication required"})
		return
	}
	resp, err := h.settings.Update(c.Request.Context(), patch, actor.ID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		if errors.Is(err, service.ErrNoSettingsToUpdate) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "no settings to update"})
			return
		}
		if errors.Is(err, service.ErrInvalidSettings) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
			return
		}
		if errors.Is(err, repository.ErrSettingsTableMissing) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "migration_required",
				"message": err.Error(),
			})
			return
		}
		if h.logger != nil {
			h.logger.Error("settings update", zap.Error(err))
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "failed to update settings"})
		return
	}
	c.JSON(http.StatusOK, resp)
}
