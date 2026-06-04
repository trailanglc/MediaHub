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
	if len(got) != 3 {
		t.Fatalf("want 1080p+720p+480p, got %+v", got)
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
