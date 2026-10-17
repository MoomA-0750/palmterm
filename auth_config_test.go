package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// 使い捨てのトークンを出し、ログインのページのボタンを押した（POST /auth）ことにして、ログインさせる。
func login(t *testing.T, s *server, userAgent string) *http.Cookie {
	t.Helper()
	return loginNamed(t, s, userAgent, "")
}

func loginNamed(t *testing.T, s *server, userAgent, name string) *http.Cookie {
	t.Helper()
	w := postLogin(s, s.logins.issue(), userAgent, name)
	c := w.Result().Cookies()
	if w.Code != http.StatusSeeOther || len(c) != 1 {
		t.Fatalf("ログインできません: code=%d cookies=%v", w.Code, c)
	}
	return c[0]
}

// ログインのページの「ログイン」ボタン（フォームの POST /auth）。
func postLogin(s *server, token, userAgent, name string) *httptest.ResponseRecorder {
	form := url.Values{"token": {token}, "name": {name}}
	r := httptest.NewRequest("POST", "/auth", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("User-Agent", userAgent)
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	return w
}

// ログイン用の URL（GET /auth?token=…）を開く。
func getLoginPage(s *server, token, userAgent string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "/auth?token="+url.QueryEscape(token), nil)
	r.Header.Set("User-Agent", userAgent)
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	return w
}

// Cookie を付けて、ログインが要る所を開く。
func authed(s *server, method, target string, body string, c *http.Cookie) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	if c != nil {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	return w
}

func TestAuthSetsCookieOnlyForTheRightToken(t *testing.T) {
	s := newTestServer(t)

	if w := postLogin(s, "wrong", "", ""); w.Code != http.StatusUnauthorized || len(w.Result().Cookies()) != 0 {
		t.Fatalf("違うトークン: code=%d cookies=%v", w.Code, w.Result().Cookies())
	}

	w := postLogin(s, s.logins.issue(), "", "")
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
		t.Fatalf("正しいトークン: code=%d location=%q", w.Code, w.Header().Get("Location"))
	}
	c := w.Result().Cookies()
	if len(c) != 1 || c[0].Name != cookieName || !c[0].HttpOnly || c[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("Cookie が違います: %+v", c)
	}
	// Cookie にはトークンそのものではなく、端末ごとのランダムな値を入れる（Cookie が漏れてもトークンは分からない）。
	if c[0].Value == testToken || strings.Contains(c[0].Value, testToken) || len(c[0].Value) < 32 {
		t.Fatalf("Cookie の値: %q", c[0].Value)
	}
	if other := login(t, s, ""); other.Value == c[0].Value {
		t.Fatal("端末ごとに違う値になっていません")
	}
}

// 固定のトークン（token ファイルの中身。端末の記録の鍵）では、ページも開けず、ログインもできない。
func TestFixedTokenCannotLogIn(t *testing.T) {
	s := newTestServer(t)
	if w := getLoginPage(s, testToken, ""); w.Code != http.StatusUnauthorized || strings.Contains(w.Body.String(), "<form") {
		t.Fatalf("ページ: code=%d %s", w.Code, w.Body)
	}
	if w := postLogin(s, testToken, "", ""); w.Code != http.StatusUnauthorized || len(w.Result().Cookies()) != 0 {
		t.Fatalf("ログイン: code=%d", w.Code)
	}
	if w := getLoginPage(s, "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("空のトークン: code=%d", w.Code)
	}
}

// ログイン用の URL を開くと、名前の欄（自動の名前を薄く出す）と「ログイン」ボタンのページが設定の言語で出る。
// 開いただけではトークンを使わないので、何度開いても同じページが出て、そのあとボタンでログインできる。
func TestLoginPageDoesNotUseTheToken(t *testing.T) {
	s := newTestServer(t)
	const iphone = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1"
	tok := s.logins.issue()
	for range 2 {
		w := getLoginPage(s, tok, iphone)
		body := w.Body.String()
		if w.Code != http.StatusOK || !strings.Contains(body, `<html lang="en">`) || !strings.Contains(body, `placeholder="iPhone · Safari"`) ||
			!strings.Contains(body, `<button type="submit">Log in</button>`) || !strings.Contains(body, `method="post" action="/auth"`) || !strings.Contains(body, `value="`+tok+`"`) {
			t.Fatalf("ページ: code=%d\n%s", w.Code, body)
		}
		if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Referrer-Policy") != "no-referrer" || len(w.Result().Cookies()) != 0 {
			t.Fatalf("見出し: %v", w.Header())
		}
	}

	writeConfig(t, "language = \"ja\"\n")
	body := getLoginPage(s, tok, iphone).Body.String()
	if !strings.Contains(body, `<html lang="ja">`) || !strings.Contains(body, `<button type="submit">ログイン</button>`) || !strings.Contains(body, "この端末の名前") {
		t.Fatalf("日本語のページ:\n%s", body)
	}

	if w := postLogin(s, tok, iphone, ""); w.Code != http.StatusSeeOther {
		t.Fatalf("開いたあとのログイン: code=%d %s", w.Code, w.Body)
	}
}

