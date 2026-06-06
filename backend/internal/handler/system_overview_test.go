package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSumVideosActive(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		hls  map[string]int64
		want int64
	}{
		{"nil", nil, 0},
		{"empty", map[string]int64{}, 0},
		{
			"excludes deleted",
			map[string]int64{"ready": 10, "failed": 2, "deleted": 5},
			12,
		},
		{
			"all statuses",
			map[string]int64{"none": 1, "pending": 2, "converting": 1, "ready": 40, "failed": 1, "deleted": 3},
			45,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sumVideosActive(tt.hls); got != tt.want {
				t.Fatalf("sumVideosActive() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCompactComponents(t *testing.T) {
	t.Parallel()
	components := map[string]componentHealth{
		"database": {Status: "healthy"},
		"worker":   {Status: "unknown", Error: "no heartbeat"},
	}
	out := compactComponents(components)
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	worker, ok := out["worker"].(gin.H)
	if !ok {
		t.Fatalf("worker entry type = %T", out["worker"])
	}
	if worker["status"] != "unknown" || worker["error"] != "no heartbeat" {
		t.Fatalf("worker = %#v", worker)
	}
}

func TestOverviewJobLimitConstant(t *testing.T) {
	t.Parallel()
	if overviewJobLimit != 5 {
		t.Fatalf("overviewJobLimit = %d, want 5", overviewJobLimit)
	}
}
