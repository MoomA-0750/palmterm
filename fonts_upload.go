package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 画面から足したフォントは、設定ファイルのあるフォルダの fonts/（既定は ~/.config/palmterm/fonts/）に置く。
// [font] に file = "fonts/名前" と書けば使える。

// 1つのフォントの大きさの上限。日本語のフォントでも収まる大きさにする。
var maxFontBytes int64 = 32 << 20

// 画面から足せるフォントの拡張子（.ttc はブラウザで読めないことがあるので受け付けない）。
var fontUploadExts = map[string]bool{".ttf": true, ".otf": true, ".woff": true, ".woff2": true}

type savedFont struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func fontsDir() string {
	return filepath.Join(filepath.Dir(settings.path), "fonts")
}

// 保存したフォントの一覧と、PC に入っているフォントの名前（パスは出さない）、組み込みのフォントの名前、
// [font] の file にいま書いてあるもの（stack の書き方なら並びの先頭）。
func (s *server) handleFonts(w http.ResponseWriter, r *http.Request) {
	fonts, err := listFonts(fontsDir())
	if err != nil {
		http.Error(w, tr("フォントの一覧を取れませんでした: ", "Could not list the fonts: ")+err.Error(), http.StatusInternalServerError)
		return
	}
	cfg, _ := settings.get()
	w.Header().Set("Content-Type", "application/json")
	file := cfg.Font.File
	if cfg.Font.Stack != nil {
		file = "" // stack の書き方では、並びの先頭を「使用中」にする
		if len(cfg.Font.Stack) > 0 {
			file = cfg.Font.Stack[0]
		}
	}
	json.NewEncoder(w).Encode(map[string]any{"fonts": fonts, "system": systemFonts(), "builtin": builtinFont, "dir": fontsDir(), "file": file})
}

func listFonts(dir string) ([]savedFont, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []savedFont{}, nil
	}
	if err != nil {
		return nil, err
	}
	fonts := []savedFont{}
	for _, e := range entries {
		if !e.Type().IsRegular() || !fontUploadExts[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		fonts = append(fonts, savedFont{Name: e.Name(), Size: info.Size()})
	}
	sort.Slice(fonts, func(i, j int) bool { return fonts[i].Name < fonts[j].Name })
	return fonts, nil
}

// 画面から送られたフォント（フォームの file を1つ）を fonts/ に保存する。
// 拡張子と中身の先頭の両方でフォントか確かめ、名前は fonts/ の直下のファイルにしかならない形に直す。
// 同じ名前があれば上書きせず、後ろに番号を付ける。
func (s *server) handleFontUpload(w http.ResponseWriter, r *http.Request) {
	tooBig := fmt.Sprintf(tr("フォントが大きすぎます（%d MB まで）", "The font is too large (up to %d MB)"), maxFontBytes>>20)
	r.Body = http.MaxBytesReader(w, r.Body, maxFontBytes+1<<20) // フォームの見出しの分を足す
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			http.Error(w, tooBig, http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, tr("ファイルを受け取れませんでした: ", "Could not receive the file: ")+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()
	files := r.MultipartForm.File["file"]
	if len(files) != 1 {
		http.Error(w, tr("フォントを1つ選んでください", "Choose one font file"), http.StatusBadRequest)
		return
	}
	fh := files[0]
	if fh.Size > maxFontBytes {
		http.Error(w, tooBig, http.StatusRequestEntityTooLarge)
		return
	}
	name := safeFileName(fh.Filename)
	notFont := fmt.Sprintf(tr("%s はフォントのファイル（.ttf・.otf・.woff・.woff2）ではありません", "%s is not a font file (.ttf, .otf, .woff, .woff2)"), fh.Filename)
	if !fontUploadExts[filepath.Ext(name)] {
		http.Error(w, notFont, http.StatusBadRequest)
		return
	}
	src, err := fh.Open()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer src.Close()
	head := make([]byte, 4)
	if _, err := io.ReadFull(src, head); err != nil || !isWebFont(head) {
		http.Error(w, notFont, http.StatusBadRequest)
		return
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	saved, err := saveFont(fontsDir(), name, src)
	if err != nil {
		log.Printf(tr("フォントを保存できませんでした（%s）: %v", "Could not save the font (%s): %v"), fh.Filename, err)
		http.Error(w, fmt.Sprintf(tr("%s を保存できませんでした: ", "Could not save %s: "), fh.Filename)+err.Error(), http.StatusInternalServerError)
		return
	}
	forgetFont() // 上書きはしないが、[font] の file が今できたファイルを指していることがある
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"name": saved, "file": "fonts/" + saved})
}

// ブラウザが読める1つのフォントのファイルの先頭か（.ttc は除く）。
func isWebFont(head []byte) bool {
	switch string(head) {
	case "wOF2", "wOFF", "OTTO", "\x00\x01\x00\x00", "true":
		return true
	}
	return false
}

func saveFont(dir, name string, src io.Reader) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 0; i < 100; i++ {
		n := name
		if i > 0 {
			n = fmt.Sprintf("%s-%d%s", stem, i, ext)
		}
		path := filepath.Join(dir, n)
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
		return n, dst.Close()
	}
	return "", fmt.Errorf(tr("同じ名前のフォントが多すぎます: %s", "Too many fonts with the same name: %s"), name)
}

// 保存したフォントを消す。名前は fonts/ の直下のフォントのファイルに限る。
func (s *server) handleFontDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !uploadNamePattern.MatchString(req.Name) || strings.Contains(req.Name, "..") || !fontUploadExts[strings.ToLower(filepath.Ext(req.Name))] {
		http.Error(w, tr("名前が正しくありません", "Invalid name"), http.StatusBadRequest)
		return
	}
	path := filepath.Join(fontsDir(), req.Name)
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		http.Error(w, tr("そのフォントはありません", "No such font"), http.StatusNotFound)
		return
	}
	if err := os.Remove(path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	forgetFont()
	w.WriteHeader(http.StatusNoContent)
}