// 名前を入れてログインすると、その名前で一覧に出る（前後の空白と制御文字は除き、40 文字まで）。空なら自動の名前。
// 一度ログインした URL は、開いても押しても使えない。
func TestLoginWithNameUsesTheTokenOnce(t *testing.T) {
	s := newTestServer(t)
	tok := s.logins.issue()
	w := postLogin(s, tok, "iPhone", "  仕事用\u001b[31m の iPhone ")
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
		t.Fatalf("ログイン: code=%d", w.Code)
	}
	phone := w.Result().Cookies()[0]
	if d := listDevices(t, s, phone); len(d) != 1 || d[0].Label != "仕事用[31m の iPhone" || d[0].Name != "iPhone" {
		t.Fatalf("一覧: %+v", d)
	}

	if w := getLoginPage(s, tok, "Linux"); w.Code != http.StatusUnauthorized || strings.Contains(w.Body.String(), "<form") {
		t.Fatalf("使ったあとのページ: code=%d", w.Code)
	}
	if w := postLogin(s, tok, "Linux", ""); w.Code != http.StatusUnauthorized || len(w.Result().Cookies()) != 0 {
		t.Fatalf("使ったあとのログイン: code=%d", w.Code)
	}

	pc := loginNamed(t, s, "Linux", "")
	loginNamed(t, s, "Android", strings.Repeat("あ", maxDeviceLabel+5))
	if len(listDevices(t, s, pc)) != 3 {
		t.Fatal("一覧の台数")
	}
	for _, d := range listDevices(t, s, pc) {
		switch d.Name {
		case "Linux":
			if d.Label != "" {
				t.Errorf("空の名前: %+v", d)
			}
		case "Android":
			if d.Label != strings.Repeat("あ", maxDeviceLabel) {
				t.Errorf("長い名前: %q", d.Label)
			}
		}
	}
}

// 出してから5分たったトークンでは、ページも開けず、ログインもできない。
func TestLoginTokenExpiresAfterFiveMinutes(t *testing.T) {
	s := newTestServer(t)
	now := time.Now()
	s.logins.now = func() time.Time { return now }
	old, fresh := s.logins.issue(), ""
	now = now.Add(loginTokenLifetime - time.Second)
	fresh = s.logins.issue()
	if w := getLoginPage(s, old, ""); w.Code != http.StatusOK {
		t.Fatalf("5分前まで: code=%d", w.Code)
	}
	now = now.Add(2 * time.Second)
	if w := getLoginPage(s, old, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("5分たったページ: code=%d", w.Code)
	}
	if w := postLogin(s, old, "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("5分たったログイン: code=%d", w.Code)
	}
	if w := postLogin(s, fresh, "", ""); w.Code != http.StatusSeeOther {
		t.Fatalf("あとで出したトークン: code=%d", w.Code)
	}
}

// 再起動すると（トークンはメモリにだけある）、まだ使っていないトークンはすべて無効になる。
func TestRestartInvalidatesUnusedLoginTokens(t *testing.T) {
	before := newTestServer(t)
	tok := before.logins.issue()
	after := newTestServer(t)
	after.devices = before.devices
	if w := getLoginPage(after, tok, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("ページ: code=%d", w.Code)
	}
	if w := postLogin(after, tok, "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("ログイン: code=%d", w.Code)
	}
}

