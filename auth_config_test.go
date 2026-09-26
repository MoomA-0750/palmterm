package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthSetsCookieOnlyForTheRightToken(t *testing.T) {
	s := newTestServer(t)

	w := do(s.handleAuth, httptest.NewRequest("GET", "/auth?token=wrong", nil))
	if w.Code != http.StatusUnauthorized || len(w.Result().Cookies()) != 0 {
		t.Fatalf("違うトークン: code=%d cookies=%v", w.Code, w.Result().Cookies())
	}

	w = do(s.handleAuth, httptest.NewRequest("GET", "/auth?token="+testToken, nil))
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
		t.Fatalf("正しいトークン: code=%d location=%q", w.Code, w.Header().Get("Location"))
	}
	c := w.Result().Cookies()
	if len(c) != 1 || c[0].Name != cookieName || c[0].Value != testToken || !c[0].HttpOnly || c[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("Cookie が違います: %+v", c)
	}
}

func TestRequireAuth(t *testing.T) {
	s := newTestServer(t)
	h := s.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))

	for name, cookie := range map[string]string{"Cookie なし": "", "違う値": "nope"} {
		r := httptest.NewRequest("GET", "/", nil)
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: code=%d", name, w.Code)
		}
	}

	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: cookieName, Value: testToken})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || w.Body.String() != "ok" {
		t.Fatalf("正しい Cookie: code=%d body=%q", w.Code, w.Body)
	}
	if s.validToken("") {
		t.Fatal("空のトークンを通しました")
	}
}

type configResponse struct {
	Language string      `json:"language"`
	Keys     []keyConfig `json:"keys"`
	Path     string      `json:"path"`
	Error    string      `json:"error"`
}

func getConfig(t *testing.T, s *server) configResponse {
	t.Helper()
	w := do(s.handleConfig, httptest.NewRequest("GET", "/api/config", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d", w.Code)
	}
	var c configResponse
	if err := json.Unmarshal(w.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestConfigDefaultsToEnglishWithoutAFile(t *testing.T) {
	s := newTestServer(t)
	c := getConfig(t, s)
	if c.Language != "en" || c.Keys != nil || c.Error != "" || c.Path != settings.path {
		t.Fatalf("%+v", c)
	}
	if tr("日本語", "English") != "English" {
		t.Fatal("既定は英語のはずです")
	}
}

func TestConfigReadsLanguageAndKeysAndReloads(t *testing.T) {
	s := newTestServer(t)
	writeConfig(t, `
language = "ja"

[[keys]]
key = "ctrl+c"
label = "^C"

[[keys]]
mod = "alt"

[[keys]]
text = "|"
repeat = true
`)
	c := getConfig(t, s)
	if c.Language != "ja" || len(c.Keys) != 3 || c.Error != "" {
		t.Fatalf("%+v", c)
	}
	if k := c.Keys[0]; k.Key != "ctrl+c" || k.Label != "^C" {
		t.Errorf("keys[0] = %+v", k)
	}
	if k := c.Keys[1]; k.Mod != "alt" {
		t.Errorf("keys[1] = %+v", k)
	}
	if k := c.Keys[2]; k.Text != "|" || k.Repeat == nil || !*k.Repeat {
		t.Errorf("keys[2] = %+v", k)
	}
	if tr("日本語", "English") != "日本語" {
		t.Fatal("language = ja なのに日本語になりません")
	}

	// 書き換えたら、次に聞いたときに読み直す。
	writeConfig(t, `language = "en"`)
	if c := getConfig(t, s); c.Language != "en" || c.Keys != nil {
		t.Fatalf("読み直していません: %+v", c)
	}
}

func TestConfigErrorsFallBackToDefaults(t *testing.T) {
	cases := map[string]string{
		"知らない項目":   "langage = \"ja\"\n",
		"知らない言語":   "language = \"fr\"\n",
		"TOML の誤り": "language = \n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			s := newTestServer(t)
			writeConfig(t, body)
			c := getConfig(t, s)
			if c.Error == "" || c.Language != "en" || c.Keys != nil {
				t.Fatalf("%+v", c)
			}
		})
	}
	s := newTestServer(t)
	writeConfig(t, "langage = \"ja\"\n")
	if c := getConfig(t, s); !strings.Contains(c.Error, "langage") {
		t.Fatalf("間違えた項目名が知らせにありません: %q", c.Error)
	}
}
