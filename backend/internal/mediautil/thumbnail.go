package mediautil

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/gif"
	_ "image/png"
)

const ThumbnailMaxEdge = 320

// GenerateJPEGThumbnail decodes an image and returns a JPEG thumbnail (max edge in pixels).
func GenerateJPEGThumbnail(src []byte, maxEdge int) ([]byte, error) {
	if maxEdge <= 0 {
		maxEdge = ThumbnailMaxEdge
	}
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	thumb := resizeToMaxEdge(img, maxEdge)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 82}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), nil
}

func resizeToMaxEdge(src image.Image, maxEdge int) image.Image {
	b := src.Bounds()
	w := b.Dx()
	h := b.Dy()
	if w <= 0 || h <= 0 {
		return src
	}
	scale := float64(maxEdge) / float64(w)
	if h > w {
		scale = float64(maxEdge) / float64(h)
	}
	if scale >= 1 {
		return src
	}
	nw := int(float64(w) * scale)
	nh := int(float64(h) * scale)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy := b.Min.Y + y*h/nh
		for x := 0; x < nw; x++ {
			sx := b.Min.X + x*w/nw
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}
