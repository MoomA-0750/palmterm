package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// 正しいトークンで /auth を開いた端末に Cookie を入れて、ログインさせる。
func login(t *testing.T, s *server, userAgent string) *http.Cookie {
	t.Helper()
	r := httptest.NewRequest("GET", "/auth?token="+testToken, nil)
	r.Header.Set("User-Agent", userAgent)
	w := do(s.handleAuth, r)
	c := w.Result().Cookies()
	if w.Code != http.StatusSeeOther || len(c) != 1 {
		t.Fatalf("ログインできません: code=%d cookies=%v", w.Code, c)
	}
	return c[0]
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

	w := do(s.handleAuth, httptest.NewRequest("GET", "/auth?token=wrong", nil))
	if w.Code != http.StatusUnauthorized || len(w.Result().Cookies()) != 0 {
		t.Fatalf("違うトークン: code=%d cookies=%v", w.Code, w.Result().Cookies())
	}

	w = do(s.handleAuth, httptest.NewRequest("GET", "/auth?token="+testToken, nil))
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

func TestRequireAuth(t *testing.T) {
	s := newTestServer(t)
	h := s.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) }))

	// トークンそのものを Cookie に入れても通さない（通すのはログインで渡した値だけ）。
	for name, cookie := range map[string]string{"Cookie なし": "", "違う値": "nope", "トークンそのもの": testToken} {
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
	if s.validToken("") {
		t.Fatal("空のトークンを通しました")
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
	secret, err := d.login("")
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

func TestValidTokenAcceptsOnlyTheToken(t *testing.T) {
	s := newTestServer(t)
	for tok, want := range map[string]bool{testToken: true, "": false, "testtoke": false, testToken + "x": false, strings.ToUpper(testToken): false} {
		if s.validToken(tok) != want {
			t.Errorf("%q", tok)
		}
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
