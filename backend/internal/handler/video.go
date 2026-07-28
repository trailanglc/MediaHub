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
	in := service.ListVideosInput{
		HLSStatus: statuses,
		Query:     c.Query("q"),
		Cursor:    cursor,
		Limit:     limit,
	}
	catRaw := strings.TrimSpace(c.Query("category"))
	switch {
	case catRaw == "" || catRaw == "uncategorized":
		in.CategoryFilter = "uncategorized"
	case catRaw == "all":
		in.CategoryFilter = "all"
	default:
		pid, err := uuid.Parse(catRaw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid category"})
			return
		}
		in.CategoryID = &pid
	}
	items, next, err := h.videos.List(c.Request.Context(), u.ID, u.Role, in)
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

func (h *VideoHandler) CancelConvert(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	err = h.videos.CancelConvert(c.Request.Context(), u.ID, u.Role, c.ClientIP(), c.Request.UserAgent(), pid)
	if err != nil {
		writeVideoError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
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

type videoCategoryPatch struct {
	CategoryPublicID *string `json:"category_public_id"`
}

func (h *VideoHandler) PatchCategory(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	var req videoCategoryPatch
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid body"})
		return
	}
	in := service.SetVideoCategoryInput{}
	if req.CategoryPublicID != nil && strings.TrimSpace(*req.CategoryPublicID) != "" {
		cid, err := uuid.Parse(strings.TrimSpace(*req.CategoryPublicID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid category_public_id"})
			return
		}
		in.CategoryPublicID = &cid
	}
	dto, err := h.videos.SetCategory(c.Request.Context(), u.ID, u.Role, pid, in)
	if err != nil {
		writeVideoError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto)
}

func writeVideoError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrVideoAccessDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "insufficient permissions"})
	case errors.Is(err, service.ErrVideoNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "video not found"})
	case errors.Is(err, service.ErrVideoCategoryNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "category not found"})
	case errors.Is(err, service.ErrVideoInvalidState):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "message": err.Error()})
	case errors.Is(err, service.ErrVideoConvertActive):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "message": "convert already in progress"})
	case errors.Is(err, service.ErrVideoConvertNotActive):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "message": "no active convert job"})
	case errors.Is(err, service.ErrVideoRetryNotAllowed):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "message": err.Error()})
	case errors.Is(err, service.ErrSystemBusy):
		WriteSystemBusy(c)
	case errors.Is(err, service.ErrConvertQueueFull):
		WriteQueueFull(c)
	case errors.Is(err, service.ErrConvertQueuePaused):
		WriteQueuePaused(c)
	case errors.Is(err, service.ErrVideoInvalidVariants):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
	case errors.Is(err, service.ErrVideoNotStreamable):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "message": "video is not ready for streaming"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "operation failed"})
	}
}
