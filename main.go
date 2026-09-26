// palmterm: スマホのブラウザから tmux を操作するための小さなサーバー。
// 画面（web/dist）を埋め込んで配り、WebSocket で tmux の端末をつなぎ、
// コピー用に tmux の履歴を返す。
package main

import (
	"crypto/rand"
	"crypto/tls"
	"embed"
	"encoding/hex"
	"errors"
	"flag"
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
	token     string
	session   string // つなぐ tmux のセッション
	origins   []string
	uploadDir string
}

var sessionNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func main() {
	// 説明は設定ファイル（言語）を読む前に出るので、既定の言語の英語で書く。
	listen := flag.String("listen", "127.0.0.1:7681", "address to listen on (HTTP)")
	session := flag.String("session", "main", "tmux session to attach to")
	token := flag.String("token", "", "login token (default: ~/.config/palmterm/token, created on first run)")
	allowOrigin := flag.String("allow-origin", "", "extra WebSocket origins, comma-separated (e.g. for the Vite dev server)")
	uploadDir := flag.String("upload-dir", "", "where uploaded files are saved (default: ~/.cache/palmterm/uploads)")
	configPath := flag.String("config", "", "configuration file (default: ~/.config/palmterm/config.toml)")
	lan := flag.String("lan", "", "also listen on this address over HTTPS for the local network, e.g. :7682 (self-signed certificate in ~/.config/palmterm)")
	flag.Parse()

	if *configPath == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			log.Fatal(err)
		}
		*configPath = filepath.Join(dir, "palmterm", "config.toml")
	}
	settings = newConfigStore(*configPath)

	if !sessionNamePattern.MatchString(*session) {
		log.Fatalf(tr("セッション名に使えない文字があります: %q", "Invalid session name: %q"), *session)
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

	s := &server{token: tok, session: *session, uploadDir: *uploadDir}
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

	handler := s.routes()
	if *lan != "" {
		go serveLAN(*lan, handler, tok)
	}
	log.Printf(tr("palmterm: http://%s/auth?token=%s を開いてください", "palmterm: open http://%s/auth?token=%s"), *listen, tok)
	log.Fatal(http.ListenAndServe(*listen, handler))
}

// LAN 向けに HTTPS で待ち受ける（証明書は自分で署名したもの）。
func serveLAN(addr string, handler http.Handler, token string) {
	dir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	host, _ := os.Hostname()
	ips := lanAddresses()
	cert, err := lanCertificate(filepath.Join(dir, "palmterm"), ips, host)
	if err != nil {
		log.Fatalf(tr("LAN 用の証明書を用意できませんでした: %v", "Could not prepare the LAN certificate: %v"), err)
	}
	srv := &http.Server{Addr: addr, Handler: handler, TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}}}
	for _, u := range lanURLs(addr, ips, token) {
		log.Printf(tr("palmterm（LAN）: %s を開いてください（最初は証明書の警告が出るので、先へ進む）",
			"palmterm (LAN): open %s (the browser warns about the certificate the first time; proceed)"), u)
	}
	log.Fatal(srv.ListenAndServeTLS("", ""))
}

func (s *server) routes() http.Handler {
	dist, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth", s.handleAuth)
	mux.Handle("GET /ws", s.requireAuth(http.HandlerFunc(s.handleTerminal)))
	mux.Handle("GET /api/config", s.requireAuth(http.HandlerFunc(s.handleConfig)))
	mux.Handle("GET /api/history", s.requireAuth(http.HandlerFunc(s.handleHistory)))
	mux.Handle("GET /api/pane", s.requireAuth(http.HandlerFunc(s.handlePane)))
	mux.Handle("GET /api/windows", s.requireAuth(http.HandlerFunc(s.handleWindows)))
	mux.Handle("POST /api/tmux", s.requireAuth(http.HandlerFunc(s.handleTmuxAction)))
	mux.Handle("POST /api/upload", s.requireAuth(http.HandlerFunc(s.handleUpload)))
	mux.Handle("GET /api/upload/{name}", s.requireAuth(http.HandlerFunc(s.handleUploadFile)))
	mux.Handle("GET /", s.requireAuth(cacheAssets(http.FileServerFS(dist))))
	return mux
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

// /assets/ のファイル名には中身のハッシュが入っているので、長くキャッシュさせる（フォントが大きい）。
func cacheAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		next.ServeHTTP(w, r)
	})
}
