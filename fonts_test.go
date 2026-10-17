package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

// 名前だけを持つ TrueType のファイルの中身（palmterm が名前を読むのに要る所だけ）。
func fakeFont(family, style string) []byte {
	type rec struct {
		id   uint16
		text string
	}
	recs := []rec{{1, family}, {2, style}}
	var strs bytes.Buffer
	name := new(bytes.Buffer)
	binary.Write(name, binary.BigEndian, []uint16{0, uint16(len(recs)), uint16(6 + 12*len(recs))})
	for _, r := range recs {
		u := utf16.Encode([]rune(r.text))
		binary.Write(name, binary.BigEndian, []uint16{3, 1, 0x409, r.id, uint16(2 * len(u)), uint16(strs.Len())})
		binary.Write(&strs, binary.BigEndian, u)
	}
	name.Write(strs.Bytes())
	var b bytes.Buffer
	binary.Write(&b, binary.BigEndian, []uint32{0x00010000})
	binary.Write(&b, binary.BigEndian, []uint16{1, 16, 0, 0})
	b.WriteString("name")
	binary.Write(&b, binary.BigEndian, []uint32{0, 12 + 16, uint32(name.Len())})
	b.Write(name.Bytes())
	return b.Bytes()
}

func writeFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

type fontResponse struct {
	Font      fontConfig `json:"font"`
	FontFaces []fontFace `json:"fontFaces"`
	FontError string     `json:"fontError"`
	Error     string     `json:"error"`
}

// 最初に配るフォントの URL（配るものが無ければ空）。
func (r fontResponse) FontURL() string {
	for _, f := range r.FontFaces {
		if f.URL != "" {
			return f.URL
		}
	}
	return ""
}

func getFontConfig(t *testing.T, s *server, c *http.Cookie) fontResponse {
	t.Helper()
	w := authed(s, "GET", "/api/config", "", c)
	var r fontResponse
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// [font] の family に PC のフォントの名前を書くと、そのファイル（普通の太さのもの）を配る。
// 無い名前なら配らない（画面はスマホのフォント、無ければ JetBrains Mono を使う）。
func TestFontFamilyIsFoundOnThePC(t *testing.T) {
	s := newTestServer(t)
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	forgetFont()
	t.Cleanup(forgetFont)
	regular := fakeFont("Palmterm Test Mono", "Regular")
	writeFile(t, filepath.Join(data, "fonts", "a", "PalmtermTestMono-Bold.ttf"), fakeFont("Palmterm Test Mono", "Bold"))
	writeFile(t, filepath.Join(data, "fonts", "b", "PalmtermTestMono-Regular.ttf"), regular)
	writeFile(t, filepath.Join(data, "fonts", "Other.ttf"), fakeFont("Other Mono", "Regular"))
	c := login(t, s, "phone")

	writeConfig(t, "[font]\nfamily = \"palmterm test mono\"\n")
	r := getFontConfig(t, s, c)
	if r.Font.Family != "palmterm test mono" || !strings.HasPrefix(r.FontURL(), "/api/font?") || r.FontError != "" {
		t.Fatalf("%+v", r)
	}
	w := authed(s, "GET", r.FontURL(), "", c)
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), regular) || w.Header().Get("Content-Type") != "font/ttf" {
		t.Fatalf("code=%d type=%q len=%d", w.Code, w.Header().Get("Content-Type"), w.Body.Len())
	}

	forgetFont()
	writeConfig(t, "[font]\nfamily = \"No Such Font Palmterm\"\n")
	r = getFontConfig(t, s, c)
	if r.Font.Family != "No Such Font Palmterm" || r.FontURL() != "" || r.FontError != "" {
		t.Fatalf("%+v", r)
	}
	if w := authed(s, "GET", "/api/font", "", c); w.Code != http.StatusNotFound {
		t.Fatalf("code=%d", w.Code)
	}
}

