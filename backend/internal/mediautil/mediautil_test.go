package mediautil_test

import (
	"testing"

	"github.com/anhtuanlc/mediahub/internal/mediautil"
)

func TestSanitizeName(t *testing.T) {
	if got := mediautil.SanitizeName("../../../etc/passwd"); got != "passwd" {
		t.Fatalf("sanitize path: got %q", got)
	}
	if got := mediautil.SanitizeName(""); got != "untitled" {
		t.Fatalf("empty: got %q", got)
	}
}

func TestUniqueSiblingName(t *testing.T) {
	taken := map[string]struct{}{
		"video.mp4": {},
		"notes":     {},
	}
	if got := mediautil.UniqueSiblingName("photo.png", taken); got != "photo.png" {
		t.Fatalf("free name: got %q", got)
	}
	if got := mediautil.UniqueSiblingName("video.mp4", taken); got != "video (1).mp4" {
		t.Fatalf("file conflict: got %q", got)
	}
	taken["video (1).mp4"] = struct{}{}
	if got := mediautil.UniqueSiblingName("video.mp4", taken); got != "video (2).mp4" {
		t.Fatalf("second conflict: got %q", got)
	}
	if got := mediautil.UniqueSiblingName("notes", taken); got != "notes (1)" {
		t.Fatalf("folder conflict: got %q", got)
	}
	if got := mediautil.UniqueSiblingName(".gitignore", map[string]struct{}{".gitignore": {}}); got != ".gitignore (1)" {
		t.Fatalf("dotfile conflict: got %q", got)
	}
}

func TestDetectObjectType(t *testing.T) {
	if mediautil.DetectObjectType("video/mp4", "clip.mp4") != "video" {
		t.Fatal("expected video")
	}
	if mediautil.DetectObjectType("image/png", "a.png") != "image" {
		t.Fatal("expected image")
	}
	if mediautil.DetectObjectType("application/pdf", "doc.pdf") != "file" {
		t.Fatal("expected file")
	}
}

func TestChunkCount(t *testing.T) {
	if n := mediautil.ChunkCount(10, 5); n != 2 {
		t.Fatalf("chunks: got %d", n)
	}
}
