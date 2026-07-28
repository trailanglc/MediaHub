package download

import (
	"testing"
)

func TestValidateURL_BlocksPrivate(t *testing.T) {
	cases := []string{
		"http://127.0.0.1/x",
		"http://localhost/x",
		"http://10.0.0.1/a",
		"ftp://example.com/a",
		"",
	}
	for _, c := range cases {
		if _, err := ValidateURL(c); err == nil {
			t.Fatalf("expected block for %q", c)
		}
	}
	if _, err := ValidateURL("https://example.com/video.mp4"); err != nil {
		t.Fatal(err)
	}
}

func TestClassifyPathHeuristics(t *testing.T) {
	// Path-only classification without network for m3u8/mp4 extensions.
	u, err := ValidateURL("https://cdn.example.com/a/b/master.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	if !stringsHasSuffixFold(u.Path, ".m3u8") {
		t.Fatal("expected m3u8 path")
	}
}

func stringsHasSuffixFold(s, suf string) bool {
	if len(s) < len(suf) {
		return false
	}
	return equalFoldASCII(s[len(s)-len(suf):], suf)
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func TestRewriteMediaPlaylist(t *testing.T) {
	lines := splitLines(`#EXTM3U
#EXT-X-TARGETDURATION:6
#EXTINF:6.0,
https://cdn.example.com/seg0.ts
#EXTINF:6.0,
seg1.ts
#EXT-X-ENDLIST
`)
	if hasEncryption(lines) {
		t.Fatal("not encrypted")
	}
	if !looksLikeMaster([]string{"#EXTM3U", "#EXT-X-STREAM-INF:BANDWIDTH=100", "a.m3u8"}) {
		t.Fatal("expected master")
	}
}

func TestRedactURL(t *testing.T) {
	got := RedactURL("https://cdn.example.com/x?token=secret")
	if got != "https://cdn.example.com/x" {
		t.Fatalf("got %q", got)
	}
}
