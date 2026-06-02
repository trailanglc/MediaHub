package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func StreamForbidden(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{
		"error":   "stream_unavailable",
		"message": "HLS streaming is not enabled until video conversion is complete",
	})
}
