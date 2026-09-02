# 图床服务端（Go）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现一个 Go 图床服务端，提供图片上传、静态托管、图片处理、Token 鉴权与 Web 管理界面。

**Architecture:** 单二进制 HTTP 服务。`net/http` + `chi` 路由；`internal/config` 负责配置，`internal/storage` 负责本地磁盘存储，`internal/imaging` 负责图片处理，`internal/api` 负责 HTTP handler 与鉴权中间件，`web/` 内嵌管理界面。

**Tech Stack:** Go 1.22、`go-chi/chi/v5`、`disintegration/imaging`、`chai2010/webp`、`gopkg.in/yaml.v3`。

**Spec:** `docs/superpowers/specs/2026-09-03-image-hosting-design.md`

## Global Constraints

- Go 版本 ≥ 1.22；module 名 `imgserver`。
- 图片存储目录 `./storage`（可配）；URL 路径 `YYYY/MM/DD/<8位hex>.<ext>`。
- 鉴权：写操作（上传/列表/删除/管理界面）要求 `X-Auth-Token` 头 == 配置 `token`；静态读公开。
- 默认处理参数：`maxDimension: 2560`、`maxSizeMB: 5`、`convertWebp: false`。
- 允许类型：`jpeg`、`png`、`gif`、`webp`、`svg`（jpeg/png/webp 参与缩放压缩；gif/svg 原样存储）。
- 扩展名映射：`jpeg→jpg`、`png→png`、`gif→gif`、`webp→webp`、`svg→svg`；转 WebP 后为 `webp`。
- 错误体统一 `{"error":"<message>"}`，状态码语义化（400/401/404/413/415/500）。
- 上传响应字段：`{id,url,filename,size,width,height,ext}`；列表响应 `{images,total,page,pageSize}`，每条 `{id,url,size,uploadedAt}`；`id` 即相对路径（含日期）。

---

### Task 1: 脚手架 + 配置模块

**Files:**
- Create: `server/go.mod`
- Create: `server/internal/config/config.go`
- Test: `server/internal/config/config_test.go`

**Interfaces:**
- Produces: `config.Default() *Config`；`config.Load(path string) (*Config, error)`；`Config` 结构体字段 `Listen, StorageDir, BaseURL, Token, MaxDimension, MaxSizeMB, ConvertWebp, AllowedTypes`。

- [ ] **Step 1: 初始化 git 与 go module**

```bash
cd /d/code/image
git init -b main
cd server
go mod init imgserver
```

- [ ] **Step 2: 写失败测试**

创建 `server/internal/config/config_test.go`：

```go
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
```

- [ ] **Step 3: 运行测试确认失败**

```bash
cd server && go test ./internal/config/ -run TestDefault -v
```

预期：编译失败（`config.go` 不存在 / `Default` 未定义）。

- [ ] **Step 4: 实现配置模块**

创建 `server/internal/config/config.go`：

```go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen       string   `yaml:"listen"`
	StorageDir   string   `yaml:"storageDir"`
	BaseURL      string   `yaml:"baseURL"`
	Token        string   `yaml:"token"`
	MaxDimension int      `yaml:"maxDimension"`
	MaxSizeMB    int64    `yaml:"maxSizeMB"`
	ConvertWebp  bool     `yaml:"convertWebp"`
	AllowedTypes []string `yaml:"allowedTypes"`
}

func Default() *Config {
	return &Config{
		Listen:       ":8080",
		StorageDir:   "./storage",
		BaseURL:      "http://localhost:8080",
		Token:        "change-me",
		MaxDimension: 2560,
		MaxSizeMB:    5,
		ConvertWebp:  false,
		AllowedTypes: []string{"jpeg", "png", "gif", "webp", "svg"},
	}
}

// Load 读取 YAML 配置并叠加在默认值之上。
func Load(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
cd server && go test ./internal/config/ -v
```

预期：PASS。

- [ ] **Step 6: 提交**

```bash
cd /d/code/image && git add server/go.mod server/go.sum server/internal/config && git commit -m "feat(server): config module"
```

---

### Task 2: 磁盘存储模块

