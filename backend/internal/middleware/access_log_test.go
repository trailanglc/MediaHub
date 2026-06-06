package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestAccessLogTier(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want accessTier
	}{
		{"/health", accessTierSilent},
		{"/metrics", accessTierSilent},
		{"/stream/vid/master.m3u8", accessTierSilent},
		{"/assets/obj-id/thumb", accessTierSilent},
		{"/embed/vid", accessTierSilent},
		{"/oembed", accessTierSilent},
		{"/api/auth/login", accessTierControl},
		{"/api/videos", accessTierControl},
	}
	for _, tc := range tests {
		if got := accessLogTier(tc.path); got != tc.want {
			t.Errorf("accessLogTier(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestAccessLogLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path   string
		status int
		want   accessLogLevelKind
	}{
		{"/stream/vid/seg.ts", 200, accessLogSkip},
		{"/stream/vid/seg.ts", 404, accessLogWarn},
		{"/stream/vid/seg.ts", 500, accessLogError},
		{"/api/videos", 200, accessLogInfo},
		{"/api/videos", 401, accessLogWarn},
		{"/health", 200, accessLogSkip},
		{"/health", 503, accessLogError},
	}
	for _, tc := range tests {
		if got := accessLogLevel(tc.path, tc.status); got != tc.want {
			t.Errorf("accessLogLevel(%q, %d) = %v, want %v", tc.path, tc.status, got, tc.want)
		}
	}
}

func TestAccessLoggerSkipsSuccessfulStream(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zapcore.InfoLevel)
	log := zap.New(core)

	r := gin.New()
	r.Use(AccessLogger(log))
	r.GET("/stream/:id/*filepath", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/stream/vid/seg.ts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if logs.Len() != 0 {
		t.Fatalf("expected no logs for successful stream request, got %d", logs.Len())
	}
}

func TestAccessLoggerLogsAPIRequest(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zapcore.InfoLevel)
	log := zap.New(core)

	r := gin.New()
	r.Use(AccessLogger(log))
	r.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	if logs.All()[0].Message != "request" {
		t.Fatalf("unexpected message: %s", logs.All()[0].Message)
	}
}
