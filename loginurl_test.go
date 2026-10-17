package main

import (
	"bytes"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"rsc.io/qr"
)

const serveStatusJSON = `{
  "TCP": {"443": {"HTTPS": true}, "8443": {"HTTPS": true}, "10443": {"HTTPS": true}},
  "Web": {
    "box.example.ts.net:443":   {"Handlers": {"/": {"Proxy": "http://127.0.0.1:7681"}}},
    "box.example.ts.net:8443":  {"Handlers": {"/": {"Proxy": "http://localhost:7681"}}},
    "box.example.ts.net:10443": {"Handlers": {"/": {"Proxy": "http://127.0.0.1:8790"}}}
  }
}`

// tailscale serve の設定から、palmterm（listen のポート）に中継している URL だけを取り出す。443 番はポートを省く。
func TestTailscaleLoginURLs(t *testing.T) {
	got := tailscaleLoginURLs([]byte(serveStatusJSON), "127.0.0.1:7681", "tok")
	want := []string{"https://box.example.ts.net/auth?token=tok", "https://box.example.ts.net:8443/auth?token=tok"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("%v", got)
	}
	if got := tailscaleLoginURLs([]byte("not json"), "127.0.0.1:7681", "tok"); len(got) != 0 {
		t.Fatalf("%v", got)
	}
}

// 端末に描く QR は、上下2つのマスを1文字にしたもの。読み戻すと元の QR の白黒と一致する（周りに余白を付ける）。
func TestRenderQRMatchesCode(t *testing.T) {
	text := "https://box.example.ts.net/auth?token=0123456789abcdef"
	code, err := qr.Encode(text, qr.L)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(renderQR(code), "\n"), "\n")
	n := code.Size + 2*qrQuietZone
	if len(lines) != (n+1)/2 {
		t.Fatalf("行数 %d", len(lines))
	}
	// 明るいマス（白）を █ の側にする（暗い背景の端末で、ふつうの QR と同じ見た目になる）
	light := func(x, y int) bool {
		x, y = x-qrQuietZone, y-qrQuietZone
		return x < 0 || y < 0 || x >= code.Size || y >= code.Size || !code.Black(x, y)
	}
	for row, line := range lines {
		cells := []rune(line)
		if len(cells) != n {
			t.Fatalf("%d 行目の幅 %d", row, len(cells))
		}
		for x, c := range cells {
			top, bottom := light(x, 2*row), 2*row+1 >= n || light(x, 2*row+1)
			want := map[[2]bool]rune{{true, true}: '█', {true, false}: '▀', {false, true}: '▄', {false, false}: ' '}[[2]bool{top, bottom}]
			if c != want {
				t.Fatalf("(%d,%d): %q want %q", x, row, c, want)
			}
		}
	}
}

// URL の行から token= の値を取り出す。
func tokenOf(t *testing.T, u string) string {
	t.Helper()
	parsed, err := url.Parse(u)
	if err != nil || parsed.Query().Get("token") == "" {
		t.Fatalf("トークンの無い URL: %q", u)
	}
	return parsed.Query().Get("token")
}

// palmterm url を打つたびに、動いている palmterm が新しいトークンを出し、待ち受けている各アドレスの URL
// （Tailscale の URL を先頭に足す）と、先頭の URL の QR を出す。前の版の URL の控えは消す。
func TestRunURLIssuesANewTokenEachTime(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	stale := filepath.Join(dir, "palmterm", staleLoginURLsFile)
	os.MkdirAll(filepath.Dir(stale), 0o700)
	os.WriteFile(stale, []byte("http://127.0.0.1:7681/auth?token=fixed\n"), 0o600)
	s := newTestServer(t)
	s.loginBases = []string{"http://127.0.0.1:7681", "https://192.0.2.10:7682"}
	s.setupLoginSocket()
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("前の版の控えが残っています: %v", err)
	}
	if info, err := os.Stat(filepath.Join(dir, "palmterm", loginSocketName)); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("ソケット: %v %v", info, err)
	}
	old := tailscaleServeStatus
	tailscaleServeStatus = func() ([]byte, error) { return []byte(serveStatusJSON), nil }
	t.Cleanup(func() { tailscaleServeStatus = old })

	var out bytes.Buffer
	if code := runURL(&out, nil); code != 0 {
		t.Fatalf("code=%d\n%s", code, out.String())
	}
	got := out.String()
	lines := strings.Split(got, "\n")
	if !strings.HasPrefix(lines[0], "https://box.example.ts.net/auth?token=") || !strings.HasPrefix(lines[1], "https://box.example.ts.net:8443/auth?token=") ||
		!strings.HasPrefix(lines[2], "http://127.0.0.1:7681/auth?token=") || !strings.HasPrefix(lines[3], "https://192.0.2.10:7682/auth?token=") || !strings.Contains(got, "█") {
		t.Fatalf("%s", got)
	}
	first := tokenOf(t, lines[0])
	for _, l := range lines[1:4] {
		if tokenOf(t, l) != first {
			t.Fatalf("アドレスごとにトークンが違います: %s", got)
		}
	}

	out.Reset()
	if code := runURL(&out, []string{"-no-qr"}); code != 0 || strings.Contains(out.String(), "█") {
		t.Fatalf("-no-qr: %s", out.String())
	}
	second := tokenOf(t, strings.Split(out.String(), "\n")[0])
	if second == first {
		t.Fatal("打ち直してもトークンが同じです")
	}
	// どちらのトークンでも（まだ使っていなければ）ログインのページが出る
	for _, tok := range []string{first, second} {
		if w := getLoginPage(s, tok, ""); w.Code != http.StatusOK {
			t.Fatalf("%s: code=%d", tok, w.Code)
		}
	}
}

// 起動時の案内（ログ）にも、使い捨てのログイン用の URL を出す（LAN の URL も同じトークン）。
func TestStartupLogShowsOneTimeLoginURL(t *testing.T) {
	s := newTestServer(t)
	s.loginBases = []string{"http://127.0.0.1:7681", "https://192.0.2.10:7682"}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	s.logLoginURLs()
	m := regexp.MustCompile(`http://127\.0\.0\.1:7681/auth\?token=([0-9a-f]+)`).FindStringSubmatch(buf.String())
	if m == nil || !strings.Contains(buf.String(), "https://192.0.2.10:7682/auth?token="+m[1]) || !strings.Contains(buf.String(), "palmterm url") {
		t.Fatalf("%s", buf.String())
	}
	if m[1] == testToken {
		t.Fatal("固定のトークンを出しています")
	}
	if w := postLogin(s, m[1], "", ""); w.Code != http.StatusSeeOther {
		t.Fatalf("案内の URL でログインできません: code=%d", w.Code)
	}
}

// palmterm が動いていなければ（ソケットにつなげなければ）、そう知らせて 1 を返す。
func TestRunURLWithoutServer(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	var out bytes.Buffer
	if code := runURL(&out, nil); code != 1 {
		t.Fatalf("code=%d %s", code, out.String())
	}
}
