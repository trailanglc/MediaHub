package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/anhtuanlc/mediahub/internal/middleware"
	downloadprog "github.com/anhtuanlc/mediahub/internal/platform/download"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DownloadHandler struct {
	downloads *service.DownloadService
}

func NewDownloadHandler(downloads *service.DownloadService) *DownloadHandler {
	return &DownloadHandler{downloads: downloads}
}

func (h *DownloadHandler) Analyze(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	_ = u
	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "url required"})
		return
	}
	out, err := h.downloads.Analyze(c.Request.Context(), req.URL)
	if err != nil {
		writeDownloadError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *DownloadHandler) CreateJob(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	var req service.CreateDownloadJobInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid body"})
		return
	}
	dto, err := h.downloads.CreateJob(c.Request.Context(), u.ID, u.Role, c.ClientIP(), c.Request.UserAgent(), req)
	if err != nil {
		writeDownloadError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, dto)
}

func (h *DownloadHandler) ListJobs(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	cursor, _ := strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, next, err := h.downloads.List(c.Request.Context(), u.ID, cursor, limit)
	if err != nil {
		writeDownloadError(c, err)
		return
	}
	resp := gin.H{"items": items}
	if next != nil {
		resp["next_cursor"] = *next
	}
	c.JSON(http.StatusOK, resp)
}

// StreamJobs pushes download job updates via Server-Sent Events (Redis pub/sub).
func (h *DownloadHandler) StreamJobs(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	store := h.downloads.ProgressStore()
	if store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "sse_unavailable",
			"message": "progress store not configured",
		})
		return
	}

	// Clear write deadline so long-lived SSE is not killed by http.Server WriteTimeout.
	if rc := http.NewResponseController(c.Writer); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.Writer.Flush()

	writeEvent := func(event string, payload any) error {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, b); err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	}

	// Subscribe first, then snapshot, so progress published during List is not missed
	// (client merges without regressing). Disconnect never cancels the worker job.
	sub := store.Subscribe(c.Request.Context(), u.ID)
	if sub == nil {
		_ = writeEvent("error", gin.H{"message": "subscribe failed"})
		return
	}
	defer sub.Close()

	items, _, err := h.downloads.List(c.Request.Context(), u.ID, 0, 50)
	if err == nil {
		_ = writeEvent("snapshot", gin.H{"items": items})
	}

	ch := sub.Channel()
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			if _, err := io.WriteString(c.Writer, ": ping\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var ev downloadprog.JobEvent
			if json.Unmarshal([]byte(msg.Payload), &ev) != nil {
				continue
			}
			if err := writeEvent("job", ev); err != nil {
				return
			}
		}
	}
}

func (h *DownloadHandler) GetJob(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	dto, err := h.downloads.Get(c.Request.Context(), u.ID, u.Role, pid)
	if err != nil {
		writeDownloadError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto)
}

func (h *DownloadHandler) CancelJob(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	if err := h.downloads.Cancel(c.Request.Context(), u.ID, u.Role, c.ClientIP(), c.Request.UserAgent(), pid); err != nil {
		writeDownloadError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *DownloadHandler) RetryJob(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	dto, err := h.downloads.Retry(c.Request.Context(), u.ID, u.Role, c.ClientIP(), c.Request.UserAgent(), pid)
	if err != nil {
		writeDownloadError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, dto)
}

func (h *DownloadHandler) DeleteJob(c *gin.Context) {
	u, ok := middleware.GetAuthUser(c)
	if !ok {
		return
	}
	pid, err := uuid.Parse(c.Param("public_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": "invalid public_id"})
		return
	}
	if err := h.downloads.Delete(c.Request.Context(), u.ID, u.Role, c.ClientIP(), c.Request.UserAgent(), pid); err != nil {
		writeDownloadError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func writeDownloadError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrDownloadNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": err.Error()})
	case errors.Is(err, service.ErrDownloadForbidden), errors.Is(err, service.ErrMediaAccessDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "insufficient permissions"})
	case errors.Is(err, service.ErrDownloadBadRequest), errors.Is(err, service.ErrDownloadNoMedia):
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
	case errors.Is(err, service.ErrDownloadQueueFull):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "queue_full", "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": err.Error()})
	}
}
