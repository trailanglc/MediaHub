package platform

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestNormalizeLogLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw, fallback, want string
	}{
		{"", "info", "info"},
		{"DEBUG", "info", "debug"},
		{"warn", "info", "warn"},
		{"invalid", "info", "info"},
	}
	for _, tc := range tests {
		if got := NormalizeLogLevel(tc.raw, tc.fallback); got != tc.want {
			t.Errorf("NormalizeLogLevel(%q, %q) = %q, want %q", tc.raw, tc.fallback, got, tc.want)
		}
	}
}

func TestDefaultLogLevel(t *testing.T) {
	t.Parallel()
	if got := DefaultLogLevel("development"); got != "debug" {
		t.Errorf("development default = %q, want debug", got)
	}
	if got := DefaultLogLevel("production"); got != "info" {
		t.Errorf("production default = %q, want info", got)
	}
}

func TestNewLoggerRespectsLevel(t *testing.T) {
	t.Parallel()
	log, err := NewLogger("production", "error")
	if err != nil {
		t.Fatal(err)
	}
	if !log.Core().Enabled(zapcore.ErrorLevel) {
		t.Error("expected error level enabled")
	}
	if log.Core().Enabled(zapcore.InfoLevel) {
		t.Error("expected info level disabled when LOG_LEVEL=error")
	}
}