**Files:**
- Create: `server/internal/storage/storage.go`
- Test: `server/internal/storage/storage_test.go`

**Interfaces:**
- Produces: `storage.New(root, baseURL string) *Store`；`(*Store).Save(data []byte, ext string) (string, error)`（返回相对路径如 `2026/09/03/abc123.png`）；`(*Store).URL(rel string) string`；`(*Store).Delete(rel string) error`；`(*Store).List() ([]Image, error)`；`Image{ID, URL string; Size int64; UploadedAt time.Time}`。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/storage/storage_test.go`：

```go
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
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/storage/ -run TestSaveAndURL -v
```

预期：编译失败（`storage.go` 不存在）。

- [ ] **Step 3: 实现存储模块**

创建 `server/internal/storage/storage.go`：

```go
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Image struct {
	ID         string    `json:"id"`
	URL        string    `json:"url"`
	Size       int64     `json:"size"`
	UploadedAt time.Time `json:"uploadedAt"`
}

type Store struct {
	root    string
	baseURL string
}

func New(root, baseURL string) *Store {
	return &Store{root: root, baseURL: strings.TrimRight(baseURL, "/")}
}

func randomName(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Save 将数据写入日期目录，返回相对路径（斜杠分隔）。
func (s *Store) Save(data []byte, ext string) (string, error) {
	name, err := randomName(4) // 4 字节 → 8 位 hex
	if err != nil {
		return "", err
	}
	rel := filepath.Join(time.Now().Format("2006/01/02"), name+"."+ext)
	abs := filepath.Join(s.root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func (s *Store) URL(rel string) string {
	return s.baseURL + "/" + rel
}

func (s *Store) Delete(rel string) error {
	return os.Remove(filepath.Join(s.root, rel))
}

func (s *Store) List() ([]Image, error) {
	var imgs []Image
	err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		imgs = append(imgs, Image{
			ID:         rel,
			URL:        s.URL(rel),
			Size:       info.Size(),
			UploadedAt: info.ModTime(),
		})
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(imgs, func(i, j int) bool { return imgs[i].UploadedAt.After(imgs[j].UploadedAt) })
	return imgs, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go test ./internal/storage/ -v
```

预期：PASS。

- [ ] **Step 5: 提交**

```bash
cd /d/code/image && git add server/internal/storage && git commit -m "feat(server): disk storage module"
```

---

### Task 3: 图片处理模块

**Files:**
- Create: `server/internal/imaging/imaging.go`
- Test: `server/internal/imaging/imaging_test.go`

**Interfaces:**
- Produces: `imaging.Format(data []byte) string`（返回 `jpeg|png|gif|webp|svg` 或空串）；`imaging.Process(data []byte, format string, maxDimension int, convertWebp bool) ([]byte, int, int, error)`（返回处理后字节、宽、高）。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/imaging/imaging_test.go`：

```go
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
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/imaging/ -run TestFormat -v
```

预期：编译失败（`imaging.go` 不存在）。

- [ ] **Step 3: 实现图片处理模块**

创建 `server/internal/imaging/imaging.go`：

```go
package imaging

import (
	"bytes"
	"image"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/chai2010/webp"
	dimg "github.com/disintegration/imaging"
)

// Format 通过魔数嗅探返回图片格式：jpeg|png|gif|webp|svg，无法识别返回空串。
func Format(b []byte) string {
	switch {
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "jpeg"
	case len(b) >= 8 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G':
		return "png"
	case len(b) >= 6 && (string(b[:6]) == "GIF87a" || string(b[:6]) == "GIF89a"):
		return "gif"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "webp"
	case bytes.Contains(bytes.ToLower(b[:min(len(b), 256)]), []byte("<svg")):
		return "svg"
	default:
		return ""
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Process 缩放超限图片并可选转 WebP，返回处理后的字节与新宽高。
// format 为 jpeg|png|webp；gif/svg 不应调用本函数（原样存储）。
func Process(data []byte, format string, maxDimension int, convertWebp bool) ([]byte, int, int, error) {
	src, err := dimg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, err
	}
	bounds := src.Bounds()
	if maxDimension > 0 && (bounds.Dx() > maxDimension || bounds.Dy() > maxDimension) {
		src = dimg.Fit(src, maxDimension, maxDimension, dimg.Lanczos)
		bounds = src.Bounds()
	}

	var out bytes.Buffer
	if convertWebp {
		if err := webp.Encode(&out, src, &webp.Options{Lossless: false, Quality: 82}); err != nil {
			return nil, 0, 0, err
		}
	} else {
		switch format {
		case "jpeg":
			err = dimg.Encode(&out, src, dimg.JPEG, dimg.JPEGQuality(85))
		case "png":
			err = dimg.Encode(&out, src, dimg.PNG)
		case "webp":
			err = webp.Encode(&out, src, &webp.Options{Lossless: false, Quality: 82})
		default:
			err = dimg.Encode(&out, src, dimg.PNG)
		}
		if err != nil {
			return nil, 0, 0, err
		}
	}
	return out.Bytes(), bounds.Dx(), bounds.Dy(), nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go mod tidy && go test ./internal/imaging/ -v
```

预期：PASS。

- [ ] **Step 5: 提交**

```bash
cd /d/code/image && git add server/internal/imaging server/go.mod server/go.sum && git commit -m "feat(server): image processing module"
```

---

### Task 4: API 路由 + 鉴权中间件 + 上传接口

**Files:**
- Create: `server/internal/api/server.go`
- Create: `server/internal/api/middleware.go`
- Create: `server/internal/api/upload.go`
- Test: `server/internal/api/api_test.go`

**Interfaces:**
- Consumes: `config.Config`、`storage.Store`、`imaging.Format/Process`。
- Produces: `api.New(cfg *config.Config, st *storage.Store) http.Handler`；`uploadHandler` 依赖 `cfg`、`st`。

- [ ] **Step 1: 写失败测试**

创建 `server/internal/api/api_test.go`：

```go
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

func newTestServer() (*httptest.Server, *config.Config, *storage.Store) {
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
	ts, _, _ := newTestServer()
	defer ts.Close()
	resp := multipartUpload(t, ts.URL+"/api/upload", "", "a.png", []byte{0x89, 'P', 'N', 'G'})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestUploadRejectsBadToken(t *testing.T) {
	ts, _, _ := newTestServer()
	defer ts.Close()
	resp := multipartUpload(t, ts.URL+"/api/upload", "wrong", "a.png", []byte{0x89, 'P', 'N', 'G'})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/api/ -run TestUploadRequiresToken -v
```

预期：编译失败（`api` 包不存在）。

- [ ] **Step 3: 实现路由与中间件**

创建 `server/internal/api/server.go`：

```go
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"imgserver/internal/config"
	"imgserver/internal/storage"
)

type server struct {
	cfg *config.Config
	st  *storage.Store
}

func New(cfg *config.Config, st *storage.Store) http.Handler {
	s := &server{cfg: cfg, st: st}
	r := chi.NewRouter()
	r.Post("/api/upload", s.requireToken(s.handleUpload))
	r.Get("/api/images", s.requireToken(s.handleList))
	r.Delete("/api/images/{id}", s.requireToken(s.handleDelete))
	r.Get("/admin", s.requireToken(s.handleAdmin))
	r.Handle("/*", http.StripPrefix("/", http.FileServer(http.Dir(cfg.StorageDir))))
	return r
}
```

创建 `server/internal/api/middleware.go`：

```go
package api

import (
	"net/http"
)

func (s *server) requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth-Token") != s.cfg.Token {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
```

- [ ] **Step 4: 运行测试确认通过（鉴权部分）**

```bash
cd server && go test ./internal/api/ -run TestUpload -v
```

预期：TestUploadRequiresToken / TestUploadRejectsBadToken PASS（handleUpload 尚未实现会 404，但鉴权已覆盖）。

> 注意：此步骤需先临时补齐 `handleUpload/handleList/handleDelete/handleAdmin` 空实现或直接进入 Step 5 一起完成。为减少往返，直接完成 Step 5。

- [ ] **Step 5: 实现上传 handler**

创建 `server/internal/api/upload.go`：

```go
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"imgserver/internal/imaging"
)

func extForFormat(format string) string {
	switch format {
	case "jpeg":
		return "jpg"
	default:
		return format
	}
}

type uploadResponse struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Ext      string `json:"ext"`
}

func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, s.cfg.MaxSizeMB<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "read file failed")
		return
	}
	if int64(len(data)) >= s.cfg.MaxSizeMB<<20 {
		writeErr(w, http.StatusRequestEntityTooLarge, "file too large")
		return
	}

	format := imaging.Format(data)
	if format == "" {
		writeErr(w, http.StatusUnsupportedMediaType, "unsupported file type")
		return
	}
	if !allowed(format, s.cfg.AllowedTypes) {
		writeErr(w, http.StatusUnsupportedMediaType, "file type not allowed")
		return
	}

	width, height := 0, 0
	if format == "jpeg" || format == "png" || format == "webp" {
		var out []byte
		out, width, height, err = imaging.Process(data, format, s.cfg.MaxDimension, s.cfg.ConvertWebp)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "image processing failed")
			return
		}
		data = out
		if s.cfg.ConvertWebp {
			format = "webp"
		}
	}

	ext := extForFormat(format)
	rel, err := s.st.Save(data, ext)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "save failed")
		return
	}

	resp := uploadResponse{
		ID:       rel,
		URL:      s.st.URL(rel),
		Filename: header.Filename,
		Size:     int64(len(data)),
		Width:    width,
		Height:   height,
		Ext:      ext,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func allowed(format string, types []string) bool {
	for _, t := range types {
		if t == format {
			return true
		}
	}
	return false
}
```

创建 `server/internal/api/list.go`（占位，Task 5 补全）：

```go
package api

import (
	"net/http"
	"strconv"
)

func (s *server) handleList(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 {
		pageSize = 50
	}
	_ = page
	_ = pageSize
	writeErr(w, http.StatusNotImplemented, "not implemented")
}
```

创建 `server/internal/api/delete.go`（占位）：

```go
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *server) handleDelete(w http.ResponseWriter, r *http.Request) {
	_ = chi.URLParam(r, "id")
	writeErr(w, http.StatusNotImplemented, "not implemented")
}
```

创建 `server/internal/api/admin.go`（占位）：

```go
package api

import "net/http"

func (s *server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<h1>admin</h1>"))
}
```

- [ ] **Step 6: 运行测试确认通过**

```bash
cd server && go mod tidy && go test ./internal/api/ -v
```

预期：PASS。

- [ ] **Step 7: 提交**

```bash
cd /d/code/image && git add server/internal/api server/go.mod server/go.sum && git commit -m "feat(server): upload API with token auth"
```

---

### Task 5: 列表与删除接口

**Files:**
- Modify: `server/internal/api/list.go`
- Modify: `server/internal/api/delete.go`
- Modify: `server/internal/api/api_test.go`（追加测试）

**Interfaces:**
- Consumes: `storage.Store.List()`、`storage.Store.Delete()`。
- Produces: 列表响应 `{images:[{id,url,size,uploadedAt}],total,page,pageSize}`。

- [ ] **Step 1: 追加失败测试**

在 `server/internal/api/api_test.go` 末尾追加：

```go
func TestListAndDelete(t *testing.T) {
	ts, cfg, _ := newTestServer()
	defer ts.Close()

	// 上传一张真实 PNG
	png := makeTinyPNG(t)
	resp := multipartUpload(t, ts.URL+"/api/upload", cfg.Token, "a.png", png)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status = %d", resp.StatusCode)
	}

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
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("delete status = %d", delResp.StatusCode)
	}
}

func TestListRequiresToken(t *testing.T) {
	ts, _, _ := newTestServer()
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/api/images")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}
```

在 `api_test.go` 顶部追加辅助函数与 import：

```go
import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
)

func makeTinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/api/ -run TestListAndDelete -v
```

预期：FAIL（列表返回 501 / 删除返回 501）。

- [ ] **Step 3: 实现列表与删除**

改写 `server/internal/api/list.go`：

```go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"imgserver/internal/storage"
)

type listResponse struct {
	Images   []storage.Image `json:"images"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
}

func (s *server) handleList(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 {
		pageSize = 50
	}

	imgs, err := s.st.List()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list failed")
		return
	}

	total := len(imgs)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pageImgs := imgs[start:end]

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(listResponse{
		Images:   pageImgs,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}
```

改写 `server/internal/api/delete.go`：

```go
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "missing id")
		return
	}
	if err := s.st.Delete(id); err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusOK)
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go test ./internal/api/ -v
```

预期：PASS。

- [ ] **Step 5: 提交**

```bash
cd /d/code/image && git add server/internal/api && git commit -m "feat(server): list and delete API"
```

---

### Task 6: 静态托管与管理界面

**Files:**
- Create: `server/internal/api/web/admin/index.html`（`go:embed` 要求文件在包目录下）
- Modify: `server/internal/api/server.go`（用 go:embed 内嵌管理界面）
- Modify: `server/internal/api/api_test.go`（追加静态托管测试）

**Interfaces:**
- Consumes: `config.StorageDir`。
- Produces: `GET /admin` 返回内嵌 HTML；`GET /{path}` 公开托管存储目录文件。

- [ ] **Step 1: 追加失败测试**

在 `api_test.go` 末尾追加：

```go
func TestStaticServing(t *testing.T) {
	ts, cfg, _ := newTestServer()
	defer ts.Close()

	png := makeTinyPNG(t)
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
	ts, _, _ := newTestServer()
	defer ts.Close()
	if resp, err := http.Get(ts.URL + "/admin"); err == nil {
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
	} else {
		t.Fatal(err)
	}
}

func relFromURL(u string) string {
	// 取 baseURL 之后的部分，例如 http://localhost:8080/2026/09/03/x.png -> 2026/09/03/x.png
	for i := 0; i < 3; i++ {
		if idx := indexByte(u, '/'); idx >= 0 {
			u = u[idx+1:]
		}
	}
	if idx := indexByte(u, '/'); idx >= 0 {
		return u[idx+1:]
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
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd server && go test ./internal/api/ -run TestStaticServing -v
```

预期：TestStaticServing FAIL（静态路由尚未正确挂载或未找到文件）；TestAdminRequiresToken PASS（鉴权中间件已存在）。

- [ ] **Step 3: 内嵌管理界面并调整路由**

创建 `server/web/admin/index.html`：

```html
<!doctype html>
<html lang="zh">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>图床管理</title>
  <style>
    body { font-family: system-ui, sans-serif; margin: 24px; color: #222; }
    .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(160px, 1fr)); gap: 12px; }
    .card { border: 1px solid #ddd; border-radius: 8px; padding: 8px; }
    .card img { width: 100%; height: 120px; object-fit: cover; border-radius: 4px; }
    .card button { margin-top: 6px; }
  </style>
</head>
<body>
  <h1>图床管理</h1>
  <input id="token" placeholder="X-Auth-Token" />
  <button onclick="load()">加载图片</button>
  <div class="grid" id="grid"></div>
  <script>
    async function api(path, method = 'GET') {
      const res = await fetch(path, { method, headers: { 'X-Auth-Token': document.getElementById('token').value } });
      return res.json();
    }
    async function load() {
      const data = await api('/api/images');
      const grid = document.getElementById('grid');
      grid.innerHTML = '';
      for (const img of (data.images || [])) {
        const card = document.createElement('div');
        card.className = 'card';
        card.innerHTML = `<img src="/${img.id}" /><button data-id="${img.id}">删除</button>`;
        card.querySelector('button').onclick = async (e) => {
          await api('/api/images/' + encodeURIComponent(img.id), 'DELETE');
          load();
        };
        grid.appendChild(card);
      }
    }
  </script>
</body>
</html>
```

改写 `server/internal/api/server.go` 顶部与 `handleAdmin`：

```go
package api

import (
	"embed"
	"net/http"

	"github.com/go-chi/chi/v5"

	"imgserver/internal/config"
	"imgserver/internal/storage"
)

//go:embed web/admin/index.html
var adminHTML []byte
```

> 说明：`//go:embed` 只能引用相对当前包目录的路径，因此 `web/` 需放在 `internal/api/` 下。将管理界面实际放到 `server/internal/api/web/admin/index.html`，embed 指令写 `web/admin/index.html`。

将 `admin.go` 改为：

```go
package api

import "net/http"

func (s *server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(adminHTML)
}
```

把管理界面文件实际创建在 `server/internal/api/web/admin/index.html`（内容同上），并删除 `server/web/admin/index.html` 占位。

- [ ] **Step 4: 运行测试确认通过**

```bash
cd server && go test ./internal/api/ -v
```

预期：PASS。

- [ ] **Step 5: 提交**

```bash
cd /d/code/image && git add server/internal/api && git commit -m "feat(server): static serving and embedded admin page"
```

---

### Task 7: 入口 main.go + 示例配置 + README

**Files:**
- Create: `server/cmd/imgserver/main.go`
- Create: `server/config.example.yaml`
- Create: `server/README.md`
- Create: `server/.gitignore`

**Interfaces:**
- Consumes: `config.Load/Default`、`storage.New`、`api.New`。
- Produces: 可执行二进制 `imgserver`。

- [ ] **Step 1: 写 main.go**

创建 `server/cmd/imgserver/main.go`：

```go
package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"imgserver/internal/api"
	"imgserver/internal/config"
	"imgserver/internal/storage"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	var cfg *config.Config
	if _, err := os.Stat(*cfgPath); err == nil {
		var err error
		cfg, err = config.Load(*cfgPath)
		if err != nil {
			log.Fatalf("load config: %v", err)
		}
	} else {
		cfg = config.Default()
		log.Printf("config %s not found, using defaults", *cfgPath)
	}

	if err := os.MkdirAll(cfg.StorageDir, 0o755); err != nil {
		log.Fatalf("mkdir storage: %v", err)
	}

	st := storage.New(cfg.StorageDir, cfg.BaseURL)
	log.Printf("imgserver listening on %s, storage=%s baseURL=%s", cfg.Listen, cfg.StorageDir, cfg.BaseURL)
	if err := http.ListenAndServe(cfg.Listen, api.New(cfg, st)); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 2: 写示例配置与说明**

创建 `server/config.example.yaml`：

```yaml
listen: ":8080"
storageDir: "./storage"
baseURL: "https://img.example.com"
token: "change-me"
maxDimension: 2560
maxSizeMB: 5
convertWebp: false
allowedTypes: ["jpeg", "png", "gif", "webp", "svg"]
```

创建 `server/.gitignore`：

```
storage/
config.yaml
imgserver
```

创建 `server/README.md`：

```markdown
# 图床服务端

Go 实现的个人图床服务。上传、静态托管、图片处理、Token 鉴权与内嵌管理界面。

## 运行

    go run ./cmd/imgserver -config config.yaml

## 配置

复制 `config.example.yaml` 为 `config.yaml` 并修改 token、baseURL。

## 接口

- `POST /api/upload`（multipart `file`，需 `X-Auth-Token`）
- `GET /api/images?page=1&pageSize=50`（需 token）
- `DELETE /api/images/{id}`（需 token）
- `GET /admin`（需 token）
- `GET /{path}`（公开静态图片）

## 交叉编译（VPS）

    GOOS=linux GOARCH=amd64 go build -o imgserver ./cmd/imgserver
```

- [ ] **Step 3: 构建并全量测试**

```bash
cd server && go build ./... && go test ./... -count=1
```

预期：build 成功、全部测试 PASS。

- [ ] **Step 4: 手动冒烟**

```bash
cd server && go run ./cmd/imgserver -config config.example.yaml
```

另一终端：

```bash
curl -X POST -H "X-Auth-Token: change-me" -F "file=@some.png" http://localhost:8080/api/upload
```

预期：返回 JSON 含 `url`。

- [ ] **Step 5: 提交**

```bash
cd /d/code/image && git add server && git commit -m "feat(server): entrypoint, example config, README"
```

---

## 完成标准

- [ ] `cd server && go test ./... -count=1` 全部通过。
- [ ] `go build ./...` 无错误。
- [ ] 上传 → 列表 → 静态访问 → 删除 全链路通过冒烟测试。
- [ ] 鉴权：无 token / 错 token 均返回 401；静态读公开可访问。
