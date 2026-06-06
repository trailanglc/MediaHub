package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/anhtuanlc/mediahub/internal/middleware"
	"github.com/anhtuanlc/mediahub/internal/platform/upload"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

type UploadHandler struct {
	uploads *service.UploadService
	logger  *zap.Logger
}

func NewUploadHandler(uploads *service.UploadService, logger *zap.Logger) *UploadHandler {
	return &UploadHandler{uploads: uploads, logger: logger}
}

type initUploadRequest struct {
	ParentID  string `json:"parent_id" binding:"required"`
	FileName  string `json:"file_name" binding:"required"`
	Size      int64  `json:"size" binding:"required"`
	MimeType  string `json:"mime_type"`
	ChunkSize int    `json:"chunk_size"`
	Mode      string `json:"upload_mode"`
}

func (h *UploadHandler) Limits(c *gin.Context) {
	if _, ok := middleware.GetAuthUser(c); !ok {
		return
	}
	out, err := h.uploads.Limits(c.Request.Context())
	if err != nil {
		writeUploadError(c, h.logger, "upload limits", err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *UploadHandler) Init(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	var req initUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}
	parentPID, err := uuid.Parse(req.ParentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid parent_id"})
		return
	}
	out, err := h.uploads.Init(c.Request.Context(), u.ID, u.Role, service.InitUploadInput{
		ParentPublicID: parentPID,
		FileName:       req.FileName,
		Size:           req.Size,
		MimeType:       req.MimeType,
		ChunkSize:      req.ChunkSize,
		Mode:           req.Mode,
	})
	if err != nil {
		writeUploadError(c, h.logger, "upload init", err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *UploadHandler) PutChunk(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	sessionPID, err := uuid.Parse(c.Param("session_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid session_id"})
		return
	}
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil || index < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid chunk index"})
		return
	}

	size := c.Request.ContentLength
	if size < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "content-length required"})
		return
	}

	if err := h.uploads.PutChunk(c.Request.Context(), u.ID, sessionPID, index, io.LimitReader(c.Request.Body, size), size); err != nil {
		writeUploadError(c, h.logger, "upload chunk", err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *UploadHandler) Complete(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	sessionPID, err := uuid.Parse(c.Param("session_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid session_id"})
		return
	}
	obj, err := h.uploads.Complete(c.Request.Context(), u.ID, u.Role, sessionPID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		writeUploadError(c, h.logger, "upload complete", err)
		return
	}
	c.JSON(http.StatusCreated, obj)
}

func (h *UploadHandler) Abort(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	sessionPID, err := uuid.Parse(c.Param("session_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid session_id"})
		return
	}
	if err := h.uploads.Abort(c.Request.Context(), u.ID, sessionPID); err != nil {
		writeUploadError(c, h.logger, "upload abort", err)
		return
	}
	c.Status(http.StatusNoContent)
}

func writeUploadError(c *gin.Context, logger *zap.Logger, op string, err error) {
	if logger != nil {
		logger.Error(op, zap.Error(err))
	}

	if msg := uploadSchemaErrorMessage(err); msg != "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service_unavailable",
			"message": msg,
		})
		return
	}

	upload.Default.FailTotal.Add(1)
	switch {
	case errors.Is(err, service.ErrUploadRateLimited):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited", "message": "quá nhiều lần bắt đầu upload, thử lại sau"})
	case errors.Is(err, service.ErrUploadTooManyPending):
		c.JSON(http.StatusConflict, gin.H{
			"error":   "conflict",
			"message": "quá nhiều upload đang chạy song song — hủy upload cũ hoặc đợi vài phút rồi thử lại",
		})
	case errors.Is(err, service.ErrUploadTooLarge):
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error":   "payload_too_large",
			"message": "file vượt quá kích thước upload tối đa (Settings → Media)",
		})
	case errors.Is(err, service.ErrMediaAccessDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "insufficient permissions"})
	case errors.Is(err, repository.ErrUploadSessionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "upload session not found"})
	case errors.Is(err, service.ErrUploadNotOwner):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "upload session not owned by user"})
	case errors.Is(err, service.ErrUploadSessionDone), errors.Is(err, service.ErrUploadSessionExpired):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "message": err.Error()})
	case errors.Is(err, service.ErrUploadInvalidChunk):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
	case errors.Is(err, service.ErrUploadIncomplete):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "upload missing one or more chunks"})
	case errors.Is(err, service.ErrUploadMultipart):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict", "message": "upload session outdated — hủy và upload lại sau migrate 000007"})
	case errors.Is(err, service.ErrStorageQuotaExceeded):
		WriteStorageQuotaExceeded(c)
	case errors.Is(err, repository.ErrMediaObjectNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "parent folder not found"})
	case errors.Is(err, service.ErrMediaInvalidParent):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid parent folder"})
	default:
		msg := "upload failed"
		if strings.Contains(err.Error(), "not seekable") {
			msg = "upload stream error — khởi động lại API sau khi cập nhật code"
		} else if strings.Contains(err.Error(), "put object") || strings.Contains(err.Error(), "get object") {
			msg = "object storage error — kiểm tra MinIO (make infra-up từ thư mục gốc repo) và STORAGE_* trong backend/.env"
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": msg,
		})
	}
}

func uploadSchemaErrorMessage(err error) string {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return ""
	}
	if pgErr.Code != "42P01" {
		return ""
	}
	if strings.Contains(pgErr.Message, "upload_sessions") {
		return "Database chưa có bảng upload_sessions. Chạy: make migrate-up"
	}
	return "Database schema chưa đầy đủ. Chạy: make migrate-up"
}