// [font] の file に書いたファイルを配る。相対パスは設定ファイルのあるフォルダから数える。
// フォントでないファイルや無いファイルは配らず、画面で知らせる理由を返す。
func TestFontFile(t *testing.T) {
	s := newTestServer(t)
	forgetFont()
	t.Cleanup(forgetFont)
	dir := filepath.Dir(settings.path)
	woff2 := append([]byte("wOF2"), bytes.Repeat([]byte{1}, 100)...)
	writeFile(t, filepath.Join(dir, "fonts", "my.woff2"), woff2)
	writeFile(t, filepath.Join(dir, "notes.txt"), []byte("secret"))
	c := login(t, s, "phone")

	writeConfig(t, "[font]\nfile = \"fonts/my.woff2\"\nfamily = \"My Font\"\n")
	r := getFontConfig(t, s, c)
	if r.FontURL() == "" || r.FontError != "" {
		t.Fatalf("%+v", r)
	}
	w := authed(s, "GET", r.FontURL(), "", c)
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), woff2) || w.Header().Get("Content-Type") != "font/woff2" {
		t.Fatalf("code=%d type=%q", w.Code, w.Header().Get("Content-Type"))
	}

	for _, file := range []string{"notes.txt", "missing.ttf", "fonts"} {
		forgetFont()
		writeConfig(t, "[font]\nfile = \""+file+"\"\n")
		r := getFontConfig(t, s, c)
		if r.FontURL() != "" || !strings.Contains(r.FontError, file) {
			t.Fatalf("%s: %+v", file, r)
		}
		if w := authed(s, "GET", "/api/font", "", c); w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("%s: code=%d", file, w.Code)
		}
	}
}

// フォントを配るのはログインした端末にだけで、URL をどう書いても [font] で決めたファイルのほかは配らない。
func TestFontIsServedOnlyToLoggedInDevicesAndOnlyThatFile(t *testing.T) {
	s := newTestServer(t)
	forgetFont()
	t.Cleanup(forgetFont)
	dir := filepath.Dir(settings.path)
	font := append([]byte("OTTO"), bytes.Repeat([]byte{2}, 100)...)
	writeFile(t, filepath.Join(dir, "my.otf"), font)
	writeFile(t, filepath.Join(dir, "token"), []byte("OTTO-looks-like-a-font-but-is-not-the-chosen-one"))
	writeConfig(t, "[font]\nfile = \"my.otf\"\n")
	h := s.routes()

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/font", nil))
	if w.Code == http.StatusOK || bytes.Equal(w.Body.Bytes(), font) {
		t.Fatalf("ログインしていないのに配りました: code=%d", w.Code)
	}

	c := login(t, s, "phone")
	for _, target := range []string{"/api/font", "/api/font?file=token", "/api/font?path=" + filepath.Join(dir, "token"), "/api/font?v=1&file=../token"} {
		r := httptest.NewRequest("GET", target, nil)
		r.AddCookie(c)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), font) {
			t.Fatalf("%s: code=%d body=%q", target, w.Code, w.Body.String())
		}
	}
	for _, target := range []string{"/api/font/token", "/api/font/../token", "/api/fonts"} {
		r := httptest.NewRequest("GET", target, nil)
		r.AddCookie(c)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if strings.Contains(w.Body.String(), "not-the-chosen-one") {
			t.Fatalf("%s: ほかのファイルを配りました", target)
		}
	}

	// 新しい版の設定ファイルでも [font] は読む。
	forgetFont()
	writeConfig(t, "version = 3\n[font]\nfile = \"my.otf\"\nweight = 300\n")
	if r := getFontConfig(t, s, c); r.FontURL() == "" || r.Font.File != "my.otf" {
		t.Fatalf("%+v", r)
	}
}

