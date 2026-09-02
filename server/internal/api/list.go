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
