package handler

import "testing"

func TestSanitizeStreamFilePath_valid(t *testing.T) {
	vid := "550e8400-e29b-41d4-a716-446655440000"
	got, ok := sanitizeStreamFilePath("master.m3u8", vid)
	if !ok || got != "master.m3u8" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
	got, ok = sanitizeStreamFilePath("720p/segment_00001.ts", vid)
	if !ok || got != "720p/segment_00001.ts" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestSanitizeStreamFilePath_rejectsTraversal(t *testing.T) {
	vid := "550e8400-e29b-41d4-a716-446655440000"
	cases := []string{
		"../secret.m3u8",
		"720p/../../other/master.m3u8",
		"..",
		"",
	}
	for _, p := range cases {
		if _, ok := sanitizeStreamFilePath(p, vid); ok {
			t.Fatalf("expected reject for %q", p)
		}
	}
}

func TestValidateStreamRelativePath(t *testing.T) {
	if _, ok := validateStreamRelativePath(".."); ok {
		t.Fatal("expected reject")
	}
	if p, ok := validateStreamRelativePath("variants/720p/index.m3u8"); !ok || p != "variants/720p/index.m3u8" {
		t.Fatalf("got %q ok=%v", p, ok)
	}
}