// [font] の stack（版 2）は、書いた順にフォントを並べて画面に渡す。拡張子で終わればファイル、JetBrains Mono は
// 組み込み、ほかは PC のフォントの名前で、PC に無ければスマホの同じ名前のフォントを使う。無いファイルは理由を知らせて飛ばす。
// 配るのはログインした端末にだけで、並びにあるファイルだけ。
func TestFontStack(t *testing.T) {
	s := newTestServer(t)
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	forgetFont()
	t.Cleanup(forgetFont)
	dir := filepath.Dir(settings.path)
	mine := append([]byte("wOF2"), bytes.Repeat([]byte{1}, 100)...)
	writeFile(t, filepath.Join(dir, "fonts", "MyMono.woff2"), mine)
	pc := fakeFont("Palmterm Stack CJK", "Regular")
	writeFile(t, filepath.Join(data, "fonts", "PalmtermStackCJK-Regular.ttf"), pc)
	writeFile(t, filepath.Join(dir, "token"), []byte("OTTO-not-in-the-stack"))
	c := login(t, s, "phone")

	writeConfig(t, `version = 2
[font]
stack = ["fonts/MyMono.woff2", "Palmterm Stack CJK", "fonts/Missing.ttf", "Phone Only Font", "jetbrains mono"]
`)
	r := getFontConfig(t, s, c)
	if r.Error != "" || len(r.FontFaces) != 4 {
		t.Fatalf("%+v", r)
	}
	if f := r.FontFaces[0]; !strings.HasPrefix(f.URL, "/api/font?i=0&") || f.Family != "" {
		t.Fatalf("ファイル: %+v", f)
	}
	if f := r.FontFaces[1]; !strings.HasPrefix(f.URL, "/api/font?i=1&") || f.Family != "Palmterm Stack CJK" {
		t.Fatalf("PC のフォント: %+v", f)
	}
	if f := r.FontFaces[2]; f != (fontFace{Family: "Phone Only Font"}) {
		t.Fatalf("PC に無いフォント: %+v", f)
	}
	if f := r.FontFaces[3]; f != (fontFace{Family: "JetBrains Mono"}) {
		t.Fatalf("組み込み: %+v", f)
	}
	if !strings.Contains(r.FontError, "Missing.ttf") {
		t.Fatalf("無いファイルの知らせ: %q", r.FontError)
	}
	for i, want := range [][]byte{mine, pc} {
		w := authed(s, "GET", r.FontFaces[i].URL, "", c)
		if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), want) {
			t.Fatalf("%d: code=%d len=%d", i, w.Code, w.Body.Len())
		}
	}

	// 並びに無い番号・ファイルは配らない。ログインしていなければ配らない
	h := s.routes()
	for _, target := range []string{"/api/font?i=2", "/api/font?i=-1", "/api/font?i=x", "/api/font?i=1e0", "/api/font?i=0&i=../token", "/api/font?file=token&i=9"} {
		req := httptest.NewRequest("GET", target, nil)
		req.AddCookie(c)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if strings.Contains(w.Body.String(), "not-in-the-stack") || (target != "/api/font?i=0&i=../token" && w.Code != http.StatusNotFound) {
			t.Fatalf("%s: code=%d", target, w.Code)
		}
	}
	for _, target := range []string{"/api/font", "/api/font?i=1"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", target, nil))
		if w.Code == http.StatusOK || bytes.Equal(w.Body.Bytes(), mine) || bytes.Equal(w.Body.Bytes(), pc) {
			t.Fatalf("%s: ログインしていないのに配りました: code=%d", target, w.Code)
		}
	}

	// stack と family・file を一緒に書くと書き間違い
	for _, body := range []string{
		"version = 2\n[font]\nstack = [\"JetBrains Mono\"]\nfamily = \"X\"\n",
		"version = 2\n[font]\nstack = []\nfile = \"fonts/MyMono.woff2\"\n",
	} {
		forgetFont()
		writeConfig(t, body)
		if r := getFontConfig(t, s, c); !strings.Contains(r.Error, "stack") || len(r.FontFaces) != 0 {
			t.Fatalf("%q: %+v", body, r)
		}
	}

	// 版 1 の書き方も今までどおり読める（file を使い、family はスマホのもの）
	forgetFont()
	writeConfig(t, "version = 1\n[font]\nfile = \"fonts/MyMono.woff2\"\nfamily = \"Palmterm Stack CJK\"\n")
	r = getFontConfig(t, s, c)
	if r.Error != "" || len(r.FontFaces) != 2 || r.FontFaces[0].URL == "" || r.FontFaces[1] != (fontFace{Family: "Palmterm Stack CJK"}) {
		t.Fatalf("%+v", r)
	}
	if w := authed(s, "GET", r.FontURL(), "", c); !bytes.Equal(w.Body.Bytes(), mine) {
		t.Fatalf("code=%d", w.Code)
	}
}

