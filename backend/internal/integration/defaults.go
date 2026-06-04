package integration

import "github.com/google/uuid"

// DefaultRootFolderPublicID is the seeded workspace root (see migration 000004).
var DefaultRootFolderPublicID = uuid.MustParse("00000000-0000-4000-8000-000000000001")

const (
	AssetVariantThumbnail = "thumbnail"
	AssetVariantImage     = "image"
	AssetVariantFile      = "file"
	AssetVariantEmbed     = "embed"
)
