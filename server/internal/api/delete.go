package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *server) handleDelete(w http.ResponseWriter, r *http.Request) {
	_ = chi.URLParam(r, "id")
	writeErr(w, http.StatusNotImplemented, "not implemented")
}
