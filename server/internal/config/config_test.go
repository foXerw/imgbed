package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	c := Default()
	if c.Listen != ":8080" {
		t.Fatalf("Listen = %q, want :8080", c.Listen)
	}
	if c.MaxDimension != 2560 {
		t.Fatalf("MaxDimension = %d, want 2560", c.MaxDimension)
	}
	if c.MaxSizeMB != 5 {
		t.Fatalf("MaxSizeMB = %d, want 5", c.MaxSizeMB)
	}
	if len(c.AllowedTypes) != 5 {
		t.Fatalf("AllowedTypes len = %d, want 5", len(c.AllowedTypes))
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "listen: \":9090\"\nstorageDir: \"/data/img\"\ntoken: \"secret\"\nconvertWebp: true\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Listen != ":9090" {
		t.Fatalf("Listen = %q, want :9090", c.Listen)
	}
	if c.StorageDir != "/data/img" {
		t.Fatalf("StorageDir = %q, want /data/img", c.StorageDir)
	}
	if c.Token != "secret" {
		t.Fatalf("Token = %q, want secret", c.Token)
	}
	if !c.ConvertWebp {
		t.Fatal("ConvertWebp = false, want true")
	}
	// 未设置的字段应回落到默认值
	if c.MaxDimension != 2560 {
		t.Fatalf("MaxDimension = %d, want default 2560", c.MaxDimension)
	}
}
