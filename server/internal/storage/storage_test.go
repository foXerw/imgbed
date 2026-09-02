package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndURL(t *testing.T) {
	dir := t.TempDir()
	s := New(dir, "https://img.example.com")
	rel, err := s.Save([]byte("hello"), "png")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rel, "20") || !strings.HasSuffix(rel, ".png") {
		t.Fatalf("rel = %q, want date-based .png path", rel)
	}
	// 文件确实落盘
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
		t.Fatalf("file not written: %v", err)
	}
	if got := s.URL(rel); got != "https://img.example.com/"+rel {
		t.Fatalf("URL = %q, want https://img.example.com/%s", got, rel)
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	s := New(dir, "http://x")
	rel, err := s.Save([]byte("x"), "png")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(rel); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, got err=%v", err)
	}
}

func TestListSorted(t *testing.T) {
	dir := t.TempDir()
	s := New(dir, "http://x")
	if _, err := s.Save([]byte("a"), "png"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save([]byte("bb"), "jpg"); err != nil {
		t.Fatal(err)
	}
	imgs, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 2 {
		t.Fatalf("len = %d, want 2", len(imgs))
	}
	if !imgs[0].UploadedAt.After(imgs[1].UploadedAt) && !imgs[0].UploadedAt.Equal(imgs[1].UploadedAt) {
		t.Fatal("list not sorted by uploadedAt desc")
	}
}

func TestListEmptyDir(t *testing.T) {
	s := New(t.TempDir(), "http://x")
	imgs, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 0 {
		t.Fatalf("len = %d, want 0", len(imgs))
	}
}
