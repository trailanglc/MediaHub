package mediautil

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"

	_ "golang.org/x/image/webp"
)

const MaxTransformEdge = 4096

// TransformOptions controls on-the-fly image delivery.
type TransformOptions struct {
	Width  int
	Height int
	Format string // jpeg, png, webp (webp when encoder available)
}

func (o TransformOptions) Active() bool {
	return o.Width > 0 || o.Height > 0 || o.Format != ""
}

func NormalizeTransformFormat(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "jpg", "jpeg":
		return "jpeg"
	case "png":
		return "png"
	case "webp":
		return "webp"
	default:
		return ""
	}
}

func ClampTransformEdge(n int) int {
	if n <= 0 {
		return 0
	}
	if n > MaxTransformEdge {
		return MaxTransformEdge
	}
	return n
}

// TransformImage decodes, optionally resizes, and re-encodes an image.
func TransformImage(src []byte, opts TransformOptions) ([]byte, string, error) {
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	w := ClampTransformEdge(opts.Width)
	h := ClampTransformEdge(opts.Height)
	if w > 0 || h > 0 {
		img = resizeToBox(img, w, h)
	}

	fmtName := NormalizeTransformFormat(opts.Format)
	if fmtName == "" {
		fmtName = "jpeg"
	}

	switch fmtName {
	case "jpeg":
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
			return nil, "", fmt.Errorf("encode jpeg: %w", err)
		}
		return buf.Bytes(), "image/jpeg", nil
	case "png":
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", fmt.Errorf("encode png: %w", err)
		}
		return buf.Bytes(), "image/png", nil
	case "webp":
		out, ct, err := encodeWebP(img)
		if err != nil {
			return nil, "", err
		}
		return out, ct, nil
	default:
		return nil, "", fmt.Errorf("unsupported format")
	}
}

func resizeToBox(src image.Image, maxW, maxH int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return src
	}
	targetW, targetH := w, h
	if maxW > 0 && maxH > 0 {
		scaleW := float64(maxW) / float64(w)
		scaleH := float64(maxH) / float64(h)
		scale := scaleW
		if scaleH < scale {
			scale = scaleH
		}
		if scale >= 1 {
			return src
		}
		targetW = int(float64(w) * scale)
		targetH = int(float64(h) * scale)
	} else if maxW > 0 {
		if w <= maxW {
			return src
		}
		targetW = maxW
		targetH = h * maxW / w
	} else if maxH > 0 {
		if h <= maxH {
			return src
		}
		targetH = maxH
		targetW = w * maxH / h
	}
	if targetW < 1 {
		targetW = 1
	}
	if targetH < 1 {
		targetH = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	for y := 0; y < targetH; y++ {
		sy := b.Min.Y + y*h/targetH
		for x := 0; x < targetW; x++ {
			sx := b.Min.X + x*w/targetW
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}