// 更新前からログインしていた端末（token ファイルの中身を鍵にした記録）は、ログインし直さずにそのまま通る。
func TestDevicesFromBeforeTheUpdateStillWork(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	os.MkdirAll(filepath.Join(dir, "palmterm"), 0o700)
	os.WriteFile(filepath.Join(dir, "palmterm", "token"), []byte("oldtoken\n"), 0o600)
	path := filepath.Join(dir, "palmterm", "devices.json")
	// 更新前の palmterm が、token ファイルのトークンでログインさせたときの記録
	secret, err := newDeviceStore(path, "oldtoken").login("iPhone", "")
	if err != nil {
		t.Fatal(err)
	}

	s := newTestServer(t)
	key, err := loadOrCreateToken()
	if err != nil || key != "oldtoken" {
		t.Fatalf("key=%q err=%v", key, err)
	}
	s.devices = newDeviceStore(path, key)
	if w := authed(s, "GET", "/api/devices", "", &http.Cookie{Name: cookieName, Value: secret}); w.Code != http.StatusOK {
		t.Fatalf("前からの端末: code=%d", w.Code)
	}
}

func TestRequireAuth(t *testing.T) {
	s := newTestServer(t)
	h := s.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))

	// トークンそのものや、前の形の Cookie（トークンから計算した値）を入れても通さない（通すのはログインで渡した値だけ）。
	mac := hmac.New(sha256.New, []byte(testToken))
	mac.Write([]byte("palmterm cookie v1"))
	legacy := hex.EncodeToString(mac.Sum(nil))
	for name, cookie := range map[string]string{"Cookie なし": "", "違う値": "nope", "トークンそのもの": testToken, "前の形の Cookie": legacy} {
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
	r.AddCookie(login(t, s, ""))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || w.Body.String() != "ok" {
		t.Fatalf("正しい Cookie: code=%d body=%q", w.Code, w.Body)
	}
}

type devicesResponse struct {
	Devices []deviceInfo `json:"devices"`
}

func listDevices(t *testing.T, s *server, c *http.Cookie) []deviceInfo {
	t.Helper()
	w := authed(s, "GET", "/api/devices", "", c)
	if w.Code != http.StatusOK {
		t.Fatalf("一覧: code=%d %s", w.Code, w.Body)
	}
	var res devicesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	return res.Devices
}

// ログインした端末を一覧にし、自分（ログアウト）もほかの端末も取り消せる。取り消した Cookie は通らない。
func TestDevicesListLogoutAndRevoke(t *testing.T) {
	s := newTestServer(t)
	phone := login(t, s, "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1")
	pc := login(t, s, "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36")
	tablet := login(t, s, "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Mobile Safari/537.36 EdgA/130.0")

	list := listDevices(t, s, pc)
	if len(list) != 3 {
		t.Fatalf("一覧: %+v", list)
	}
	names := map[string]bool{}
	var phoneID string
	for _, d := range list {
		names[d.Name] = true
		if d.Current != (d.Name == "Linux · Chrome") {
			t.Errorf("自分の印: %+v", d)
		}
		if d.Name == "iPhone · Safari" {
			phoneID = d.ID
		}
	}
	if !names["iPhone · Safari"] || !names["Linux · Chrome"] || !names["Android · Edge"] {
		t.Fatalf("名前: %v", names)
	}

	// PC からスマホを取り消す。
	if w := authed(s, "POST", "/api/devices/revoke", `{"id":"`+phoneID+`"}`, pc); w.Code != http.StatusNoContent || len(w.Result().Cookies()) != 0 {
		t.Fatalf("取り消し: code=%d %s cookies=%v", w.Code, w.Body, w.Result().Cookies())
	}
	if w := authed(s, "GET", "/api/devices", "", phone); w.Code != http.StatusUnauthorized {
		t.Fatalf("取り消したスマホが通ります: code=%d", w.Code)
	}
	if w := authed(s, "POST", "/api/devices/revoke", `{"id":"`+phoneID+`"}`, pc); w.Code != http.StatusNotFound {
		t.Fatalf("もう無い端末の取り消し: code=%d", w.Code)
	}

	// タブレットが自分でログアウトする。Cookie も消す。
	w := authed(s, "POST", "/api/devices/revoke", "", tablet)
	if w.Code != http.StatusNoContent {
		t.Fatalf("ログアウト: code=%d %s", w.Code, w.Body)
	}
	if c := w.Result().Cookies(); len(c) != 1 || c[0].Name != cookieName || c[0].MaxAge >= 0 {
		t.Fatalf("Cookie を消していません: %+v", c)
	}
	if w := authed(s, "GET", "/", "", tablet); w.Code != http.StatusUnauthorized {
		t.Fatalf("ログアウトした端末が通ります: code=%d", w.Code)
	}

	if list := listDevices(t, s, pc); len(list) != 1 || !list[0].Current {
		t.Fatalf("残った一覧: %+v", list)
	}
}

