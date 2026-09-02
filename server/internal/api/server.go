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
	r.Delete("/api/images/*", s.requireToken(s.handleDelete))
	r.Get("/admin", s.requireToken(s.handleAdmin))
	r.Handle("/*", http.StripPrefix("/", http.FileServer(http.Dir(cfg.StorageDir))))
	return r
}
