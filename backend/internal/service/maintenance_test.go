package service

import (
	"testing"

	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/google/uuid"
)

func TestPublicIDFromStorageKey(t *testing.T) {
	t.Parallel()
	vid := "11111111-1111-4111-8111-111111111111"
	cases := []struct {
		key  string
		want string
	}{
		{storage.PrefixThumbnails + vid + ".jpg", vid},
		{storage.PrefixHLS + vid + "/720p/seg001.ts", vid},
		{storage.PrefixOriginals + "abc", ""},
	}
	for _, tc := range cases {
		if got := publicIDFromStorageKey(tc.key); got != tc.want {
			t.Errorf("publicIDFromStorageKey(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestIsProtectedStorageKey(t *testing.T) {
	t.Parallel()
	vid := "22222222-2222-4222-8222-222222222222"
	pid, err := uuid.Parse(vid)
	if err != nil {
		t.Fatal(err)
	}
	thumb := storage.ThumbnailObjectKey(pid)
	refs := map[string]struct{}{"originals/kept": {}}
	publicIDs := map[string]struct{}{vid: {}}

	if !isProtectedStorageKey("originals/kept", refs, nil, publicIDs) {
		t.Fatal("expected referenced key protected")
	}
	if !isProtectedStorageKey(thumb, refs, nil, publicIDs) {
		t.Fatal("expected thumbnail by public id protected")
	}
	if isProtectedStorageKey("originals/orphan", refs, nil, publicIDs) {
		t.Fatal("unexpected orphan protected")
	}
	if !isProtectedStorageKey("temp/uploads/x/chunk_00001", refs, []string{"temp/uploads/x/"}, publicIDs) {
		t.Fatal("expected upload prefix guard")
	}
}
