package mediautil

import (
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
	if len(out) > 255 {
		ext := path.Ext(out)
		name := strings.TrimSuffix(out, ext)
		if len(name) > 250 {
			name = name[:250]
		}
		out = name + ext
	}
	return out
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
