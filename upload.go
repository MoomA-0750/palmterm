package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const maxUploadBytes = 64 << 20

var unsafeNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// 画面から送られたファイル（フォームの file、複数可）を保存し、保存したパスを返す。
// スマホの写真を Claude Code などに渡すため。パスはそのまま端末に入力できるよう、空白などを含まない名前にする。
func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Error(w, "ファイルを受け取れませんでした: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()
	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		http.Error(w, "ファイルがありません", http.StatusBadRequest)
		return
	}
	if err := os.MkdirAll(s.uploadDir, 0o700); err != nil {
		http.Error(w, "保存先を作れませんでした: "+err.Error(), http.StatusInternalServerError)
		return
	}
	paths := make([]string, 0, len(files))
	for _, fh := range files {
		path, err := s.saveUpload(fh)
		if err != nil {
			log.Printf("アップロードを保存できませんでした: %v", err)
			http.Error(w, "保存できませんでした: "+err.Error(), http.StatusInternalServerError)
			return
		}
		paths = append(paths, path)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"paths": paths})
}

func (s *server) saveUpload(fh *multipart.FileHeader) (string, error) {
	src, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	base := time.Now().Format("20060102-150405") + "-" + safeFileName(fh.Filename)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 0; i < 100; i++ {
		name := base
		if i > 0 {
			name = fmt.Sprintf("%s-%d%s", stem, i, ext)
		}
		path := filepath.Join(s.uploadDir, name)
		dst, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(dst, src); err != nil {
			dst.Close()
			os.Remove(path)
			return "", err
		}
		return path, dst.Close()
	}
	return "", fmt.Errorf("同じ名前のファイルが多すぎます: %s", base)
}

// 端末にそのまま打てる名前にする（英数字と . _ - 以外は _ に）。拡張子は残す。
func safeFileName(name string) string {
	name = filepath.Base(name)
	ext := strings.ToLower(unsafeNameChars.ReplaceAllString(filepath.Ext(name), ""))
	stem := strings.Trim(unsafeNameChars.ReplaceAllString(strings.TrimSuffix(name, filepath.Ext(name)), "_"), "._-")
	if len(stem) > 60 {
		stem = stem[:60]
	}
	if stem == "" {
		stem = "file"
	}
	if ext == "." {
		ext = ""
	}
	return stem + ext
}
