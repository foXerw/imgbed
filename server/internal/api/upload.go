package api

import (
	"encoding/json"
	"errors"
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
	// 先用 MaxBytesReader 限制整个请求体，防止未限流的多部分上传写满临时磁盘。
	const overhead = 1 << 20 // 多部分边界与头部的余量
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxSizeMB<<20+overhead)
	if err := r.ParseMultipartForm(overhead); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeErr(w, http.StatusRequestEntityTooLarge, "file too large")
		} else {
			writeErr(w, http.StatusBadRequest, "invalid multipart form")
		}
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	// 多读 1 字节以区分「恰好等于上限」与「超过上限」。
	data, err := io.ReadAll(io.LimitReader(file, s.cfg.MaxSizeMB<<20+1))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "read file failed")
		return
	}
	if int64(len(data)) > s.cfg.MaxSizeMB<<20 {
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
