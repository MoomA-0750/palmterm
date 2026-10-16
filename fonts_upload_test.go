package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// フォームの file にフォントを1つ入れて /api/fonts に送る（ファイル名はそのまま送る）。
func uploadFont(t *testing.T, s *server, c *http.Cookie, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "x")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(data)
	mw.Close()
	// CreateFormFile は名前の中の / を残さないので、送る名前は見出しを直接書き換える。
	raw := bytes.Replace(body.Bytes(), []byte(`filename="x"`), []byte(`filename="`+filename+`"`), 1)
	r := httptest.NewRequest("POST", "/api/fonts", bytes.NewReader(raw))
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.AddCookie(c)
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	return w
}

func listedFonts(t *testing.T, s *server, c *http.Cookie) []savedFont {
	t.Helper()
	w := authed(s, "GET", "/api/fonts", "", c)
	if w.Code != http.StatusOK {
		t.Fatalf("一覧: code=%d body=%s", w.Code, w.Body)
	}
	var resp struct{ Fonts []savedFont }
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp.Fonts
}

// 画面から送ったフォントは、設定ファイルのあるフォルダの fonts/ に保存され、[font] の file = "fonts/名前" で使える。
func TestUploadedFontIsSavedAndUsable(t *testing.T) {
	s := newTestServer(t)
	c := login(t, s, "")
	forgetFont()
	t.Cleanup(forgetFont)
	dir := filepath.Join(filepath.Dir(settings.path), "fonts")

	font := fakeFont("Palmterm Upload", "Regular")
	woff2 := append([]byte("wOF2"), make([]byte, 60)...)
	for _, f := range []struct {
		name, saved string
		data        []byte
	}{
		{"My Font.ttf", "My_Font.ttf", font},
		{"Other.WOFF2", "Other.woff2", woff2},
		{"My Font.ttf", "My_Font-1.ttf", font}, // 同じ名前は上書きせず、番号を付ける
	} {
		w := uploadFont(t, s, c, f.name, f.data)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: code=%d body=%s", f.name, w.Code, w.Body)
		}
		var resp struct{ Name, File string }
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Name != f.saved || resp.File != "fonts/"+f.saved {
			t.Fatalf("%s: 応答 %+v、%s のはず", f.name, resp, f.saved)
		}
		if got, err := os.ReadFile(filepath.Join(dir, f.saved)); err != nil || !bytes.Equal(got, f.data) {
			t.Fatalf("%s: 保存されていません: %v", f.saved, err)
		}
	}
	got := listedFonts(t, s, c)
	if len(got) != 3 || got[0].Name != "My_Font-1.ttf" || got[1].Name != "My_Font.ttf" || got[2].Name != "Other.woff2" || got[1].Size != int64(len(font)) {
		t.Fatalf("一覧: %+v", got)
	}

	writeConfig(t, "[font]\nfile = \"fonts/My_Font.ttf\"\n")
	r := getFontConfig(t, s, c)
	if r.FontURL == "" || r.FontError != "" {
		t.Fatalf("file = fonts/My_Font.ttf を配りません: %+v", r)
	}
	w := authed(s, "GET", r.FontURL, "", c)
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), font) {
		t.Fatalf("配ったフォント: code=%d", w.Code)
	}

	// ログインしていなければ、保存も一覧もできない。
	if w := uploadFont(t, s, &http.Cookie{Name: "x", Value: "y"}, "Evil.ttf", font); w.Code == http.StatusOK {
		t.Fatalf("ログインしていないのに保存できました")
	}
	if _, err := os.Stat(filepath.Join(dir, "Evil.ttf")); err == nil {
		t.Fatalf("ログインしていないのに保存されました")
	}
	if w := authed(s, "GET", "/api/fonts", "", nil); w.Code == http.StatusOK {
		t.Fatalf("ログインしていないのに一覧が出ました")
	}
}

