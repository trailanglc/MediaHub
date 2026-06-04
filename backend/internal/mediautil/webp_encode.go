//go:build cgo

package mediautil

import (
	"bytes"
	"fmt"
	"image"

	"github.com/chai2010/webp"
)

func encodeWebP(img image.Image) ([]byte, string, error) {
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, &webp.Options{Quality: 85}); err != nil {
		return nil, "", fmt.Errorf("encode webp: %w", err)
	}
	return buf.Bytes(), "image/webp", nil
}
