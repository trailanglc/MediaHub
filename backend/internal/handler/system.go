package handler

import (
	"net/http"

	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
)

type SystemHandler struct {
	maintenance *service.MaintenanceService
}

func NewSystemHandler(maintenance *service.MaintenanceService) *SystemHandler {
	return &SystemHandler{maintenance: maintenance}
}

type cleanupOrphansRequest struct {
	DryRun    *bool    `json:"dry_run"`
	MaxDelete *int     `json:"max_delete"`
	Prefixes  []string `json:"prefixes"`
}

func (h *SystemHandler) CleanupTemp(c *gin.Context) {
	if h.maintenance == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable", "message": "maintenance not configured"})
		return
	}
	actor, ok := middleware.GetAuthUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "authentication required"})
		return
	}
	res, err := h.maintenance.CleanupTempWithAudit(c.Request.Context(), actor.ID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *SystemHandler) CleanupOrphans(c *gin.Context) {
	if h.maintenance == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable", "message": "maintenance not configured"})
		return
	}
	actor, ok := middleware.GetAuthUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "authentication required"})
		return
	}

	dryRun := true
	if v := c.Query("dry_run"); v == "false" || v == "0" {
		dryRun = false
	}

	var req cleanupOrphansRequest
	_ = c.ShouldBindJSON(&req)
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}

	in := service.CleanupOrphansInput{DryRun: dryRun, Prefixes: req.Prefixes}
	if req.MaxDelete != nil {
		in.MaxDelete = *req.MaxDelete
	}

	res, err := h.maintenance.CleanupOrphansWithAudit(c.Request.Context(), in, actor.ID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}
