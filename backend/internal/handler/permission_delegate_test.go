package handler

import "testing"

func TestDelegateMayGrant(t *testing.T) {
	t.Parallel()
	if delegateMayGrant("manage") || delegateMayGrant("share") {
		t.Fatal("delegates must not grant manage/share")
	}
	for _, p := range []string{"read", "upload", "update", "delete", "convert", "stream", "download"} {
		if !delegateMayGrant(p) {
			t.Fatalf("delegate should grant %q", p)
		}
	}
}
