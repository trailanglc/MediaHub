package service

import "testing"

func exceedsStorageQuota(used, pending, additional, quota int64) bool {
	if quota <= 0 {
		return false
	}
	return used+pending+additional > quota
}

func TestExceedsStorageQuota(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		used, pending, add, quota int64
		want bool
	}{
		{"no quota", 100, 10, 5, 0, false},
		{"under", 100, 10, 5, 200, false},
		{"exact", 100, 50, 50, 200, false},
		{"over init", 100, 50, 51, 200, true},
		{"pending buffers", 190, 5, 6, 200, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := exceedsStorageQuota(tc.used, tc.pending, tc.add, tc.quota); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
