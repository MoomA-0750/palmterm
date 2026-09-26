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
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const maxUploadBytes = 64 << 20

var unsafeNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Claude Code が画像として読める形式。これ以外の画像（HEIC など）は JPEG に変換してから渡す。
var claudeImageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true}

type uploadedFile struct {
	Path string `json:"path"` // 端末に入れるパス（変換したときは JPEG のほう）
	Name string `json:"name"` // GET /api/upload/{name} で取るときの名前
}

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
	saved := make([]uploadedFile, 0, len(files))
	for _, fh := range files {
		path, err := s.saveUpload(fh)
		if err == nil && !claudeImageExts[strings.ToLower(filepath.Ext(path))] {
			var jpeg string
			if jpeg, err = convertToJPEG(path); err != nil {
				os.Remove(path) // 使えないものは残さない
			}
			path = jpeg
		}
		if err != nil {
			log.Printf("アップロードを保存できませんでした（%s）: %v", fh.Filename, err)
			http.Error(w, fh.Filename+" を保存できませんでした: "+err.Error(), http.StatusInternalServerError)
			return
		}
		saved = append(saved, uploadedFile{Path: path, Name: filepath.Base(path)})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]uploadedFile{"files": saved})
}

// ffmpeg で JPEG に変換し、変換後のパスを返す（元のファイルは残す）。
// ImageMagick は HEIC を読めない環境があり、ffmpeg ならタイル分割された HEIC もつないで出せる。
func convertToJPEG(src string) (string, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return "", fmt.Errorf("%s は Claude Code が読めない形式で、変換に使う ffmpeg が見つかりません", filepath.Ext(src))
	}
	dst := strings.TrimSuffix(src, filepath.Ext(src)) + ".jpg"
	out, err := exec.Command("ffmpeg", "-v", "error", "-n", "-i", src, "-frames:v", "1", "-q:v", "2", dst).CombinedOutput()
	if err != nil {
		os.Remove(dst)
		log.Printf("ffmpeg で %s を変換できませんでした: %v\n%s", src, err, out)
		// 画面に出すのは最後の1行（原因の要約）だけにする。
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		return "", fmt.Errorf("JPEG に変換できませんでした（%s）", lines[len(lines)-1])
	}
	return dst, nil
}

var uploadNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// 保存した画像を返す（添付欄のプレビュー用。HEIC はブラウザで表示できないことがあるので、変換後の JPEG を見せる）。
// 名前は保存先の直下のファイルに限る。
func (s *server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !uploadNamePattern.MatchString(name) || strings.Contains(name, "..") {
		http.Error(w, "名前が正しくありません", http.StatusBadRequest)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, filepath.Join(s.uploadDir, name))
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
