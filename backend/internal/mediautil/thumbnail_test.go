package mediautil

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestGenerateJPEGThumbnail(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 8), B: 100, A: 255})
		}
	}
	var src bytes.Buffer
	if err := png.Encode(&src, img); err != nil {
		t.Fatal(err)
	}
	out, err := GenerateJPEGThumbnail(src.Bytes(), 16)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 100 {
		t.Fatalf("thumbnail too small: %d bytes", len(out))
	}
}
