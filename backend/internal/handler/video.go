package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type VideoHandler struct {
	videos       *service.VideoService
	deletePrefix func(ctx context.Context, prefix string) error
}

func NewVideoHandler(videos *service.VideoService, s3 *storage.S3Storage) *VideoHandler {
	h := &VideoHandler{videos: videos}
	if s3 != nil {
		h.deletePrefix = s3.DeletePrefix
	}
	return h
}

func (h *VideoHandler) List(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	var statuses []string
	if raw := strings.TrimSpace(c.Query("hls_status")); raw != "" {
		statuses = strings.Split(raw, ",")
	}
	cursor, _ := strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, next, err := h.videos.List(c.Request.Context(), u.ID, u.Role, service.ListVideosInput{
		HLSStatus: statuses,
		Query:     c.Query("q"),
		Cursor:    cursor,
		Limit:     limit,
	})
	if err != nil {
		writeVideoError(c, err)
		return
	}
	resp := gin.H{"items": items}
	if next != nil {
		resp["next_cursor"] = *next
	}
	c.JSON(http.StatusOK, resp)
}

func (h *VideoHandler) Get(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	dto, err := h.videos.Get(c.Request.Context(), u.ID, u.Role, pid)
	if err != nil {
		writeVideoError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto)
}

func (h *VideoHandler) Convert(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	var req service.StartConvertInput
	_ = c.ShouldBindJSON(&req)
	job, err := h.videos.StartConvert(c.Request.Context(), u.ID, u.Role, c.ClientIP(), c.Request.UserAgent(), pid, req)
	if err != nil {
		writeVideoError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"job": job})
}

func (h *VideoHandler) RetryConvert(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	var req service.StartConvertInput
	_ = c.ShouldBindJSON(&req)
	job, err := h.videos.RetryConvert(c.Request.Context(), u.ID, u.Role, c.ClientIP(), c.Request.UserAgent(), pid, req)
	if err != nil {
		writeVideoError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"job": job})
}

func (h *VideoHandler) GetHLS(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	access, err := h.videos.GetHLSAccess(c.Request.Context(), u.ID, u.Role, pid)
	if err != nil {
		writeVideoError(c, err)
		return
	}
	c.JSON(http.StatusOK, access)
}

func (h *VideoHandler) DeleteHLS(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	err = h.videos.DeleteHLS(c.Request.Context(), u.ID, u.Role, c.ClientIP(), c.Request.UserAgent(), pid, h.deletePrefix)
	if err != nil {
		writeVideoError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type streamPolicyPatch struct {
	AccessMode      *string   `json:"access_mode"`
	AllowedDomains  *[]string `json:"allowed_domains"`
	TokenTTLSeconds *int      `json:"token_ttl_seconds"`
	AllowDownload   *bool     `json:"allow_download"`
}

func (h *VideoHandler) GetStreamPolicy(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	pol, err := h.videos.GetStreamPolicy(c.Request.Context(), u.ID, u.Role, pid)
	if err != nil {
		writeVideoError(c, err)
		return
	}
	c.JSON(http.StatusOK, pol)
}

func (h *VideoHandler) PatchStreamPolicy(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	var req streamPolicyPatch
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid body"})
		return
	}
	pol, err := h.videos.UpdateStreamPolicy(c.Request.Context(), u.ID, u.Role, c.ClientIP(), c.Request.UserAgent(), pid, service.UpdateStreamPolicyInput{
		AccessMode:      req.AccessMode,
		AllowedDomains:  req.AllowedDomains,
		TokenTTLSeconds: req.TokenTTLSeconds,
		AllowDownload:   req.AllowDownload,
	})
	if err != nil {
		writeVideoError(c, err)
		return
	}
	c.JSON(http.StatusOK, pol)
}

func writeVideoError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrVideoAccessDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "insufficient permissions"})
	case errors.Is(err, service.ErrVideoNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "video not found"})
	case errors.Is(err, service.ErrVideoInvalidState):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "message": err.Error()})
	case errors.Is(err, service.ErrVideoConvertActive):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "message": "convert already in progress"})
	case errors.Is(err, service.ErrVideoRetryNotAllowed):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "message": err.Error()})
	case errors.Is(err, service.ErrSystemBusy):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "system_busy", "message": "hệ thống đang quá tải, thử lại sau"})
	case errors.Is(err, service.ErrVideoInvalidVariants):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
	case errors.Is(err, service.ErrVideoNotStreamable):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "message": "video is not ready for streaming"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "operation failed"})
	}
}
