package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"imgserver/internal/config"
	"imgserver/internal/storage"
)

func newTestServer(t *testing.T) (*httptest.Server, *config.Config, *storage.Store) {
	t.Helper()
	cfg := config.Default()
	cfg.Token = "test-token"
	cfg.StorageDir = t.TempDir()
	st := storage.New(cfg.StorageDir, cfg.BaseURL)
	h := New(cfg, st)
	return httptest.NewServer(h), cfg, st
}

func multipartUpload(t *testing.T, url, token, filename string, data []byte) *http.Response {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	w.Close()
	req, _ := http.NewRequest(http.MethodPost, url, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if token != "" {
		req.Header.Set("X-Auth-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestUploadRequiresToken(t *testing.T) {
	ts, _, _ := newTestServer(t)
	defer ts.Close()
	resp := multipartUpload(t, ts.URL+"/api/upload", "", "a.png", []byte{0x89, 'P', 'N', 'G'})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestUploadRejectsBadToken(t *testing.T) {
	ts, _, _ := newTestServer(t)
	defer ts.Close()
	resp := multipartUpload(t, ts.URL+"/api/upload", "wrong", "a.png", []byte{0x89, 'P', 'N', 'G'})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func validPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUploadSuccess(t *testing.T) {
	ts, _, _ := newTestServer(t)
	defer ts.Close()
	resp := multipartUpload(t, ts.URL+"/api/upload", "test-token", "test.png", validPNG(t))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got uploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.ID == "" {
		t.Fatal("missing id")
	}
	if got.URL == "" {
		t.Fatal("missing url")
	}
	if got.Filename != "test.png" {
		t.Fatalf("filename = %q, want test.png", got.Filename)
	}
	if got.Size <= 0 {
		t.Fatalf("size = %d, want > 0", got.Size)
	}
	if got.Width != 10 || got.Height != 10 {
		t.Fatalf("dims = %dx%d, want 10x10", got.Width, got.Height)
	}
	if got.Ext != "png" {
		t.Fatalf("ext = %q, want png", got.Ext)
	}
	if !strings.HasSuffix(got.ID, ".png") {
		t.Fatalf("id = %q, want .png suffix", got.ID)
	}
}

func TestUploadTooLarge(t *testing.T) {
	ts, cfg, _ := newTestServer(t)
	defer ts.Close()
	cfg.MaxSizeMB = 1
	resp := multipartUpload(t, ts.URL+"/api/upload", "test-token", "big.bin", bytes.Repeat([]byte("a"), (1<<20)+1))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.StatusCode)
	}
}

func TestUploadUnsupportedType(t *testing.T) {
	ts, _, _ := newTestServer(t)
	defer ts.Close()
	resp := multipartUpload(t, ts.URL+"/api/upload", "test-token", "bad.txt", []byte("this is not an image"))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", resp.StatusCode)
	}
}
