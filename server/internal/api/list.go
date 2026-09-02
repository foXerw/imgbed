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
