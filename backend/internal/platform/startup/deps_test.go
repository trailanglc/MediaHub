package startup

import (
	"errors"
	"net"
	"strings"
	"testing"
)

func TestWrapPostgres_connectionRefused(t *testing.T) {
	err := WrapPostgres(
		&net.OpError{Op: "dial", Err: errors.New("connect: connection refused")},
		"postgres://mediahub:secret@localhost:5432/mediahub?sslmode=disable",
	)
	var de *DependencyError
	if !errors.As(err, &de) {
		t.Fatalf("expected DependencyError, got %T", err)
	}
	if de.Service != "PostgreSQL" {
		t.Fatalf("unexpected service: %s", de.Service)
	}
	if !strings.Contains(de.Detail, "connection refused") {
		t.Fatalf("detail: %s", de.Detail)
	}
}