// ログインした端末の記録は再起動しても残る。トークンを変えたら全部捨てる。期限の切れたものも通さない。
func TestDevicesSurviveRestartButNotTokenChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.json")
	d := newDeviceStore(path, "tok1")
	secret, err := d.login("", "")
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("記録のファイル: %v %v", info, err)
	}
	if b, _ := os.ReadFile(path); strings.Contains(string(b), secret) {
		t.Fatal("Cookie の値がそのままファイルにあります")
	}
	if newDeviceStore(path, "tok1").lookup(secret) == nil {
		t.Fatal("再起動したらログインが消えました")
	}
	if newDeviceStore(path, "tok2").lookup(secret) != nil {
		t.Fatal("トークンを変えてもログインが残っています")
	}

	later := newDeviceStore(path, "tok1")
	later.now = func() time.Time { return time.Now().Add(loginLifetime + time.Hour) }
	if later.lookup(secret) != nil {
		t.Fatal("期限の切れたログインが通ります")
	}
}

type configResponse struct {
	Language string       `json:"language"`
	Keys     []keyConfig  `json:"keys"`
	Theme    themeConfig  `json:"theme"`
	Themes   customThemes `json:"themes"`
	Path     string       `json:"path"`
	Version  string       `json:"version"`
	Error    string       `json:"error"`
	Newer    int          `json:"newerConfig"`
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

func TestConfigIncludesVersion(t *testing.T) {
	s := newTestServer(t)
	defer func(v string) { version = v }(version)
	version = "v1.2.3-4-gabcdef0-dirty"
	if c := getConfig(t, s); c.Version != version {
		t.Fatalf("version=%q", c.Version)
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

// 設定ファイルの版。書いていなければ 1 とみなし、今までの設定ファイルはそのまま読める。今の版は 2。
func TestConfigVersion(t *testing.T) {
	s := newTestServer(t)
	for name, body := range map[string]string{
		"版なし":         "language = \"ja\"\n[[keys]]\nkey = \"esc\"\n",
		"version = 1": "version = 1\nlanguage = \"ja\"\n[[keys]]\nkey = \"esc\"\n",
		"version = 2": "version = 2\nlanguage = \"ja\"\n[[keys]]\nkey = \"esc\"\n",
	} {
		writeConfig(t, body)
		c := getConfig(t, s)
		if c.Error != "" || c.Newer != 0 || c.Language != "ja" || len(c.Keys) != 1 || c.Keys[0].Key != "esc" {
			t.Errorf("%s: %+v", name, c)
		}
		if c.Version != version {
			t.Errorf("%s: 応答の version は palmterm の版のままのはずです: %q", name, c.Version)
		}
	}
	for _, body := range []string{"version = 0\n", "version = -1\n", "version = \"1\"\n"} {
		writeConfig(t, body)
		if c := getConfig(t, s); c.Error == "" || c.Newer != 0 {
			t.Errorf("%q: %+v", body, c)
		}
	}
}

// 知らない新しい版なら、読める項目だけ使い、画面に新しい版であることを知らせる（書き間違いの知らせは出さない）。
func TestConfigNewerVersionUsesWhatItCanRead(t *testing.T) {
	s := newTestServer(t)
	writeConfig(t, `
version = 3
language = "ja"
theme = "dark"

[[keys]]
key = "ctrl+c"
color = "red"

[[keys]]
mod = "alt"

[layout]
rows = 2
`)
	c := getConfig(t, s)
	if c.Newer != 3 || c.Error != "" {
		t.Fatalf("新しい版の知らせ: %+v", c)
	}
	if c.Language != "ja" || len(c.Keys) != 2 || c.Keys[0].Key != "ctrl+c" || c.Keys[1].Mod != "alt" {
		t.Fatalf("読める項目が使われていません: %+v", c)
	}

	// 書き方が変わって読めない項目は飛ばし、残りは使う。
	writeConfig(t, "version = 4\nlanguage = \"ja\"\nkeys = \"esc tab\"\n")
	if c := getConfig(t, s); c.Newer != 4 || c.Error != "" || c.Language != "ja" || c.Keys != nil {
		t.Fatalf("%+v", c)
	}
	writeConfig(t, "version = 3\nlanguage = \"fr\"\n")
	if c := getConfig(t, s); c.Newer != 3 || c.Error != "" || c.Language != "en" {
		t.Fatalf("%+v", c)
	}

	// TOML として読めなければ、今までどおり書き間違いとして知らせる。
	writeConfig(t, "version = 3\nlanguage = \n")
	if c := getConfig(t, s); c.Error == "" || c.Newer != 0 {
		t.Fatalf("%+v", c)
	}
}

// 見本の設定ファイルは今の版（version = 2）で、そのまま読める。
func TestConfigExampleReadsAsIs(t *testing.T) {
	body, err := os.ReadFile("config.example.toml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), fmt.Sprintf("\nversion = %d\n", configVersion)) {
		t.Fatalf("config.example.toml に version = %d がありません", configVersion)
	}
	s := newTestServer(t)
	writeConfig(t, string(body))
	c := getConfig(t, s)
	if c.Error != "" || c.Newer != 0 || c.Language != "en" || len(c.Keys) == 0 || c.Theme.Name != "default" {
		t.Fatalf("%+v", c)
	}
}

// [theme] には name だけを書き、[themes.名前] の自分のテーマは書いたとおり画面に渡す
// （色・項目・base の確かめと base への戻しは画面側）。[theme] に name 以外を書くと書き間違いとして知らせる。
func TestConfigTheme(t *testing.T) {
	s := newTestServer(t)
	writeConfig(t, `
[theme]
name = "my-dark"

[themes.my-dark]
base = "solarized-dark"
accent = "#ff5f87"

[themes.mine]
red = "#12345"
backgroud = "#000000"
base = "dracula"
`)
	c := getConfig(t, s)
	want := customThemes{
		"my-dark": {"base": "solarized-dark", "accent": "#ff5f87"},
		"mine":    {"red": "#12345", "backgroud": "#000000", "base": "dracula"},
	}
	if c.Error != "" || c.Theme != (themeConfig{Name: "my-dark"}) || !reflect.DeepEqual(c.Themes, want) {
		t.Fatalf("%+v", c)
	}

	// [theme] に色を書くと書き間違い。既定の設定を使う（色は [themes.名前] に書くよう知らせる）
	for _, bad := range []string{
		"[theme]\nname = \"light\"\naccent = \"#ff5f87\"\n",
		"[theme]\nname = \"light\"\nbackgroud = \"#000000\"\n",
	} {
		writeConfig(t, bad)
		c := getConfig(t, s)
		if !strings.Contains(c.Error, "[themes.NAME]") || !strings.Contains(c.Error, "theme.") || c.Theme != (themeConfig{}) || c.Themes != nil {
			t.Fatalf("%q: %+v", bad, c)
		}
	}
	writeConfig(t, "language = \"en\"\n[theme]\nname = \"light\"\nbackgroud = \"#000000\"\n")
	if c := getConfig(t, s); !strings.Contains(c.Error, "theme.backgroud") || c.Language != "en" {
		t.Fatalf("%+v", c)
	}

	// 書いていなければ空（画面側で default になる）。
	writeConfig(t, "language = \"ja\"\n")
	if c := getConfig(t, s); c.Error != "" || c.Theme != (themeConfig{}) || c.Themes != nil {
		t.Fatalf("%+v", c)
	}

	// 新しい版の設定ファイルでも [theme] の name と [themes] は読む（[theme] の知らない項目は飛ばす）。
	writeConfig(t, "version = 3\n[theme]\nname = \"mine\"\naccent = \"#0969da\"\n[themes.mine]\nglow = true\n")
	if c := getConfig(t, s); c.Newer != 3 || c.Error != "" || c.Theme != (themeConfig{Name: "mine"}) || !reflect.DeepEqual(c.Themes, customThemes{"mine": {"glow": true}}) {
		t.Fatalf("%+v", c)
	}
}

// 手で作ったなどでほかの人が読める権限になっているトークンのファイルは、読むときに 0600 に直す。
func TestLoadTokenFixesLoosePermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "palmterm", "token")
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, []byte("handmade\n"), 0o644)
	os.Chmod(path, 0o644)
	tok, err := loadOrCreateToken()
	if err != nil || tok != "handmade" {
		t.Fatalf("tok=%q err=%v", tok, err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Fatalf("権限: %v", info.Mode().Perm())
	}
}

// ログインを取り消すと、その端末の端末の接続も切る。ほかの端末の接続は切らない。
func TestRevokeClosesThatDevicesTerminal(t *testing.T) {
	isolateTmux(t, "t1")
	s := newTestServer(t)
	phone, pc := login(t, s, "iPhone"), login(t, s, "Linux")
	srv := httptest.NewServer(s.routes())
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dial := func(c *http.Cookie) *websocket.Conn {
		h := http.Header{}
		h.Set("Cookie", c.Name+"="+c.Value)
		conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws?cols=80&rows=20", &websocket.DialOptions{HTTPHeader: h})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.CloseNow() })
		return conn
	}
	phoneConn, pcConn := dial(phone), dial(pc)
	if _, _, err := phoneConn.Read(ctx); err != nil { // つながって、端末の出力が来るまで待つ
		t.Fatal(err)
	}

	var phoneID string
	for _, d := range listDevices(t, s, pc) {
		if d.Name == "iPhone" {
			phoneID = d.ID
		}
	}
	if w := authed(s, "POST", "/api/devices/revoke", `{"id":"`+phoneID+`"}`, pc); w.Code != http.StatusNoContent {
		t.Fatalf("取り消し: code=%d %s", w.Code, w.Body)
	}
	for {
		if _, _, err := phoneConn.Read(ctx); err != nil {
			if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
				t.Fatalf("切れ方: %v", err)
			}
			break
		}
	}
	// PC の接続は生きている（入力が届いて、出力が返る）。
	if err := pcConn.Write(ctx, websocket.MessageBinary, []byte("echo still-here\r")); err != nil {
		t.Fatal(err)
	}
	for {
		_, data, err := pcConn.Read(ctx)
		if err != nil {
			t.Fatalf("PC の接続が切れました: %v", err)
		}
		if strings.Contains(string(data), "still-here") {
			return
		}
	}
}

