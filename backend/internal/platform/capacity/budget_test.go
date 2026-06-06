package capacity

import "testing"

func TestConvertSlotsBudget(t *testing.T) {
	tests := []struct {
		cpus, ramGiB, want int
	}{
		{2, 4, 1},
		{4, 8, 2},
		{8, 32, 7},
		{16, 64, 14},
		{32, 128, 28},
	}
	for _, tt := range tests {
		if got := ConvertSlotsBudget(tt.cpus, tt.ramGiB); got != tt.want {
			t.Fatalf("ConvertSlotsBudget(%d,%d)=%d want %d", tt.cpus, tt.ramGiB, got, tt.want)
		}
	}
}

func TestRAMMinIdlePercent(t *testing.T) {
	if got := RAMMinIdlePercent(4 << 30); got != 25 {
		t.Fatalf("4GiB min idle=%d want 25", got)
	}
	if got := RAMMinIdlePercent(64 << 30); got != 10 {
		t.Fatalf("64GiB min idle=%d want 10", got)
	}
}
