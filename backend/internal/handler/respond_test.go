package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anhtuanlc/mediahub/internal/handler"
	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
)

func TestWriteServicePressure_QueuePaused(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if !handler.WriteServicePressure(c, service.ErrConvertQueuePaused) {
		t.Fatal("expected queue paused mapping")
	}
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "queue_paused" {
		t.Fatalf("error = %v", body["error"])
	}
}

func TestWriteStorageQuotaExceeded(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	handler.WriteStorageQuotaExceeded(c)
	if w.Code != http.StatusInsufficientStorage {
		t.Fatalf("status = %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "storage_quota_exceeded" {
		t.Fatalf("error = %v", body["error"])
	}
}

func TestWriteServicePressure_Unknown(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if handler.WriteServicePressure(c, errors.New("other")) {
		t.Fatal("expected false for unknown error")
	}
}
