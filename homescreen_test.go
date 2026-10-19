package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// ホーム画面に追加できるよう、manifest（アイコンと display: standalone）を返し、画面がそれを読む。
// manifest とアイコンは Cookie なしでも返す（ホーム画面のアプリはブラウザとログインの記録が分かれることがあるため）。
func TestHomeScreenManifest(t *testing.T) {
	s := newTestServer(t)

	w := authed(s, "GET", "/manifest.webmanifest", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("manifest: code=%d", w.Code)
	}
	var m struct {
		Name     string `json:"name"`
		StartURL string `json:"start_url"`
		Display  string `json:"display"`
		Icons    []struct {
			Src   string `json:"src"`
			Sizes string `json:"sizes"`
			Type  string `json:"type"`
		} `json:"icons"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("manifest が JSON でない: %v\n%s", err, w.Body)
	}
	if m.Name != "palmterm" || m.Display != "standalone" || m.StartURL != "/" {
		t.Fatalf("manifest: %+v", m)
	}
	sizes := map[string]bool{}
	for _, ic := range m.Icons {
		sizes[ic.Sizes] = true
		iw := authed(s, "GET", ic.Src, "", nil)
		if iw.Code != http.StatusOK || iw.Header().Get("Content-Type") != "image/png" || !strings.HasPrefix(iw.Body.String(), "\x89PNG") {
			t.Errorf("アイコン %s: code=%d type=%q", ic.Src, iw.Code, iw.Header().Get("Content-Type"))
		}
	}
	if !sizes["192x192"] || !sizes["512x512"] {
		t.Errorf("192 と 512 のアイコンが要る: %v", sizes)
	}

	// 画面（ログインしているとき）と、ログインしていないときのページの両方が manifest と iOS のアイコンを読む。
	page := authed(s, "GET", "/", "", login(t, s, ""))
	paste := authed(s, "GET", "/", "", nil)
	if page.Code != http.StatusOK || paste.Code != http.StatusUnauthorized {
		t.Fatalf("/: ログイン済み code=%d、未ログイン code=%d", page.Code, paste.Code)
	}
	for name, body := range map[string]string{"画面": page.Body.String(), "未ログインのページ": paste.Body.String()} {
		for _, want := range []string{`rel="manifest" href="/manifest.webmanifest"`, `rel="apple-touch-icon" href="/icons/icon-180.png"`, `name="apple-mobile-web-app-capable" content="yes"`} {
			if !strings.Contains(body, want) {
				t.Errorf("%s に %s が無い", name, want)
			}
		}
	}
	if w := authed(s, "GET", "/icons/icon-180.png", "", nil); w.Code != http.StatusOK {
		t.Errorf("iOS のアイコン: code=%d", w.Code)
	}

	// manifest とアイコン以外は、今までどおりログインが要る。
	for _, p := range []string{"/index.html", "/api/config", "/assets/"} {
		if w := authed(s, "GET", p, "", nil); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: Cookie なしで code=%d", p, w.Code)
		}
	}
}

// ホーム画面のアプリにはアドレスバーが無いので、ログインしていないときのページにログイン用の URL を貼って
// 「次へ」を押すと（GET /auth?token=貼った URL）、名前の欄と「ログイン」ボタンのページに進める。
func TestPasteLoginURL(t *testing.T) {
	s := newTestServer(t)

	paste := authed(s, "GET", "/", "", nil).Body.String()
	if !strings.Contains(paste, `<form id="paste" method="get" action="/auth">`) || !strings.Contains(paste, `name="token"`) {
		t.Fatalf("貼り付けの欄が無い:\n%s", paste)
	}

	tok := s.logins.issue()
	for name, pasted := range map[string]string{
		"URL を丸ごと": "https://box.example.ts.net/auth?token=" + tok,
		"前後に空白":    "  http://192.0.2.1:7681/auth?token=" + tok + "\n",
		"トークンだけ":   tok,
	} {
		w := getLoginPage(s, pasted, "")
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `name="token" value="`+tok+`"`) {
			t.Errorf("%s: code=%d\n%s", name, w.Code, w.Body)
		}
	}
	for name, pasted := range map[string]string{
		"違うトークン":    "https://box.example.ts.net/auth?token=nope",
		"token が無い": "https://box.example.ts.net/auth?x=" + tok,
		"空":         "",
	} {
		if w := getLoginPage(s, pasted, ""); w.Code != http.StatusUnauthorized || strings.Contains(w.Body.String(), "<form") {
			t.Errorf("%s: code=%d", name, w.Code)
		}
	}

	// 貼って進んだページのボタンでログインでき、トークンは使い切る。
	if w := postLogin(s, tok, "", ""); w.Code != http.StatusSeeOther {
		t.Fatalf("ログイン: code=%d", w.Code)
	}
	if w := getLoginPage(s, "https://box.example.ts.net/auth?token="+url.QueryEscape(tok), ""); w.Code != http.StatusUnauthorized {
		t.Errorf("使ったトークン: code=%d", w.Code)
	}
}
