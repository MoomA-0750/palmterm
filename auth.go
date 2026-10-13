package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

const cookieName = "palmterm_token"

type deviceKey struct{}

// /auth?token=… で正しいトークンを受け取ったら、この端末を記録して Cookie を入れ、以降はそれで通す。
// Cookie にはトークンではなく端末ごとのランダムな値を入れる。Cookie はポートで分けられず、同じホスト名の
// 別のポートのサービスにも送られるので、そこから漏れてもトークン（新しい端末でのログイン）は渡らず、
// 漏れた端末の分だけ取り消せる。
func (s *server) handleAuth(w http.ResponseWriter, r *http.Request) {
	if !s.validToken(r.URL.Query().Get("token")) {
		http.Error(w, tr("トークンが違います", "Wrong token"), http.StatusUnauthorized)
		return
	}
	secret, err := s.devices.login(r.UserAgent())
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
			http.Error(w, tr("サーバーの起動時に表示された /auth?token=… の URL で開いてください（palmterm url でも出せます）", "Open the /auth?token=… URL printed when the server started (or run: palmterm url)"), http.StatusUnauthorized)
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

// トークンが合っているか。長さの違いも時間から分からないよう、ハッシュにして同じ長さにしてから比べる。
func (s *server) validToken(t string) bool {
	got, want := sha256.Sum256([]byte(t)), sha256.Sum256([]byte(s.token))
	return t != "" && subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

type deviceInfo struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Created  time.Time `json:"created"`
	LastSeen time.Time `json:"lastSeen"`
	Current  bool      `json:"current"` // 問い合わせた端末自身
}

// ログインしている端末の一覧（新しく使った順）。
func (s *server) handleDevices(w http.ResponseWriter, r *http.Request) {
	me := deviceID(r)
	list := []deviceInfo{}
	for _, d := range s.devices.list() {
		list = append(list, deviceInfo{ID: d.ID, Name: d.Name, Created: d.Created, LastSeen: d.LastSeen, Current: d.ID == me})
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
