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
	if !strings.Contains(body, `"name":"solarized-light"`) || !strings.Contains(body, `"file":"fonts/Mine.ttf"`) || !strings.Contains(body, `"url":"/api/font?i=0\u0026v=`) {
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
	if got, want := readConfigFile(t), "version = 2\n\n[theme]\nname = \"light\"\n\n[font]\nfile = \"fonts/A.woff2\"\n"; got != want {
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

// 画面で選んだ言語は language の行だけを書き換え、コメント・ほかの項目・並び順・改行の形・権限はそのまま残す。
// 行が無ければ最初のテーブルより前に足し、ファイルが無ければ作る。別の端末で読み込み直しても同じ言語になる。
func TestLanguageIsWrittenBackKeepingTheRest(t *testing.T) {
	s := newTestServer(t)
	phone, other := login(t, s, "phone"), login(t, s, "tablet")
	choose := func(lang string) {
		t.Helper()
		if code, body := chooseAppearance(t, s, phone, `{"language":"`+lang+`"}`); code != http.StatusOK {
			t.Fatalf("code=%d %s", code, body)
		}
	}

	// 行があれば値だけ書き換える（後ろのコメントは残る）
	writeConfig(t, "# 私の設定\nversion = 1\nlanguage = 'en'   # 言語\n\n[theme]\nname = \"light\"\n")
	if err := os.Chmod(settings.path, 0o640); err != nil {
		t.Fatal(err)
	}
	choose("ja")
	if got, want := readConfigFile(t), "# 私の設定\nversion = 1\nlanguage = \"ja\"   # 言語\n\n[theme]\nname = \"light\"\n"; got != want {
		t.Fatalf("書き戻した中身:\n%q", got)
	}
	if info, _ := os.Stat(settings.path); info.Mode().Perm() != 0o640 {
		t.Fatalf("権限が変わりました: %v", info.Mode())
	}
	w := authed(s, "GET", "/api/config", "", other)
	if !strings.Contains(w.Body.String(), `"language":"ja"`) {
		t.Fatalf("別の端末: %s", w.Body.String())
	}

	for _, tc := range []struct{ name, before, after string }{
		{"最初のテーブルより前の最後の項目の後ろに足す",
			"# 私の設定\nversion = 1\n\n# 色\n[theme]\nname = \"light\"\n",
			"# 私の設定\nversion = 1\nlanguage = \"ja\"\n\n# 色\n[theme]\nname = \"light\"\n"},
		{"項目が無ければ先頭のコメントの後ろに足す",
			"# 私の設定\n\n[theme]\nname = \"light\"\n",
			"# 私の設定\nlanguage = \"ja\"\n\n[theme]\nname = \"light\"\n"},
		{"テーブルしか無ければ先頭に足す",
			"[theme]\nname = \"light\"\n",
			"language = \"ja\"\n\n[theme]\nname = \"light\"\n"},
		{"CRLF と最後の改行の無さはそのまま",
			"version = 1\r\n[theme]\r\nname = \"light\"",
			"version = 1\r\nlanguage = \"ja\"\r\n[theme]\r\nname = \"light\""},
		{"テーブルの中の language は別のもの",
			"[[keys]]\nkey = \"esc\"\n",
			"language = \"ja\"\n\n[[keys]]\nkey = \"esc\"\n"},
	} {
		writeConfig(t, tc.before)
		choose("ja")
		if got := readConfigFile(t); got != tc.after {
			t.Errorf("%s:\n%q", tc.name, got)
		}
		if cfg, err := settings.get(); err != nil || cfg.Language != "ja" {
			t.Errorf("%s: %+v %v", tc.name, cfg, err)
		}
	}

	// ファイルが無ければ作る
	os.Remove(settings.path)
	choose("en")
	if got, want := readConfigFile(t), "version = 2\nlanguage = \"en\"\n"; got != want {
		t.Fatalf("作った中身:\n%q", got)
	}
	if info, _ := os.Stat(settings.path); info.Mode().Perm() != 0o600 {
		t.Fatalf("権限: %v", info.Mode())
	}
}

// 書き間違いがあって読めない設定ファイルには言語も書き戻さず、理由を返す。知らない言語は断る。
func TestLanguageIsNotWrittenToAnUnreadableConfig(t *testing.T) {
	s := newTestServer(t)
	c := login(t, s, "")
	for _, tc := range []struct{ body, says string }{
		{"language = \"en\"\n[theme]\nbackgroud = \"#000\"\n", "has a mistake"},
		{"language = \"en\n", "has a mistake"},
		{"language = \"\"\"en\"\"\"\n", "not a one-line string"},
	} {
		writeConfig(t, tc.body)
		code, msg := chooseAppearance(t, s, c, `{"language":"ja"}`)
		if code != http.StatusConflict || !strings.Contains(msg, tc.says) {
			t.Errorf("%q: code=%d %s", tc.body, code, msg)
		}
		if got := readConfigFile(t); got != tc.body {
			t.Errorf("%q を書き換えました: %q", tc.body, got)
		}
	}
	writeConfig(t, "# mine\n")
	for _, body := range []string{`{"language":"fr"}`, `{"language":""}`} {
		if code, msg := chooseAppearance(t, s, c, body); code != http.StatusBadRequest {
			t.Errorf("%s: code=%d %s", body, code, msg)
		}
	}
	if code, _ := chooseAppearance(t, s, nil, `{"language":"ja"}`); code != http.StatusUnauthorized {
		t.Errorf("ログインしていない端末: code=%d", code)
	}
	if got := readConfigFile(t); got != "# mine\n" {
		t.Fatalf("書き換えました: %q", got)
	}
}

// stack の書き方なら、一覧の「使う」は stack をそのフォント1つに書き換え、「既定のフォント」は stack の行を消す。
// 版 1 の書き方なら今までどおり file を書き換える（TestAppearanceIsWrittenBackKeepingTheRest）。
func TestAppearanceRewritesTheStack(t *testing.T) {
	s := newTestServer(t)
	c := login(t, s, "phone")
	putFont(t, "Mine.ttf")
	src := "version = 2\n\n[font]\n# 好きな順\nstack = [\"fonts/Old.woff2\", 'Noto Sans Mono CJK JP', \"JetBrains Mono\"]   # 並び\n\n[theme]\nname = \"light\"\n"
	writeConfig(t, src)
	code, body := chooseAppearance(t, s, c, `{"font":"Mine.ttf"}`)
	if code != http.StatusOK {
		t.Fatalf("code=%d %s", code, body)
	}
	want := "version = 2\n\n[font]\n# 好きな順\nstack = [\"fonts/Mine.ttf\"]   # 並び\n\n[theme]\nname = \"light\"\n"
	if got := readConfigFile(t); got != want {
		t.Fatalf("書き戻した中身:\n%s", got)
	}
	if cfg, err := settings.get(); err != nil || !slices.Equal(cfg.Font.Stack, []string{"fonts/Mine.ttf"}) || cfg.Font.File != "" {
		t.Fatalf("%+v %v", cfg.Font, err)
	}
	if w := authed(s, "GET", "/api/fonts", "", c); !strings.Contains(w.Body.String(), `"file":"fonts/Mine.ttf"`) {
		t.Fatalf("一覧の使用中: %s", w.Body.String())
	}

	if code, body := chooseAppearance(t, s, c, `{"font":""}`); code != http.StatusOK {
		t.Fatalf("code=%d %s", code, body)
	}
	if got, want := readConfigFile(t), "version = 2\n\n[font]\n# 好きな順\n\n[theme]\nname = \"light\"\n"; got != want {
		t.Fatalf("既定に戻した中身:\n%s", got)
	}

	// 何行にも分けて書いた stack は書き換えず、手で直すよう知らせる
	multi := "version = 2\n[font]\nstack = [\n  \"fonts/Old.woff2\",\n  \"JetBrains Mono\",\n]\n"
	writeConfig(t, multi)
	if code, body := chooseAppearance(t, s, c, `{"font":"Mine.ttf"}`); code != http.StatusConflict || !strings.Contains(body, "one-line list") {
		t.Fatalf("code=%d %s", code, body)
	}
	if got := readConfigFile(t); got != multi {
		t.Fatalf("書き換えてしまいました:\n%s", got)
	}
}

// 画面の「使う順」の並びで、[font] の stack をその順に書き換える。並べられるのは fonts/ の直下のフォント・組み込み・PC のフォントと、
// 今の stack にもう書いてあるものだけ。空にすると stack の行を消して既定に戻す。版 1 の family・file は消して stack にする。
func TestAppearanceSetsTheWholeStack(t *testing.T) {
	s := newTestServer(t)
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	forgetFont()
	t.Cleanup(forgetFont)
	writeFile(t, filepath.Join(data, "fonts", "PalmtermTestMono-Regular.ttf"), fakeFont("Palmterm Test Mono", "Regular"))
	c, other := login(t, s, "phone"), login(t, s, "tablet")
	putFont(t, "Mine.ttf")

	src := "version = 2\n\n[font]\nstack = [\"~/my/Hand.otf\", \"Phone Only Sans\"]  # 並び\n\n[theme]\nname = \"light\"\n"
	writeConfig(t, src)
	code, body := chooseAppearance(t, s, c, `{"stack":["fonts/Mine.ttf","palmterm test mono","Phone Only Sans","jetbrains mono","~/my/Hand.otf"]}`)
	if code != http.StatusOK {
		t.Fatalf("code=%d %s", code, body)
	}
	want := "version = 2\n\n[font]\nstack = [\"fonts/Mine.ttf\", \"Palmterm Test Mono\", \"Phone Only Sans\", \"JetBrains Mono\", \"~/my/Hand.otf\"]  # 並び\n\n[theme]\nname = \"light\"\n"
	if got := readConfigFile(t); got != want {
		t.Fatalf("書き戻した中身:\n%s", got)
	}
	// 別の端末で読み込み直しても同じ並び
	r := getFontConfig(t, s, other)
	if r.Error != "" || len(r.FontFaces) < 4 || r.FontFaces[0].URL == "" || r.FontFaces[2].Family != "Phone Only Sans" || r.FontFaces[3].Family != "JetBrains Mono" {
		t.Fatalf("%+v", r)
	}

	// 入れ替えると、その順に書く
	if code, body := chooseAppearance(t, s, c, `{"stack":["JetBrains Mono","fonts/Mine.ttf"]}`); code != http.StatusOK {
		t.Fatalf("code=%d %s", code, body)
	}
	if got := readConfigFile(t); !strings.Contains(got, "stack = [\"JetBrains Mono\", \"fonts/Mine.ttf\"]  # 並び\n") {
		t.Fatalf("書き戻した中身:\n%s", got)
	}

	// 無い・並べられないもの、同じものを2回、font と一緒、ログインしていない、は断って書き換えない
	before := readConfigFile(t)
	for _, req := range []string{
		`{"stack":["No Such Font Palmterm"]}`,
		`{"stack":["fonts/../config.toml"]}`,
		`{"stack":["fonts/Gone.ttf"]}`,
		`{"stack":["~/other/secret.ttf"]}`,
		`{"stack":["fonts/Mine.ttf","JetBrains Mono","jetbrains mono"]}`,
		`{"stack":["JetBrains Mono"],"font":"Mine.ttf"}`,
		`{"stack":["JetBrains Mono"],"systemFont":"JetBrains Mono"}`,
	} {
		if code, body := chooseAppearance(t, s, c, req); code == http.StatusOK {
			t.Fatalf("%s: 受け付けました %s", req, body)
		}
	}
	if code, _ := chooseAppearance(t, s, nil, `{"stack":[]}`); code == http.StatusOK {
		t.Fatalf("ログインしていないのに書き戻しました")
	}
	if got := readConfigFile(t); got != before {
		t.Fatalf("書き換えました:\n%s", got)
	}

	// 空にすると stack の行を消し、既定で描く
	if code, body := chooseAppearance(t, s, c, `{"stack":[]}`); code != http.StatusOK {
		t.Fatalf("code=%d %s", code, body)
	}
	if got, want := readConfigFile(t), "version = 2\n\n[font]\n\n[theme]\nname = \"light\"\n"; got != want {
		t.Fatalf("空にした中身:\n%s", got)
	}
	if r := getFontConfig(t, s, other); len(r.FontFaces) != 0 || r.Error != "" {
		t.Fatalf("%+v", r)
	}

	// 版 1 の書き方なら family・file を消して stack にする
	writeConfig(t, "version = 1\n[font]\nfamily = \"Old\"\nfile = \"fonts/Mine.ttf\"\n")
	if code, body := chooseAppearance(t, s, c, `{"stack":["fonts/Mine.ttf","JetBrains Mono"]}`); code != http.StatusOK {
		t.Fatalf("code=%d %s", code, body)
	}
	if got, want := readConfigFile(t), "version = 1\n[font]\nstack = [\"fonts/Mine.ttf\", \"JetBrains Mono\"]\n"; got != want {
		t.Fatalf("版 1 からの中身:\n%s", got)
	}
}
