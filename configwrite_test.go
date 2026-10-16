package main

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func chooseAppearance(t *testing.T, s *server, c *http.Cookie, body string) (int, string) {
	t.Helper()
	w := authed(s, "POST", "/api/config/appearance", body, c)
	return w.Code, w.Body.String()
}

func readConfigFile(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(settings.path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func putFont(t *testing.T, name string) {
	t.Helper()
	if err := os.MkdirAll(fontsDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fontsDir(), name), []byte("\x00\x01\x00\x00 font"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// 画面で選んだテーマとフォントは [theme] の name と [font] の file の行だけを書き換え、
// コメント・ほかの項目・並び順はそのまま残す。別の端末で読み込み直しても同じ設定になる。
func TestAppearanceIsWrittenBackKeepingTheRest(t *testing.T) {
	s := newTestServer(t)
	phone, other := login(t, s, "phone"), login(t, s, "tablet")
	putFont(t, "Mine.ttf")
	writeConfig(t, `# 私の設定
version = 1
language = "ja" # 日本語で

[theme]
# 色の名前
name = "default"   # あとで変える
accent = "#0969da"

# フォント
[font]
family = "Liberation Mono"

[[keys]]
key = "esc"
`)
	code, body := chooseAppearance(t, s, phone, `{"theme":"solarized-light","font":"Mine.ttf"}`)
	if code != http.StatusOK {
		t.Fatalf("code=%d %s", code, body)
	}
	want := `# 私の設定
version = 1
language = "ja" # 日本語で

[theme]
# 色の名前
name = "solarized-light"   # あとで変える
accent = "#0969da"

# フォント
[font]
family = "Liberation Mono"
file = "fonts/Mine.ttf"

[[keys]]
key = "esc"
`
	if got := readConfigFile(t); got != want {
		t.Fatalf("書き戻した中身:\n%s", got)
	}
	// 応答は /api/config と同じ形で、今の設定（すぐ画面に当てる）
	if !strings.Contains(body, `"name":"solarized-light"`) || !strings.Contains(body, `"file":"fonts/Mine.ttf"`) || !strings.Contains(body, `"fontURL":"/api/font?v=`) {
		t.Fatalf("応答: %s", body)
	}
	// 別の端末で読み込み直しても同じ
	w := authed(s, "GET", "/api/config", "", other)
	if !strings.Contains(w.Body.String(), `"name":"solarized-light"`) || !strings.Contains(w.Body.String(), `"file":"fonts/Mine.ttf"`) {
		t.Fatalf("別の端末: %s", w.Body.String())
	}

	// フォントを「既定」にすると file の行だけ消える。テーマだけ選べばテーマだけ変わる
	if code, body := chooseAppearance(t, s, phone, `{"font":""}`); code != http.StatusOK {
		t.Fatalf("code=%d %s", code, body)
	}
	if code, body := chooseAppearance(t, s, phone, `{"theme":"light"}`); code != http.StatusOK {
		t.Fatalf("code=%d %s", code, body)
	}
	want = strings.Replace(strings.Replace(want, "file = \"fonts/Mine.ttf\"\n", "", 1), `"solarized-light"`, `"light"`, 1)
	if got := readConfigFile(t); got != want {
		t.Fatalf("書き戻した中身:\n%s", got)
	}
}

// 設定ファイルが無ければ作り、[theme] や [font] が無ければ足す。
func TestAppearanceCreatesTheFileAndTables(t *testing.T) {
	s := newTestServer(t)
	c := login(t, s, "")
	putFont(t, "A.woff2")
	if code, body := chooseAppearance(t, s, c, `{"theme":"light","font":"A.woff2"}`); code != http.StatusOK {
		t.Fatalf("code=%d %s", code, body)
	}
	if got, want := readConfigFile(t), "version = 1\n\n[theme]\nname = \"light\"\n\n[font]\nfile = \"fonts/A.woff2\"\n"; got != want {
		t.Fatalf("作った中身:\n%q", got)
	}
	if info, _ := os.Stat(settings.path); info.Mode().Perm() != 0o600 {
		t.Fatalf("権限: %v", info.Mode())
	}
	if cfg, err := settings.get(); err != nil || cfg.Theme.Name != "light" || cfg.Font.File != "fonts/A.woff2" {
		t.Fatalf("%+v %v", cfg, err)
	}

	// [theme] はあるが name が無い。最後の行に改行が無い。CRLF で書いてある
	writeConfig(t, "language = \"en\"\r\n[theme]\r\nred = \"#ff0000\"\r\n\r\n# keys\r\n[[keys]]\r\nkey = \"esc\"")
	if code, body := chooseAppearance(t, s, c, `{"theme":"solarized-dark"}`); code != http.StatusOK {
		t.Fatalf("code=%d %s", code, body)
	}
	if got, want := readConfigFile(t), "language = \"en\"\r\n[theme]\r\nred = \"#ff0000\"\r\nname = \"solarized-dark\"\r\n\r\n# keys\r\n[[keys]]\r\nkey = \"esc\""; got != want {
		t.Fatalf("足した中身:\n%q", got)
	}

	// 設定ファイルがリンクなら、リンクのまま先を書き換える
	real := filepath.Join(t.TempDir(), "real.toml")
	if err := os.WriteFile(real, []byte("[theme]\nname = \"light\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Remove(settings.path)
	if err := os.Symlink(real, settings.path); err != nil {
		t.Fatal(err)
	}
	if code, body := chooseAppearance(t, s, c, `{"theme":"default"}`); code != http.StatusOK {
		t.Fatalf("code=%d %s", code, body)
	}
	if info, err := os.Lstat(settings.path); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("リンクが置き換わりました")
	}
	if b, _ := os.ReadFile(real); string(b) != "[theme]\nname = \"default\"\n" {
		t.Fatalf("リンクの先: %q", b)
	}
	if info, _ := os.Stat(real); info.Mode().Perm() != 0o644 {
		t.Fatalf("権限が変わりました: %v", info.Mode())
	}
}

// 設定ファイルに書き間違いがあって読めないとき、行を見つけられない書き方のときは書き戻さず、理由を返す。
func TestAppearanceIsNotWrittenToAnUnreadableConfig(t *testing.T) {
	s := newTestServer(t)
	c := login(t, s, "")
	for _, tc := range []struct{ body, says string }{
		{"[theme]\nname = \"light\"\nbackgroud = \"#000\"\n", "theme.backgroud"}, // 知らない項目
		{"[theme\nname = \"light\"\n", "has a mistake"},                          // TOML として読めない
		{"language = \"fr\"\n", "has a mistake"},                                 // 値の間違い
		{"theme = { name = \"light\" }\n", "edit [theme] by hand"},               // インラインのテーブル
		{"theme.name = \"light\"\n", "edit [theme] by hand"},                     // ドットの付いた書き方
		{"[theme]\nname = \"\"\"light\"\"\"\n", "not a one-line string"},
	} {
		writeConfig(t, tc.body)
		code, msg := chooseAppearance(t, s, c, `{"theme":"solarized-dark"}`)
		if code != http.StatusConflict || !strings.Contains(msg, tc.says) {
			t.Errorf("%q: code=%d %s", tc.body, code, msg)
		}
		if got := readConfigFile(t); got != tc.body {
			t.Errorf("%q を書き換えました: %q", tc.body, got)
		}
	}
}

func TestAppearanceRejectsUnknownNames(t *testing.T) {
	s := newTestServer(t)
	c := login(t, s, "")
	writeConfig(t, "# mine\n")
	for _, tc := range []struct {
		body string
		code int
	}{
		{`{"theme":"dracula"}`, http.StatusBadRequest},
		{`{"font":"../config.toml"}`, http.StatusBadRequest},
		{`{"font":"notes.txt"}`, http.StatusBadRequest},
		{`{"font":"Missing.ttf"}`, http.StatusNotFound},
		{`{}`, http.StatusBadRequest},
	} {
		if code, msg := chooseAppearance(t, s, c, tc.body); code != tc.code {
			t.Errorf("%s: code=%d %s", tc.body, code, msg)
		}
	}
	if code, _ := chooseAppearance(t, s, nil, `{"theme":"light"}`); code != http.StatusUnauthorized {
		t.Errorf("ログインしていない端末: code=%d", code)
	}
	if got := readConfigFile(t); got != "# mine\n" {
		t.Fatalf("書き換えました: %q", got)
	}
}

// 選べるテーマの名前は、画面の組み込みのテーマ（web/src/theme.ts の THEMES）と同じ。
func TestThemeNamesMatchTheScreen(t *testing.T) {
	src, err := os.ReadFile("web/src/theme.ts")
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`(?s)export const THEMES[^{]*\{(.*?)\n\};`).FindSubmatch(src)
	if block == nil {
		t.Fatal("THEMES が見つかりません")
	}
	var names []string
	for _, m := range regexp.MustCompile(`(?m)^  "([a-z-]+)": \{`).FindAllSubmatch(block[1], -1) {
		names = append(names, string(m[1]))
	}
	if !slices.Equal(names, themeNames) {
		t.Fatalf("画面: %v、サーバー: %v", names, themeNames)
	}
}
