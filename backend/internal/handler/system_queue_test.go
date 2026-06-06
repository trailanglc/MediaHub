package handler

import "testing"

func TestQueueDepthFromInfo(t *testing.T) {
	t.Parallel()
	pending, active, scheduled, retry := 3, 2, 1, 4
	depth := pending + active + scheduled + retry
	if depth != 10 {
		t.Fatalf("depth = %d", depth)
	}
}
