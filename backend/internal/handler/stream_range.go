package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// parseByteRange parses a single Range header value "bytes=start-end" against total size.
func parseByteRange(rangeHeader string, size int64) (st, en int64, ok bool) {
	if size <= 0 || rangeHeader == "" {
		return 0, 0, false
	}
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, 0, false
	}
	spec := strings.TrimSpace(strings.TrimPrefix(rangeHeader, "bytes="))
	if spec == "" {
		return 0, 0, false
	}
	if i := strings.Index(spec, ","); i >= 0 {
		spec = strings.TrimSpace(spec[:i])
	}
	if strings.HasPrefix(spec, "-") {
		suffix, err := strconv.ParseInt(strings.TrimPrefix(spec, "-"), 10, 64)
		if err != nil || suffix <= 0 {
			return 0, 0, false
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, size - 1, true
	}
	parts := strings.Split(spec, "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	st, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || st < 0 {
		return 0, 0, false
	}
	if st >= size {
		return 0, 0, false
	}
	if parts[1] == "" {
		return st, size - 1, true
	}
	en, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil || en < st {
		return 0, 0, false
	}
	if en >= size {
		en = size - 1
	}
	return st, en, true
}

func streamContentType(filePath string) string {
	lower := strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(lower, ".m3u8"):
		return "application/vnd.apple.mpegurl"
	case strings.HasSuffix(lower, ".ts"):
		return "video/mp2t"
	case strings.HasSuffix(lower, ".m4s"):
		return "video/iso.segment"
	default:
		return "application/octet-stream"
	}
}

func setImmutableCacheHeaders(h http.Header, etag string) {
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	h.Set("Accept-Ranges", "bytes")
	if etag != "" {
		h.Set("ETag", fmt.Sprintf(`"%s"`, etag))
	}
}
