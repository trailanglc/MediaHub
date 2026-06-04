package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anhtuanlc/mediahub/internal/service"
	"github.com/gin-gonic/gin"
)

func TestRequireAPIKey_missingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &service.APIKeyService{}
	r := gin.New()
	r.GET("/test", RequireAPIKey(svc, "media:read"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d body=%s", w.Code, w.Body.String())
	}
}
