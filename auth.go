package main

import (
	"crypto/subtle"
	"net/http"
)

const cookieName = "palmterm_token"

// /auth?token=… で正しいトークンを受け取ったら Cookie に入れ、以降はそれで通す。
func (s *server) handleAuth(w http.ResponseWriter, r *http.Request) {
	if !s.validToken(r.URL.Query().Get("token")) {
		http.Error(w, tr("トークンが違います", "Wrong token"), http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    s.token,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil || !s.validToken(c.Value) {
			http.Error(w, tr("サーバーの起動時に表示された /auth?token=… の URL で開いてください", "Open the /auth?token=… URL printed when the server started"), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) validToken(t string) bool {
	return t != "" && subtle.ConstantTimeCompare([]byte(t), []byte(s.token)) == 1
}
