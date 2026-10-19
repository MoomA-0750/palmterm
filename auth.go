package main

import (
	"context"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const cookieName = "palmterm_token"

type deviceKey struct{}

// GET /auth?token=… は、使い捨てのトークンが使えれば、端末の名前の欄と「ログイン」ボタンのページを出す。
// 開いただけではトークンを使わない（チャットのアプリなどがリンクのプレビューを取りに来ても使い切られない）。
// ボタンで POST /auth すると、トークンを使い切って、この端末を記録して Cookie を入れ、以降はそれで通す。
// Cookie にはトークンではなく端末ごとのランダムな値を入れる。Cookie はポートで分けられず、同じホスト名の
// 別のポートのサービスにも送られるので、そこから漏れても新しい端末でのログインには使えず、
// 漏れた端末の分だけ取り消せる。
func (s *server) handleAuthPage(w http.ResponseWriter, r *http.Request) {
	tok := loginTokenFrom(r.URL.Query().Get("token"))
	if !s.logins.valid(tok) {
		badLoginToken(w)
		return
	}
	// トークンを含む URL をほかへ渡さない・キャッシュさせない・ほかのページに埋め込ませない。
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	loginPage.Execute(w, map[string]string{
		"Lang":   tr("ja", "en"),
		"Title":  tr("palmterm にログイン", "Log in to palmterm"),
		"Label":  tr("この端末の名前", "Name for this device"),
		"Hint":   tr("「ログイン中の端末」の一覧に出ます。空なら自動の名前になります。", "Shown in the list of logged-in devices. Leave empty to use the automatic name."),
		"Auto":   deviceName(r.UserAgent()),
		"Button": tr("ログイン", "Log in"),
		"Token":  tok,
	})
}

func (s *server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !s.logins.use(r.PostForm.Get("token")) {
		badLoginToken(w)
		return
	}
	secret, err := s.devices.login(r.UserAgent(), r.PostForm.Get("name"))
	if err != nil {
		// 記録をファイルに書けなくても、動いている間はログインできる（再起動するとログインし直し）。
		log.Printf(tr("ログインした端末の記録を書けませんでした: %v", "Could not save the logged-in devices: %v"), err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    secret,
		Path:     "/",
		MaxAge:   int(loginLifetime / time.Second),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		// Strict だと、ほかのアプリ（チャットなど）から開いたリンクや、証明書の警告の画面から進んだときに
		// Cookie が送られず、ログイン済みでもログインを求める画面になる。Lax でも、別のサイトからの POST
		// （tmux の操作・アップロード・ログアウト）には付かず、WebSocket は Origin を確かめるので、勝手には操作されない。
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ログインしていない画面の欄に貼られたものからトークンを取り出す。ログイン用の URL を丸ごと貼っても、
// トークンだけを貼ってもよい（ホーム画面のアプリにはアドレスバーが無く、URL を開けないため）。
func loginTokenFrom(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "?"); i >= 0 {
		if q, err := url.ParseQuery(s[i+1:]); err == nil {
			return q.Get("token")
		}
		return ""
	}
	return s
}

func badLoginToken(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, tr("このログイン用の URL は使えません（使用済みか、出してから5分たっています）。PC で palmterm url を実行し、新しい URL を開いてください",
		"This login URL no longer works (already used, or older than 5 minutes). Run palmterm url on the PC and open the new URL"), http.StatusUnauthorized)
}

var loginPage = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="referrer" content="no-referrer">
<title>palmterm</title>
<style>
:root { color-scheme: light dark; font-family: system-ui, sans-serif; }
body { margin: 0; min-height: 100vh; display: flex; align-items: center; justify-content: center; }
form { width: min(360px, calc(100vw - 32px)); display: flex; flex-direction: column; gap: 12px; }
h1 { font-size: 1.25rem; margin: 0 0 4px; }
label { font-weight: 600; }
input, button { font: inherit; font-size: 16px; padding: 10px 12px; border-radius: 8px; }
input { border: 1px solid #8888; }
input::placeholder { opacity: .5; }
p { margin: 0; font-size: .875rem; opacity: .7; }
button { border: 0; background: #2563eb; color: #fff; font-weight: 600; }
</style>
</head>
<body>
<form id="login" method="post" action="/auth">
<h1>{{.Title}}</h1>
<label for="name">{{.Label}}</label>
<input id="name" name="name" placeholder="{{.Auto}}" autocomplete="off">
<p>{{.Hint}}</p>
<input type="hidden" name="token" value="{{.Token}}">
<button type="submit">{{.Button}}</button>
</form>
</body>
</html>
`))

func notLoggedInPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src 'self'; manifest-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	pastePage.Execute(w, map[string]string{
		"Lang":   tr("ja", "en"),
		"Title":  tr("palmterm にログイン", "Log in to palmterm"),
		"Hint":   tr("PC で palmterm url を実行し、出たログイン用の URL（QR コード）を開いてください。ホーム画面から開いた palmterm では、その URL をここに貼って「次へ」を押します。", "Run palmterm url on the PC and open the login URL (or scan the QR code) it prints. In palmterm opened from the home screen, paste that URL here and tap Next."),
		"Label":  tr("ログイン用の URL", "Login URL"),
		"Button": tr("次へ", "Next"),
	})
}

// ログインしていないときの画面。manifest も読ませておくと、この画面からでもホーム画面に追加できる。
var pastePage = template.Must(template.New("paste").Parse(`<!doctype html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="referrer" content="no-referrer">
<meta name="theme-color" content="#1e1e1e">
<link rel="manifest" href="/manifest.webmanifest">
<link rel="apple-touch-icon" href="/icons/icon-180.png">
<meta name="apple-mobile-web-app-capable" content="yes">
<meta name="mobile-web-app-capable" content="yes">
<meta name="apple-mobile-web-app-title" content="palmterm">
<title>palmterm</title>
<style>
:root { color-scheme: light dark; font-family: system-ui, sans-serif; }
body { margin: 0; min-height: 100vh; display: flex; align-items: center; justify-content: center; }
form { width: min(360px, calc(100vw - 32px)); display: flex; flex-direction: column; gap: 12px; }
h1 { font-size: 1.25rem; margin: 0 0 4px; }
label { font-weight: 600; }
input, button { font: inherit; font-size: 16px; padding: 10px 12px; border-radius: 8px; }
input { border: 1px solid #8888; }
p { margin: 0; font-size: .875rem; opacity: .7; }
button { border: 0; background: #2563eb; color: #fff; font-weight: 600; }
</style>
</head>
<body>
<form id="paste" method="get" action="/auth">
<h1>{{.Title}}</h1>
<p>{{.Hint}}</p>
<label for="token">{{.Label}}</label>
<input id="token" name="token" placeholder="https://…/auth?token=…" autocomplete="off" autocapitalize="off" autocorrect="off" spellcheck="false" required>
<button type="submit">{{.Button}}</button>
</form>
</body>
</html>
`))

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func (s *server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var dev *device
		if c, err := r.Cookie(cookieName); err == nil {
			dev = s.devices.lookup(c.Value)
		}
		if dev == nil {
			// 画面そのもの（/）を開いたときは、ログイン用の URL を貼ってログインできるページを出す。
			// ホーム画面に追加した palmterm は、ブラウザとログインの記録が分かれていることがあり、
			// アドレスバーも無いので、ログイン用の URL を開く代わりにここへ貼る。
			if r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/index.html") {
				notLoggedInPage(w, r)
				return
			}
			http.Error(w, tr("ログインしていません。PC で palmterm url を実行し、出たログイン用の URL（QR コード）を開いてください", "Not logged in. Run palmterm url on the PC and open the login URL (or scan the QR code) it prints"), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), deviceKey{}, dev.ID)))
	})
}

// requireAuth を通ったリクエストの端末の ID。
func deviceID(r *http.Request) string {
	id, _ := r.Context().Value(deviceKey{}).(string)
	return id
}

type deviceInfo struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Label    string    `json:"label,omitempty"`
	Created  time.Time `json:"created"`
	LastSeen time.Time `json:"lastSeen"`
	Current  bool      `json:"current"` // 問い合わせた端末自身
}

// ログインしている端末の一覧（新しく使った順）。
func (s *server) handleDevices(w http.ResponseWriter, r *http.Request) {
	me := deviceID(r)
	list := []deviceInfo{}
	for _, d := range s.devices.list() {
		list = append(list, deviceInfo{ID: d.ID, Name: d.Name, Label: d.Label, Created: d.Created, LastSeen: d.LastSeen, Current: d.ID == me})
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]any{"devices": list})
}

// 端末のログインを取り消す。{"id": …} を省くと、問い合わせた端末自身（ログアウト）。
// 取り消した端末の端末の接続も切る。
func (s *server) handleRevokeDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	id := req.ID
	if id == "" {
		id = deviceID(r)
	}
	ok, err := s.devices.remove(id)
	if err != nil {
		log.Printf(tr("ログインした端末の記録を書けませんでした: %v", "Could not save the logged-in devices: %v"), err)
	}
	if !ok {
		http.Error(w, tr("その端末はログインしていません", "That device is not logged in"), http.StatusNotFound)
		return
	}
	s.clients.closeDevice(id)
	if id == deviceID(r) {
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode})
	}
	w.WriteHeader(http.StatusNoContent)
}

// 端末に名前を付ける（{"id": …, "name": …}。name が空なら付けた名前を消す）。
func (s *server) handleRenameDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ok, err := s.devices.rename(req.ID, req.Name)
	if err != nil {
		log.Printf(tr("ログインした端末の記録を書けませんでした: %v", "Could not save the logged-in devices: %v"), err)
	}
	if !ok {
		http.Error(w, tr("その端末はログインしていません", "That device is not logged in"), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
