package api

import (
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (s *server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "*")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "missing id")
		return
	}
	if p := path.Clean(id); p == ".." || strings.HasPrefix(p, "../") || path.IsAbs(id) {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.st.Delete(id); err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusOK)
}
