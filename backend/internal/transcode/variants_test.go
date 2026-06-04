package transcode

import (
	"testing"
)

func TestResolveVariantsExplicit(t *testing.T) {
	src := SourceProfile{Height: 1080, Bitrate: 6_000_000}
	got, err := ResolveVariants([]string{"480p", "720p"}, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "720p" || got[1].Name != "480p" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveVariantsExceedsHeight(t *testing.T) {
	src := SourceProfile{Height: 720, Bitrate: 6_000_000}
	_, err := ResolveVariants([]string{"1080p"}, src)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveVariantsSkipsUpscaleKeepsValid(t *testing.T) {
	src := SourceProfile{Height: 720, Bitrate: 6_000_000}
	got, err := ResolveVariants([]string{"1080p", "720p", "480p"}, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "720p" || got[1].Name != "480p" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveVariantsLowBitrate1080p(t *testing.T) {
	// ~1958 kb/s measured — must not reject 1080p (encode bitrate scales down in ffmpeg).
	src := SourceProfile{Height: 1080, Bitrate: 1_958_000}
	got, err := ResolveVariants([]string{"1080p"}, src)
	if err != nil {
		t.Fatalf("1080p @ ~2Mbps: %v", err)
	}
	if len(got) != 1 || got[0].Name != "1080p" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveVariantsYouTubeLike1080p(t *testing.T) {
	// YouTube re-upload: 1080p nhưng bitrate container thấp — vẫn cho full ladder theo height.
	src := SourceProfile{Height: 1080, Bitrate: 1_823_000}
	got, err := ResolveVariants(nil, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("want 1080p+720p+480p+360p, got %+v", got)
	}
	_, err = ResolveVariants([]string{"1080p", "720p"}, src)
	if err != nil {
		t.Fatalf("explicit 1080p/720p: %v", err)
	}
}

func TestResolveVariantsUnknown(t *testing.T) {
	_, err := ResolveVariants([]string{"4k"}, SourceProfile{Height: 1080})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEncodedWidthAspectRatio(t *testing.T) {
	// 16:9 source: 720p -> 1280 wide.
	if w := encodedWidth(720, SourceProfile{Width: 1920, Height: 1080}); w != 1280 {
		t.Fatalf("16:9 720p width: got %d want 1280", w)
	}
	// Unknown source falls back to 16:9.
	if w := encodedWidth(360, SourceProfile{}); w != 640 {
		t.Fatalf("fallback 360p width: got %d want 640", w)
	}
	// Width is always even.
	if w := encodedWidth(480, SourceProfile{Width: 1080, Height: 1920}); w%2 != 0 {
		t.Fatalf("portrait width must be even, got %d", w)
	}
}

func TestH264CodecsAndLevel(t *testing.T) {
	// 1080p30 needs level 4.0 -> avc1.640028.
	if got := h264AudioCodecs(1920, 1080, 30); got != "avc1.640028,mp4a.40.2" {
		t.Fatalf("1080p30 codecs: got %q", got)
	}
	if got := h264LevelString(1920, 1080, 30); got != "4.0" {
		t.Fatalf("1080p30 level: got %q want 4.0", got)
	}
	// 720p30 fits level 3.1 -> avc1.64001f.
	if got := h264AudioCodecs(1280, 720, 30); got != "avc1.64001f,mp4a.40.2" {
		t.Fatalf("720p30 codecs: got %q", got)
	}
	// 360p30 fits level 3.0 -> avc1.64001e.
	if got := h264AudioCodecs(640, 360, 30); got != "avc1.64001e,mp4a.40.2" {
		t.Fatalf("360p30 codecs: got %q", got)
	}
	// High frame rate at 1080p pushes the level above 4.0.
	if got := h264LevelIDC(1920, 1080, 60); got <= 40 {
		t.Fatalf("1080p60 must exceed level 4.0, got idc %d", got)
	}
}

func TestEncodeBitrateScaledFromSource(t *testing.T) {
	v1080 := variantsByName["1080p"]
	br := EncodeBitrate(v1080, SourceProfile{Height: 1080, Bitrate: 2_000_000})
	if br != "2000k" {
		t.Fatalf("1080p same res: got %q want 2000k", br)
	}
	v720 := variantsByName["720p"]
	br720 := EncodeBitrate(v720, SourceProfile{Height: 1080, Bitrate: 2_000_000})
	if br720 == "2800k" {
		t.Fatalf("720p should scale down from source, got %q", br720)
	}
}