// 端末に名前を付けられる。一覧に出て、再起動しても残る。前後の空白と制御文字は除き、長すぎれば切る。空にすると消える。
func TestRenameDevice(t *testing.T) {
	s := newTestServer(t)
	phone := login(t, s, "iPhone")
	id := listDevices(t, s, phone)[0].ID

	if w := authed(s, "POST", "/api/devices/rename", `{"id":"`+id+`","name":"  仕事用\u001b[31m の iPhone "}`, phone); w.Code != http.StatusNoContent {
		t.Fatalf("名前を付ける: code=%d %s", w.Code, w.Body)
	}
	if d := listDevices(t, s, phone)[0]; d.Label != "仕事用[31m の iPhone" || d.Name != "iPhone" {
		t.Fatalf("一覧: %+v", d)
	}
	restarted := newDeviceStore(s.devices.path, testToken)
	if l := restarted.list(); len(l) != 1 || l[0].Label != "仕事用[31m の iPhone" {
		t.Fatalf("再起動したら名前が消えました: %+v", l)
	}

	long := strings.Repeat("あ", maxDeviceLabel+5)
	authed(s, "POST", "/api/devices/rename", `{"id":"`+id+`","name":"`+long+`"}`, phone)
	if d := listDevices(t, s, phone)[0]; d.Label != strings.Repeat("あ", maxDeviceLabel) {
		t.Fatalf("長い名前: %q", d.Label)
	}

	authed(s, "POST", "/api/devices/rename", `{"id":"`+id+`","name":"  "}`, phone)
	if d := listDevices(t, s, phone)[0]; d.Label != "" {
		t.Fatalf("空にしても消えません: %+v", d)
	}
	if w := authed(s, "POST", "/api/devices/rename", `{"id":"nope","name":"x"}`, phone); w.Code != http.StatusNotFound {
		t.Fatalf("知らない端末: code=%d", w.Code)
	}
}
