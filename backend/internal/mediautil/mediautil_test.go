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
