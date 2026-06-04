package storage

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	PrefixOriginals  = "originals/"
	PrefixHLS        = "hls/"
	PrefixThumbnails = "thumbnails/"
	PrefixTemp       = "temp/"
	PrefixDocuments  = "documents/"
)

// NewObjectKey returns a provider-neutral object key under the given prefix.
func NewObjectKey(prefix string) string {
	id := strings.ReplaceAll(uuid.New().String(), "-", "")
	return fmt.Sprintf("%s%s", prefix, id)
}

// UploadSessionPrefix returns temp storage prefix for chunked uploads.
func UploadSessionPrefix(sessionPublicID uuid.UUID) string {
	return fmt.Sprintf("%suploads/%s/", PrefixTemp, sessionPublicID.String())
}

// UploadChunkKey returns the storage key for a chunk index.
func UploadChunkKey(prefix string, index int) string {
	return fmt.Sprintf("%schunk_%05d", prefix, index)
}

// ThumbnailObjectKey returns the storage key for a generated image thumbnail.
func ThumbnailObjectKey(publicID uuid.UUID) string {
	return fmt.Sprintf("%s%s.jpg", PrefixThumbnails, publicID.String())
}

// HLSPrefix returns the storage prefix for all HLS output of a video.
func HLSPrefix(videoPublicID uuid.UUID) string {
	return fmt.Sprintf("%s%s/", PrefixHLS, videoPublicID.String())
}

// HLSMasterKey returns the master playlist object key.
func HLSMasterKey(videoPublicID uuid.UUID) string {
	return fmt.Sprintf("%smaster.m3u8", HLSPrefix(videoPublicID))
}

// HLSVariantPlaylistKey returns a variant playlist key (e.g. 720p/index.m3u8).
func HLSVariantPlaylistKey(videoPublicID uuid.UUID, quality string) string {
	return fmt.Sprintf("%s%s/index.m3u8", HLSPrefix(videoPublicID), quality)
}
