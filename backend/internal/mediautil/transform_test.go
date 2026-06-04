package mediautil_test

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"

	"github.com/anhtuanlc/mediahub/internal/mediautil"
)

func TestTransformImageResizeJPEG(t *testing.T) {
	src := makeTestJPEG(t, 200, 100)
	out, ct, err := mediautil.TransformImage(src, mediautil.TransformOptions{
		Width:  80,
		Height: 0,
		Format: "jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ct != "image/jpeg" {
		t.Fatalf("content type: %s", ct)
	}
	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 80 {
		t.Fatalf("width=%d want 80", img.Bounds().Dx())
	}
}

func makeTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, m, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