// フォントの一覧には PC に入っているフォントを名前ごとに1つずつ出し（ファイルのパスは出さない）、
// 「使う」で [font] の stack をそのフォント1つにすると、そのファイルを配る。一覧も配るのもログインした端末にだけ。
func TestSystemFontsAreListedAndChosen(t *testing.T) {
	s := newTestServer(t)
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	forgetFont()
	t.Cleanup(forgetFont)
	regular := fakeFont("Palmterm Test Mono", "Regular")
	writeFile(t, filepath.Join(data, "fonts", "PalmtermTestMono-Bold.ttf"), fakeFont("Palmterm Test Mono", "Bold"))
	writeFile(t, filepath.Join(data, "fonts", "PalmtermTestMono-Regular.ttf"), regular)
	writeFile(t, filepath.Join(data, "fonts", "sub", "Other.otf"), fakeFont("Palmterm Other", "Regular"))
	writeFile(t, filepath.Join(data, "fonts", "JetBrainsMono.ttf"), fakeFont("JetBrains Mono", "Regular"))
	c := login(t, s, "phone")

	if w := authed(s, "GET", "/api/fonts", "", nil); w.Code == http.StatusOK || strings.Contains(w.Body.String(), "Palmterm") {
		t.Fatalf("ログインしていないのに一覧を返しました: code=%d", w.Code)
	}
	w := authed(s, "GET", "/api/fonts", "", c)
	var list struct {
		System  []string `json:"system"`
		Builtin string   `json:"builtin"`
		File    string   `json:"file"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	for _, n := range list.System {
		count[n]++
	}
	if count["Palmterm Test Mono"] != 1 || count["Palmterm Other"] != 1 || count["JetBrains Mono"] != 0 || list.Builtin != "JetBrains Mono" {
		t.Fatalf("%+v", list)
	}
	if strings.Contains(w.Body.String(), data) || strings.Contains(w.Body.String(), ".ttf") || strings.Contains(w.Body.String(), ".otf") {
		t.Fatalf("ファイルのパスを出しました: %s", w.Body.String())
	}

	// 版 1 の書き方なら family・file の行を消して stack にする（一緒には書けないため）
	writeConfig(t, "version = 1\n[font]\nfamily = \"Old\"\nfile = \"old.ttf\"\n# keep\n\n[theme]\nname = \"light\"\n")
	if code, body := chooseAppearance(t, s, c, `{"systemFont":"palmterm test mono"}`); code != http.StatusOK {
		t.Fatalf("code=%d %s", code, body)
	}
	if got, want := readConfigFile(t), "version = 1\n[font]\nstack = [\"Palmterm Test Mono\"]\n# keep\n\n[theme]\nname = \"light\"\n"; got != want {
		t.Fatalf("書き戻した中身:\n%s", got)
	}
	r := getFontConfig(t, s, c)
	if r.FontURL() == "" || r.FontError != "" || r.Error != "" {
		t.Fatalf("%+v", r)
	}
	if w := authed(s, "GET", r.FontURL(), "", c); w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), regular) {
		t.Fatalf("code=%d len=%d", w.Code, w.Body.Len())
	}
	if w := authed(s, "GET", r.FontURL(), "", nil); w.Code == http.StatusOK {
		t.Fatalf("ログインしていないのに配りました")
	}
	if w := authed(s, "GET", "/api/fonts", "", c); !strings.Contains(w.Body.String(), `"file":"Palmterm Test Mono"`) {
		t.Fatalf("一覧の使用中: %s", w.Body.String())
	}

	// 組み込みの JetBrains Mono は配らず、画面の組み込みのものを使う
	if code, body := chooseAppearance(t, s, c, `{"systemFont":"jetbrains mono"}`); code != http.StatusOK {
		t.Fatalf("code=%d %s", code, body)
	}
	if got := readConfigFile(t); !strings.Contains(got, "stack = [\"JetBrains Mono\"]\n") {
		t.Fatalf("書き戻した中身:\n%s", got)
	}
	if r := getFontConfig(t, s, c); r.FontURL() != "" || len(r.FontFaces) != 1 || r.FontFaces[0].Family != "JetBrains Mono" {
		t.Fatalf("%+v", r)
	}

	// PC に無い名前やパスは断り、書き換えない
	before := readConfigFile(t)
	for _, name := range []string{"No Such Font Palmterm", "../config.toml", filepath.Join(data, "fonts", "PalmtermTestMono-Regular.ttf"), ""} {
		if code, body := chooseAppearance(t, s, c, `{"systemFont":`+jsonString(name)+`}`); code != http.StatusNotFound {
			t.Fatalf("%q: code=%d %s", name, code, body)
		}
	}
	if code, _ := chooseAppearance(t, s, nil, `{"systemFont":"Palmterm Other"}`); code == http.StatusOK {
		t.Fatalf("ログインしていないのに書き戻しました")
	}
	if got := readConfigFile(t); got != before {
		t.Fatalf("書き換えました:\n%s", got)
	}
}
