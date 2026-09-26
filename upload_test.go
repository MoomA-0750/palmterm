package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeFileName(t *testing.T) {
	cases := map[string]string{
		"photo.PNG":                      "photo.png",
		"赤い 四角.png":                      "file.png",
		"my photo (1).jpeg":              "my_photo_1.jpeg",
		"../../etc/passwd":               "passwd",
		".hidden":                        "file.hidden",
		"noext":                          "noext",
		strings.Repeat("a", 80) + ".gif": strings.Repeat("a", 60) + ".gif",
	}
	for in, want := range cases {
		if got := safeFileName(in); got != want {
			t.Errorf("safeFileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func uploadRequest(t *testing.T, files map[string][]byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for name, data := range files {
		fw, err := mw.CreateFormFile("file", name)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(data)
	}
	mw.Close()
	r := httptest.NewRequest("POST", "/api/upload", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r
}

func decodeUploaded(t *testing.T, w *httptest.ResponseRecorder) []uploadedFile {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
	var resp struct{ Files []uploadedFile }
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp.Files
}

func TestUploadSavesClaudeReadableImagesAsIs(t *testing.T) {
	s := newTestServer(t)
	data := []byte("\x89PNG fake")
	files := decodeUploaded(t, do(s.handleUpload, uploadRequest(t, map[string][]byte{"shot 1.png": data})))
	if len(files) != 1 {
		t.Fatalf("%+v", files)
	}
	f := files[0]
	if filepath.Dir(f.Path) != s.uploadDir || filepath.Base(f.Path) != f.Name || !strings.HasSuffix(f.Name, "-shot_1.png") {
		t.Fatalf("%+v", f)
	}
	if got, _ := os.ReadFile(f.Path); !bytes.Equal(got, data) {
		t.Fatalf("中身が違います: %q", got)
	}

	// 同じ名前をもう一度送っても上書きしない。
	again := decodeUploaded(t, do(s.handleUpload, uploadRequest(t, map[string][]byte{"shot 1.png": data})))
	if again[0].Path == f.Path {
		t.Fatalf("同じパスに保存しました: %s", f.Path)
	}

	// 保存したものは名前で取れる。
	r := httptest.NewRequest("GET", "/api/upload/"+f.Name, nil)
	r.SetPathValue("name", f.Name)
	if w := do(s.handleUploadFile, r); w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), data) {
		t.Fatalf("取れません: code=%d", w.Code)
	}
}

func TestUploadRejectsBadRequests(t *testing.T) {
	s := newTestServer(t)
	if w := do(s.handleUpload, uploadRequest(t, nil)); w.Code != http.StatusBadRequest {
		t.Errorf("ファイルなし: code=%d", w.Code)
	}
	for _, name := range []string{"..", "../x", ".hidden", "a/b", "a..b"} {
		r := httptest.NewRequest("GET", "/api/upload/x", nil)
		r.SetPathValue("name", name)
		if w := do(s.handleUploadFile, r); w.Code != http.StatusBadRequest {
			t.Errorf("%q: code=%d", name, w.Code)
		}
	}
}

func TestUploadConvertsOtherImagesToJPEG(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg がありません")
	}
	s := newTestServer(t)
	bmp := filepath.Join(t.TempDir(), "in.bmp")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=red:s=8x8", "-frames:v", "1", bmp).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	data, _ := os.ReadFile(bmp)
	files := decodeUploaded(t, do(s.handleUpload, uploadRequest(t, map[string][]byte{"pic.bmp": data})))
	f := files[0]
	if !strings.HasSuffix(f.Path, "-pic.jpg") || f.Name != filepath.Base(f.Path) {
		t.Fatalf("%+v", f)
	}
	if got, _ := os.ReadFile(f.Path); !bytes.HasPrefix(got, []byte{0xff, 0xd8}) {
		t.Fatal("JPEG になっていません")
	}

	// 変換できないものは、元のファイルも残さずに理由を返す。
	w := do(s.handleUpload, uploadRequest(t, map[string][]byte{"broken.heic": []byte("not an image")}))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "broken.heic") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
	left, _ := filepath.Glob(filepath.Join(s.uploadDir, "*broken*"))
	if len(left) != 0 {
		t.Fatalf("残っています: %v", left)
	}
}
