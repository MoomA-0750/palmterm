package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
)

const cookieName = "palmterm_token"

// /auth?token=… で正しいトークンを受け取ったら Cookie を入れ、以降はそれで通す。
// Cookie にはトークンそのものではなく、トークンから計算した値を入れる。Cookie はポートで分けられず、
// 同じホスト名の別のポートのサービスにも送られるので、そこから漏れてもトークン（新しい端末でのログイン）は渡らない。
func (s *server) handleAuth(w http.ResponseWriter, r *http.Request) {
	if !s.validToken(r.URL.Query().Get("token")) {
		http.Error(w, tr("トークンが違います", "Wrong token"), http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    s.cookieValue(),
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		// Strict だと、ほかのアプリ（チャットなど）から開いたリンクや、証明書の警告の画面から進んだときに
		// Cookie が送られず、ログイン済みでもログインを求める画面になる。Lax でも、別のサイトからの POST
		// （tmux の操作・アップロード）には付かず、WebSocket は Origin を確かめるので、勝手には操作されない。
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil || subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.cookieValue())) != 1 {
			http.Error(w, tr("サーバーの起動時に表示された /auth?token=… の URL で開いてください", "Open the /auth?token=… URL printed when the server started"), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) validToken(t string) bool {
	return t != "" && subtle.ConstantTimeCompare([]byte(t), []byte(s.token)) == 1
}

// Cookie に入れる値（トークンから計算する。トークンを変えれば全端末の Cookie も無効になる）。
func (s *server) cookieValue() string {
	mac := hmac.New(sha256.New, []byte(s.token))
	mac.Write([]byte("palmterm cookie v1"))
	return hex.EncodeToString(mac.Sum(nil))
}
