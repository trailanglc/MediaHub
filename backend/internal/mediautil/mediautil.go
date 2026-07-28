package mediautil

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	DefaultChunkSize = 5 * 1024 * 1024 // 5 MiB
	MaxChunkSize     = 32 * 1024 * 1024
	MinChunkSize     = 256 * 1024
	MaxChunks        = 10000
	MaxNameLen       = 255
)

// SanitizeName returns a safe display name from the original filename.
func SanitizeName(original string) string {
	base := filepath.Base(strings.ReplaceAll(original, "\\", "/"))
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == ".." {
		return "untitled"
	}
	var b strings.Builder
	for _, r := range base {
		if r == '/' || r == '\\' || unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "untitled"
	}
	return truncateName(out, MaxNameLen)
}

func truncateName(name string, maxLen int) string {
	if maxLen <= 0 || len(name) <= maxLen {
		return name
	}
	ext, stem := splitNameExt(name)
	room := maxLen - len(ext)
	if room < 1 {
		return name[:maxLen]
	}
	if len(stem) > room {
		stem = stem[:room]
	}
	return stem + ext
}

func splitNameExt(name string) (ext, stem string) {
	ext = path.Ext(name)
	stem = strings.TrimSuffix(name, ext)
	if stem == "" {
		return "", name
	}
	return ext, stem
}

// UniqueSiblingName returns desired, or "stem (1).ext", "stem (2).ext", …
// when desired is already present in taken. Keys in taken are exact names.
func UniqueSiblingName(desired string, taken map[string]struct{}) string {
	desired = strings.TrimSpace(desired)
	if desired == "" {
		desired = "untitled"
	}
	desired = truncateName(desired, MaxNameLen)
	if taken == nil {
		return desired
	}
	if _, exists := taken[desired]; !exists {
		return desired
	}
	ext, stem := splitNameExt(desired)
	for n := 1; ; n++ {
		suffix := fmt.Sprintf(" (%d)", n)
		room := MaxNameLen - len(suffix) - len(ext)
		if room < 1 {
			room = 1
		}
		s := stem
		if len(s) > room {
			s = s[:room]
		}
		candidate := s + suffix + ext
		if _, exists := taken[candidate]; !exists {
			return candidate
		}
	}
}

// DetectObjectType classifies media object type from MIME and extension.
func DetectObjectType(mime, filename string) string {
	m := strings.ToLower(strings.TrimSpace(mime))
	ext := strings.ToLower(filepath.Ext(filename))

	if strings.HasPrefix(m, "video/") || ext == ".mp4" || ext == ".mov" || ext == ".mkv" || ext == ".webm" {
		return "video"
	}
	if strings.HasPrefix(m, "image/") || ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" {
		return "image"
	}
	return "file"
}

// StoragePrefixForType returns the object storage prefix for a media type.
func StoragePrefixForType(objectType string) string {
	switch objectType {
	case "video", "image":
		return "originals/"
	case "file":
		return "documents/"
	default:
		return "documents/"
	}
}

// NormalizeChunkSize clamps chunk size to allowed bounds.
func NormalizeChunkSize(size int) int {
	if size <= 0 {
		return DefaultChunkSize
	}
	if size < MinChunkSize {
		return MinChunkSize
	}
	if size > MaxChunkSize {
		return MaxChunkSize
	}
	return size
}

// ChunkCount returns the number of chunks for a file size.
func ChunkCount(totalSize int64, chunkSize int) int {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	n := int((totalSize + int64(chunkSize) - 1) / int64(chunkSize))
	if n < 1 {
		return 1
	}
	return n
}
