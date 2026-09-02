package imaging

import (
	"image"
	"image/color"
	"image/png"
	"bytes"
	"testing"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestFormat(t *testing.T) {
	if got := Format(pngBytes(t, 10, 10)); got != "png" {
		t.Fatalf("Format = %q, want png", got)
	}
	if got := Format([]byte("<svg xmlns=\"x\"></svg>")); got != "svg" {
		t.Fatalf("Format = %q, want svg", got)
	}
	if got := Format([]byte("garbage")); got != "" {
		t.Fatalf("Format = %q, want empty", got)
	}
}

func TestProcessDownscales(t *testing.T) {
	data := pngBytes(t, 4000, 2000)
	out, w, h, err := Process(data, "png", 2560, false)
	if err != nil {
		t.Fatal(err)
	}
	if w != 2560 || h != 1280 {
		t.Fatalf("dims = %dx%d, want 2560x1280", w, h)
	}
	if len(out) == 0 {
		t.Fatal("empty output")
	}
}

func TestProcessNoUpscale(t *testing.T) {
	data := pngBytes(t, 100, 100)
	_, w, h, err := Process(data, "png", 2560, false)
	if err != nil {
		t.Fatal(err)
	}
	if w != 100 || h != 100 {
		t.Fatalf("dims = %dx%d, want 100x100 (no upscale)", w, h)
	}
}

func TestProcessToWebp(t *testing.T) {
	data := pngBytes(t, 200, 200)
	out, _, _, err := Process(data, "png", 2560, true)
	if err != nil {
		t.Fatal(err)
	}
	if Format(out) != "webp" {
		t.Fatalf("Format = %q, want webp", Format(out))
	}
}
