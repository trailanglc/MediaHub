package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/platform/clientip"
	"github.com/anhtuanlc/mediahub/internal/platform/logctx"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Request = c.Request.WithContext(logctx.WithRequestID(c.Request.Context(), id))
		c.Next()
	}
}

// Logger logs every HTTP request at info level (legacy; prefer AccessLogger).
func Logger(log *zap.Logger) gin.HandlerFunc {
	return AccessLogger(log)
}

// AccessLogger applies tiered access logging: data-plane paths are silent on success.
func AccessLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		level := accessLogLevel(c.Request.URL.Path, status)
		if level == accessLogSkip {
			return
		}

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("latency", time.Since(start)),
			zap.String("request_id", c.GetString("request_id")),
			zap.String("client_ip", clientip.FromGin(c)),
		}
		if u, ok := GetAuthUser(c); ok {
			fields = append(fields, zap.Int64("user_id", u.ID))
		}

		switch level {
		case accessLogWarn:
			log.Warn("request", fields...)
		case accessLogError:
			log.Error("request", fields...)
		default:
			log.Info("request", fields...)
		}
	}
}

type accessLogLevelKind int

const (
	accessLogSkip accessLogLevelKind = iota
	accessLogInfo
	accessLogWarn
	accessLogError
)

func accessLogLevel(path string, status int) accessLogLevelKind {
	tier := accessLogTier(path)
	switch {
	case status >= http.StatusInternalServerError:
		return accessLogError
	case status >= http.StatusBadRequest:
		return accessLogWarn
	case tier == accessTierSilent:
		return accessLogSkip
	default:
		return accessLogInfo
	}
}

type accessTier int

const (
	accessTierSilent accessTier = iota
	accessTierControl
)

func accessLogTier(path string) accessTier {
	if path == "/health" || path == "/metrics" {
		return accessTierSilent
	}
	if strings.HasPrefix(path, "/stream/") ||
		strings.HasPrefix(path, "/assets/") ||
		strings.HasPrefix(path, "/embed/") ||
		path == "/oembed" {
		return accessTierSilent
	}
	return accessTierControl
}

func CORS(allowedOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", allowedOrigin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