// フォントでないファイル・大きすぎるファイルは保存せず理由を返す。名前に ../ などがあっても fonts/ の外には書かない。
func TestFontUploadRejectsNonFontsAndStaysInFontsDir(t *testing.T) {
	s := newTestServer(t)
	c := login(t, s, "")
	base := filepath.Dir(settings.path)
	dir := filepath.Join(base, "fonts")
	font := fakeFont("Palmterm Upload", "Regular")

	for _, f := range []struct {
		name string
		data []byte
		code int
	}{
		{"notes.ttf", []byte("hello, this is not a font"), http.StatusBadRequest}, // 拡張子だけフォント
		{"photo.png", []byte("\x89PNG\r\n\x1a\n...."), http.StatusBadRequest},
		{"font.png", font, http.StatusBadRequest}, // 中身はフォントでも拡張子が違う
		{"coll.ttc", append([]byte("ttcf"), font...), http.StatusBadRequest},
		{"tiny.ttf", []byte("OT"), http.StatusBadRequest},
	} {
		w := uploadFont(t, s, c, f.name, f.data)
		if w.Code != f.code || w.Body.Len() == 0 {
			t.Errorf("%s: code=%d body=%q", f.name, w.Code, w.Body)
		}
	}
	if got := listedFonts(t, s, c); len(got) != 0 {
		t.Fatalf("保存されたものがあります: %+v", got)
	}

	old := maxFontBytes
	maxFontBytes = 1 << 20
	t.Cleanup(func() { maxFontBytes = old })
	big := append(append([]byte{}, font...), make([]byte, 1<<20)...)
	if w := uploadFont(t, s, c, "Big.ttf", big); w.Code != http.StatusRequestEntityTooLarge || !bytes.Contains(w.Body.Bytes(), []byte("MB")) {
		t.Fatalf("大きすぎるファイル: code=%d body=%q", w.Code, w.Body)
	}
	huge := append(append([]byte{}, font...), make([]byte, 3<<20)...) // フォームの見出しの分の余裕も超える
	if w := uploadFont(t, s, c, "Huge.ttf", huge); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("とても大きいファイル: code=%d body=%q", w.Code, w.Body)
	}
	if got := listedFonts(t, s, c); len(got) != 0 {
		t.Fatalf("大きすぎるのに保存されました: %+v", got)
	}

	for _, name := range []string{"../escape.ttf", "../../escape2.ttf", `..\escape3.ttf`, "/tmp/abs.ttf", "a/../../escape4.ttf", "..", ".ttf"} {
		w := uploadFont(t, s, c, name, font)
		if w.Code == http.StatusOK {
			var resp struct{ Name string }
			json.Unmarshal(w.Body.Bytes(), &resp)
			if _, err := os.Stat(filepath.Join(dir, resp.Name)); err != nil || filepath.Base(resp.Name) != resp.Name {
				t.Errorf("%s: fonts/ の直下にありません（%q）", name, resp.Name)
			}
		}
	}
	// fonts/ の外（設定ファイルのフォルダとその上）にフォントができていない。
	for _, d := range []string{base, filepath.Dir(base)} {
		entries, _ := os.ReadDir(d)
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".ttf" {
				t.Errorf("fonts/ の外に書きました: %s", filepath.Join(d, e.Name()))
			}
		}
	}
	if _, err := os.Stat("/tmp/abs.ttf"); err == nil {
		t.Errorf("/tmp/abs.ttf に書きました")
	}
}

// 保存したフォントは名前で消せる。fonts/ の直下のフォントのほかは消さない。
func TestFontDelete(t *testing.T) {
	s := newTestServer(t)
	c := login(t, s, "")
	base := filepath.Dir(settings.path)
	if w := uploadFont(t, s, c, "Gone.ttf", fakeFont("Palmterm Upload", "Regular")); w.Code != http.StatusOK {
		t.Fatalf("保存: code=%d", w.Code)
	}
	writeFile(t, filepath.Join(base, "keep.ttf"), []byte("OTTO"))
	writeFile(t, filepath.Join(base, "fonts", "notes.txt"), []byte("x"))

	for _, name := range []string{"../keep.ttf", "..", "notes.txt", "missing.ttf", "../config.toml", ""} {
		w := authed(s, "POST", "/api/fonts/delete", `{"name":`+jsonString(name)+`}`, c)
		if w.Code < 400 {
			t.Errorf("%q: code=%d、断るはず", name, w.Code)
		}
	}
	if _, err := os.Stat(filepath.Join(base, "keep.ttf")); err != nil {
		t.Fatalf("fonts/ の外のファイルを消しました")
	}
	if _, err := os.Stat(filepath.Join(base, "fonts", "notes.txt")); err != nil {
		t.Fatalf("フォントでないファイルを消しました")
	}

	if w := authed(s, "POST", "/api/fonts/delete", `{"name":"Gone.ttf"}`, nil); w.Code < 400 {
		t.Fatalf("ログインしていないのに消せました")
	}
	if w := authed(s, "POST", "/api/fonts/delete", `{"name":"Gone.ttf"}`, c); w.Code != http.StatusNoContent {
		t.Fatalf("消す: code=%d body=%s", w.Code, w.Body)
	}
	if got := listedFonts(t, s, c); len(got) != 0 {
		t.Fatalf("消えていません: %+v", got)
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
