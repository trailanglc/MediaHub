package handler

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestParseAuditListFilters(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/?actions=auth.login,video.convert&actor_public_id=550e8400-e29b-41d4-a716-446655440000&from=2026-01-01&to=2026-01-31", nil)

	filters, err := parseAuditListFilters(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(filters.Actions) != 2 {
		t.Fatalf("actions = %v", filters.Actions)
	}
	if filters.ActorPublicID == nil {
		t.Fatal("expected actor public id")
	}
	if filters.From == nil || filters.To == nil {
		t.Fatal("expected from/to")
	}
	if filters.To.Hour() != 23 {
		t.Fatalf("to end-of-day expected, got %v", filters.To)
	}
}

func TestParseAuditTimeRFC3339(t *testing.T) {
	t.Parallel()
	raw := "2026-06-06T10:30:00Z"
	got, err := parseAuditTime(raw, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.UTC().Format(time.RFC3339) != raw {
		t.Fatalf("got %v", got)
	}
}
