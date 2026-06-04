package handler

import "testing"

func TestIsHLSPlaylistPath(t *testing.T) {
	cases := map[string]bool{
		"master.m3u8":           true,
		"720p/index.m3u8":       true,
		"720p/segment_00001.ts": false,
		"seg.M3U8":              true,
	}
	for path, want := range cases {
		if got := isHLSPlaylistPath(path); got != want {
			t.Fatalf("%q: want %v got %v", path, want, got)
		}
	}
}

func TestNormalizePlaylistURLs_variantSegmentStaysRelative(t *testing.T) {
	vid := "550e8400-e29b-41d4-a716-446655440000"
	in := "#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:6.0,\nsegment_00001.ts\n#EXT-X-ENDLIST\n"
	out := normalizePlaylistURLs(in, vid, "720p/index.m3u8")
	if !containsStr(out, "segment_00001.ts") {
		t.Fatalf("expected playlist-relative segment, got %q", out)
	}
	if containsStr(out, "720p/segment_00001.ts") {
		t.Fatalf("must not prefix variant dir twice, got %q", out)
	}
}

func TestNormalizePlaylistURLs_masterKeepsVariantPaths(t *testing.T) {
	vid := "550e8400-e29b-41d4-a716-446655440000"
	in := "#EXTM3U\n720p/index.m3u8\n1080p/index.m3u8\n"
	out := normalizePlaylistURLs(in, vid, "master.m3u8")
	if !containsStr(out, "720p/index.m3u8") || !containsStr(out, "1080p/index.m3u8") {
		t.Fatalf("expected variant playlist paths from master, got %q", out)
	}
}

func TestNormalizePlaylistURLs_stripsOldSignedURL(t *testing.T) {
	vid := "550e8400-e29b-41d4-a716-446655440000"
	line := "http://api.local/stream/" + vid + "/720p/segment_00001.ts?token=abc&exp=1"
	in := "#EXTM3U\n" + line + "\n"
	out := normalizePlaylistURLs(in, vid, "720p/index.m3u8")
	if containsStr(out, "token=") || containsStr(out, "http://") {
		t.Fatalf("expected relative path only, got %q", out)
	}
	if !containsStr(out, "segment_00001.ts") {
		t.Fatalf("missing segment path in %q", out)
	}
	if containsStr(out, "720p/segment_00001.ts") {
		t.Fatalf("variant playlist must not double-prefix, got %q", out)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (sub == "" || findSub(s, sub))
}

func findSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestPlaylistCache(t *testing.T) {
	c := newPlaylistBodyCache()
	c.Set("v1", "master.m3u8", []byte("data"))
	if _, hit := c.Get("v1", "master.m3u8"); !hit {
		t.Fatal("expected cache hit")
	}
	c.InvalidateVideo("v1")
	if _, hit := c.Get("v1", "master.m3u8"); hit {
		t.Fatal("expected miss after invalidate")
	}
}
