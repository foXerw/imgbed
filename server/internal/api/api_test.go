package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
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

func TestListAndDelete(t *testing.T) {
	ts, cfg, _ := newTestServer(t)
	defer ts.Close()

	// 上传一张真实 PNG
	png := validPNG(t)
	resp := multipartUpload(t, ts.URL+"/api/upload", cfg.Token, "a.png", png)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 列表
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/images", nil)
	req.Header.Set("X-Auth-Token", cfg.Token)
	listResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d", listResp.StatusCode)
	}
	defer listResp.Body.Close()
	var list struct {
		Images []struct {
			ID  string `json:"id"`
			URL string `json:"url"`
		} `json:"images"`
		Total int `json:"total"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || len(list.Images) != 1 {
		t.Fatalf("total=%d len=%d, want 1/1", list.Total, len(list.Images))
	}
	id := list.Images[0].ID

	// 删除
	delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/images/"+id, nil)
	delReq.Header.Set("X-Auth-Token", cfg.Token)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("delete status = %d", delResp.StatusCode)
	}
}

func TestListRequiresToken(t *testing.T) {
	ts, _, _ := newTestServer(t)
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/api/images")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestDeleteRejectsTraversal(t *testing.T) {
	ts, cfg, _ := newTestServer(t)
	defer ts.Close()
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/images/../../etc/passwd", nil)
	req.Header.Set("X-Auth-Token", cfg.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestStaticServing(t *testing.T) {
	ts, cfg, _ := newTestServer(t)
	defer ts.Close()

	png := validPNG(t)
	resp := multipartUpload(t, ts.URL+"/api/upload", cfg.Token, "a.png", png)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status = %d", resp.StatusCode)
	}
	var up struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&up); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// 通过相对路径直接 GET 图片（公开、无 token）
	getResp, err := http.Get(ts.URL + "/" + relFromURL(up.URL))
	if err != nil {
		t.Fatal(err)
	}
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("static status = %d, want 200", getResp.StatusCode)
	}
}

func TestAdminRequiresToken(t *testing.T) {
	ts, _, _ := newTestServer(t)
	defer ts.Close()
	if resp, err := http.Get(ts.URL + "/admin"); err == nil {
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
	} else {
		t.Fatal(err)
	}
}

func TestAdminServesEmbeddedPage(t *testing.T) {
	ts, cfg, _ := newTestServer(t)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/admin", nil)
	req.Header.Set("X-Auth-Token", cfg.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "图床管理") {
		t.Fatalf("body missing admin title")
	}
}

func relFromURL(u string) string {
	for i := 0; i < 3; i++ {
		if idx := indexByte(u, '/'); idx >= 0 {
			u = u[idx+1:]
		}
	}
	return u
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
