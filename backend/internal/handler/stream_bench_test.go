package handler

import (
	"fmt"
	"strings"
	"testing"
)

func buildMediaPlaylist(segments int) string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:6\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n")
	for i := 0; i < segments; i++ {
		b.WriteString("#EXTINF:6.000000,\n")
		b.WriteString(fmt.Sprintf("segment_%05d.ts\n", i))
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	return b.String()
}

func BenchmarkNormalizePlaylist(b *testing.B) {
	videoID := "550e8400-e29b-41d4-a716-446655440000"
	for _, n := range []int{10, 100, 600} {
		content := buildMediaPlaylist(n)
		b.Run(fmt.Sprintf("segments=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = normalizePlaylistURLs(content, videoID, "720p/index.m3u8")
			}
		})
	}
}

func BenchmarkSanitizeStreamFilePath(b *testing.B) {
	vid := "550e8400-e29b-41d4-a716-446655440000"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = sanitizeStreamFilePath("720p/segment_00042.ts", vid)
	}
}
