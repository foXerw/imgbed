package imaging

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
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

func TestProcessInLimitPassthrough(t *testing.T) {
	data := pngBytes(t, 100, 100)
	out, w, h, err := Process(data, "png", 2560, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) {
		t.Fatal("in-limit PNG with convertWebp=false should return original bytes unchanged")
	}
	if w != 100 || h != 100 {
		t.Fatalf("dims = %dx%d, want 100x100", w, h)
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

// pngHeader 构造一个只有签名与 IHDR 的最小 PNG，声明宽高但不含真实像素数据。
func pngHeader(w, h uint32) []byte {
	b := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	var chunk []byte
	chunk = append(chunk, 0, 0, 0, 13) // IHDR 数据长度
	chunk = append(chunk, 'I', 'H', 'D', 'R')
	var fields [13]byte
	binary.BigEndian.PutUint32(fields[0:4], w)
	binary.BigEndian.PutUint32(fields[4:8], h)
	fields[8] = 8 // bit depth
	fields[9] = 6 // color type RGBA
	chunk = append(chunk, fields[:]...)
	var crc [4]byte
	binary.BigEndian.PutUint32(crc[:], crc32.ChecksumIEEE(chunk[4:]))
	chunk = append(chunk, crc[:]...)
	return append(b, chunk...)
}

func TestProcessRejectsHugeImage(t *testing.T) {
	// 头声明 20000×20000（4 亿像素，超过 5000 万上限），应在完整解码前拒绝。
	data := pngHeader(20000, 20000)
	if _, _, _, err := Process(data, "png", 2560, false); err == nil {
		t.Fatal("expected error for oversized image, got nil")
	}
}
