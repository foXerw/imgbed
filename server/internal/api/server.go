package api

import (
	_ "embed"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"imgserver/internal/config"
	"imgserver/internal/storage"
)

//go:embed web/admin/index.html
var adminHTML []byte

// noListFileSystem 阻止 http.FileServer 输出目录列表，防止图片 ID 被枚举。
type noListFileSystem struct {
	fs http.FileSystem
}

func (n noListFileSystem) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if info.IsDir() {
		f.Close()
		return nil, os.ErrNotExist
	}
	return f, nil
}

type server struct {
	cfg *config.Config
	st  *storage.Store
}

func New(cfg *config.Config, st *storage.Store) http.Handler {
	s := &server{cfg: cfg, st: st}
	r := chi.NewRouter()
	r.Post("/api/upload", s.requireToken(s.handleUpload))
	r.Get("/api/images", s.requireToken(s.handleList))
	r.Delete("/api/images/*", s.requireToken(s.handleDelete))
	r.Get("/admin", s.requireToken(s.handleAdmin))
	r.Handle("/*", http.StripPrefix("/", http.FileServer(noListFileSystem{http.Dir(cfg.StorageDir)})))
	return r
}
