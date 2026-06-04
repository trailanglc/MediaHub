//go:build !cgo

package mediautil

import (
	"fmt"
	"image"
)

func encodeWebP(_ image.Image) ([]byte, string, error) {
	return nil, "", fmt.Errorf("webp encoding requires CGO (build with CGO_ENABLED=1 and libwebp)")
}
