// palmterm: スマホのブラウザから tmux を操作するための小さなサーバー。
// 画面（web/dist）を埋め込んで配り、WebSocket で tmux の端末をつなぎ、
// コピー用に tmux の履歴を返す。
package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed all:web/dist
var webDist embed.FS

type server struct {
	token          string
	defaultSession string
	origins        []string
	uploadDir      string
}

var sessionNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func main() {
	listen := flag.String("listen", "127.0.0.1:7681", "待ち受けるアドレス")
	session := flag.String("session", "main", "既定でつなぐ tmux のセッション名")
	token := flag.String("token", "", "ログイン用のトークン（省略時は ~/.config/palmterm/token を使い、なければ作る）")
	allowOrigin := flag.String("allow-origin", "", "WebSocket を許す別の Origin（カンマ区切り。開発時の Vite 用など）")
	uploadDir := flag.String("upload-dir", "", "アップロードしたファイルの保存先（省略時は ~/.cache/palmterm/uploads）")
	flag.Parse()

	if !sessionNamePattern.MatchString(*session) {
		log.Fatalf("セッション名に使えない文字があります: %q", *session)
	}
	tok := *token
	if tok == "" {
		tok = os.Getenv("PALMTERM_TOKEN")
	}
	if tok == "" {
		var err error
		if tok, err = loadOrCreateToken(); err != nil {
			log.Fatal(err)
		}
	}

	s := &server{token: tok, defaultSession: *session, uploadDir: *uploadDir}
	if s.uploadDir == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			log.Fatal(err)
		}
		s.uploadDir = filepath.Join(cache, "palmterm", "uploads")
	}
	if *allowOrigin != "" {
		s.origins = strings.Split(*allowOrigin, ",")
	}

	dist, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth", s.handleAuth)
	mux.Handle("GET /ws", s.requireAuth(http.HandlerFunc(s.handleTerminal)))
	mux.Handle("GET /api/history", s.requireAuth(http.HandlerFunc(s.handleHistory)))
	mux.Handle("GET /api/pane", s.requireAuth(http.HandlerFunc(s.handlePane)))
	mux.Handle("POST /api/upload", s.requireAuth(http.HandlerFunc(s.handleUpload)))
	mux.Handle("GET /", s.requireAuth(cacheAssets(http.FileServerFS(dist))))

	log.Printf("palmterm: http://%s/auth?token=%s を開いてください", *listen, tok)
	log.Fatal(http.ListenAndServe(*listen, mux))
}

func loadOrCreateToken() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "palmterm", "token")
	if b, err := os.ReadFile(path); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			return t, nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	t := hex.EncodeToString(buf)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(t+"\n"), 0o600); err != nil {
		return "", err
	}
	return t, nil
}

// セッション名は URL で指定でき、なければ既定のものを使う。
func (s *server) sessionFrom(r *http.Request) (string, error) {
	name := r.URL.Query().Get("session")
	if name == "" {
		return s.defaultSession, nil
	}
	if !sessionNamePattern.MatchString(name) {
		return "", fmt.Errorf("セッション名に使えない文字があります: %q", name)
	}
	return name, nil
}

// /assets/ のファイル名には中身のハッシュが入っているので、長くキャッシュさせる（フォントが大きい）。
func cacheAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		next.ServeHTTP(w, r)
	})
}
