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
