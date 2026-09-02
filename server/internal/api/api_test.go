package api

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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
