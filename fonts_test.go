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
	FontURL   string     `json:"fontURL"`
	FontError string     `json:"fontError"`
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
	if r.Font.Family != "palmterm test mono" || !strings.HasPrefix(r.FontURL, "/api/font?") || r.FontError != "" {
		t.Fatalf("%+v", r)
	}
	w := authed(s, "GET", r.FontURL, "", c)
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), regular) || w.Header().Get("Content-Type") != "font/ttf" {
		t.Fatalf("code=%d type=%q len=%d", w.Code, w.Header().Get("Content-Type"), w.Body.Len())
	}

	forgetFont()
	writeConfig(t, "[font]\nfamily = \"No Such Font Palmterm\"\n")
	r = getFontConfig(t, s, c)
	if r.Font.Family != "No Such Font Palmterm" || r.FontURL != "" || r.FontError != "" {
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
	if r.FontURL == "" || r.FontError != "" {
		t.Fatalf("%+v", r)
	}
	w := authed(s, "GET", r.FontURL, "", c)
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), woff2) || w.Header().Get("Content-Type") != "font/woff2" {
		t.Fatalf("code=%d type=%q", w.Code, w.Header().Get("Content-Type"))
	}

	for _, file := range []string{"notes.txt", "missing.ttf", "fonts"} {
		forgetFont()
		writeConfig(t, "[font]\nfile = \""+file+"\"\n")
		r := getFontConfig(t, s, c)
		if r.FontURL != "" || !strings.Contains(r.FontError, file) {
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
	writeConfig(t, "version = 2\n[font]\nfile = \"my.otf\"\nweight = 300\n")
	if r := getFontConfig(t, s, c); r.FontURL == "" || r.Font.File != "my.otf" {
		t.Fatalf("%+v", r)
	}
}
