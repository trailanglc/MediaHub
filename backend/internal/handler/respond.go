package handler

import (
	"errors"
	"net/http"

	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
)

const systemBusyRetryAfterSec = 30

// WriteSystemBusy responds with HTTP 503 when the server is throttling under resource pressure.
func WriteSystemBusy(c *gin.Context) {
	c.Header("Retry-After", "30")
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":               "system_busy",
		"message":             "Hệ thống đang quá tải tài nguyên. Vui lòng thử lại sau.",
		"retry_after_seconds": systemBusyRetryAfterSec,
	})
}

// WriteQueueFull responds with HTTP 503 when the convert queue has reached its cap.
func WriteQueueFull(c *gin.Context) {
	c.Header("Retry-After", "60")
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error":               "queue_full",
		"message":             "Hàng đợi chuyển mã đầy. Vui lòng thử lại sau.",
		"retry_after_seconds": 60,
	})
}

// WriteServicePressure maps resource-related service errors to stable HTTP responses.
func WriteServicePressure(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, service.ErrSystemBusy):
		WriteSystemBusy(c)
		return true
	case errors.Is(err, service.ErrConvertQueueFull):
		WriteQueueFull(c)
		return true
	default:
		return false
	}
}
