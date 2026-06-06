package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type IntegrationV1Handler struct {
	Integration *service.IntegrationService
}

func NewIntegrationV1Handler(integration *service.IntegrationService) *IntegrationV1Handler {
	return &IntegrationV1Handler{Integration: integration}
}

type v1UploadInitRequest struct {
	ParentID   *string `json:"parent_id"`
	FileName   string  `json:"file_name" binding:"required"`
	Size       int64   `json:"size" binding:"required"`
	MimeType   string  `json:"mime_type"`
	ChunkSize  int     `json:"chunk_size"`
	UploadMode string  `json:"upload_mode"`
}

func (h *IntegrationV1Handler) UploadLimits(c *gin.Context) {
	out, err := h.Integration.UploadLimits(c.Request.Context())
	if err != nil {
		writeIntegrationError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *IntegrationV1Handler) UploadInit(c *gin.Context) {
	key, ok := middleware.GetAPIKey(c)
	if !ok {
		return
	}
	var req v1UploadInitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}
	var parentPID *uuid.UUID
	if req.ParentID != nil && strings.TrimSpace(*req.ParentID) != "" {
		pid, err := uuid.Parse(strings.TrimSpace(*req.ParentID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid parent_id"})
			return
		}
		parentPID = &pid
	}
	out, err := h.Integration.InitUpload(c.Request.Context(), key, service.InitUploadInput{
		FileName:  req.FileName,
		Size:      req.Size,
		MimeType:  req.MimeType,
		ChunkSize: req.ChunkSize,
		Mode:      req.UploadMode,
	}, parentPID)
	if err != nil {
		writeIntegrationError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *IntegrationV1Handler) UploadChunk(c *gin.Context) {
	key, ok := middleware.GetAPIKey(c)
	if !ok {
		return
	}
	sessionID, err := uuid.Parse(c.Param("session_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid session_id"})
		return
	}
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid chunk index"})
		return
	}
	sizeHeader := c.GetHeader("Content-Length")
	size, _ := strconv.ParseInt(sizeHeader, 10, 64)
	if err := h.Integration.PutChunk(c.Request.Context(), key, sessionID, index, c.Request.Body, size); err != nil {
		writeIntegrationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *IntegrationV1Handler) UploadComplete(c *gin.Context) {
	key, ok := middleware.GetAPIKey(c)
	if !ok {
		return
	}
	sessionID, err := uuid.Parse(c.Param("session_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid session_id"})
		return
	}
	out, err := h.Integration.CompleteUpload(c.Request.Context(), key, sessionID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeIntegrationError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *IntegrationV1Handler) UploadAbort(c *gin.Context) {
	key, ok := middleware.GetAPIKey(c)
	if !ok {
		return
	}
	sessionID, err := uuid.Parse(c.Param("session_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid session_id"})
		return
	}
	if err := h.Integration.AbortUpload(c.Request.Context(), key, sessionID); err != nil {
		writeIntegrationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *IntegrationV1Handler) GetMedia(c *gin.Context) {
	key, ok := middleware.GetAPIKey(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	out, err := h.Integration.GetMedia(c.Request.Context(), key, pid)
	if err != nil {
		writeIntegrationError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *IntegrationV1Handler) DeleteMedia(c *gin.Context) {
	key, ok := middleware.GetAPIKey(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	out, err := h.Integration.DeleteObject(c.Request.Context(), key, pid, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeIntegrationError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type v1ConvertRequest struct {
	Variants []string `json:"variants"`
}

func (h *IntegrationV1Handler) Convert(c *gin.Context) {
	key, ok := middleware.GetAPIKey(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	var req v1ConvertRequest
	_ = c.ShouldBindJSON(&req)
	job, err := h.Integration.StartConvert(c.Request.Context(), key, pid, service.StartConvertInput{Variants: req.Variants}, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeIntegrationError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"job": job})
}

func (h *IntegrationV1Handler) RetryConvert(c *gin.Context) {
	key, ok := middleware.GetAPIKey(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	var req v1ConvertRequest
	_ = c.ShouldBindJSON(&req)
	job, err := h.Integration.RetryConvert(c.Request.Context(), key, pid, service.StartConvertInput{Variants: req.Variants}, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeIntegrationError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"job": job})
}

func (h *IntegrationV1Handler) GetHLS(c *gin.Context) {
	key, ok := middleware.GetAPIKey(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	out, err := h.Integration.GetHLSAccess(c.Request.Context(), key, pid)
	if err != nil {
		writeIntegrationError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type v1DeliveryURLsRequest struct {
	IDs []string `json:"ids" binding:"required"`
}

func (h *IntegrationV1Handler) DeliveryURLs(c *gin.Context) {
	key, ok := middleware.GetAPIKey(c)
	if !ok {
		return
	}
	var req v1DeliveryURLsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}
	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, raw := range req.IDs {
		pid, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid id in list"})
			return
		}
		ids = append(ids, pid)
	}
	urls, err := h.Integration.DeliveryURLs(c.Request.Context(), key, ids)
	if err != nil {
		writeIntegrationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"urls": urls})
}

func writeIntegrationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrIntegrationAccessDenied),
		errors.Is(err, service.ErrMediaAccessDenied),
		errors.Is(err, service.ErrVideoAccessDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": err.Error()})
	case errors.Is(err, service.ErrIntegrationInvalidParent),
		errors.Is(err, service.ErrMediaInvalidParent):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
	case errors.Is(err, repository.ErrMediaObjectNotFound),
		errors.Is(err, service.ErrVideoNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": err.Error()})
	case errors.Is(err, service.ErrUploadTooLarge),
		errors.Is(err, service.ErrUploadInvalidChunk),
		errors.Is(err, service.ErrUploadIncomplete):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
	case errors.Is(err, service.ErrUploadRateLimited),
		errors.Is(err, service.ErrIntegrationQuotaExceeded):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited", "message": err.Error()})
	case errors.Is(err, service.ErrVideoInvalidState),
		errors.Is(err, service.ErrVideoConvertActive),
		errors.Is(err, service.ErrVideoRetryNotAllowed):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "message": err.Error()})
	case errors.Is(err, service.ErrSystemBusy):
		WriteSystemBusy(c)
	case errors.Is(err, service.ErrConvertQueueFull):
		WriteQueueFull(c)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
	}
}
