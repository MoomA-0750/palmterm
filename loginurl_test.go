package main

import (
	"bytes"
	"os"
	"path/filepath"
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

// サーバーが書いた URL の控えを読み、Tailscale の URL を先頭に足して並べ、先頭の URL の QR を出す。
func TestRunURLPrintsURLsAndQR(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	if err := writeLoginURLs([]string{"http://127.0.0.1:7681/auth?token=tok", "https://192.0.2.10:7682/auth?token=tok"}); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(filepath.Join(dir, "palmterm", loginURLsFile)); info.Mode().Perm() != 0o600 {
		t.Fatalf("権限: %v", info.Mode())
	}
	old := tailscaleServeStatus
	tailscaleServeStatus = func() ([]byte, error) { return []byte(serveStatusJSON), nil }
	t.Cleanup(func() { tailscaleServeStatus = old })

	var out bytes.Buffer
	if code := runURL(&out, nil); code != 0 {
		t.Fatalf("code=%d\n%s", code, out.String())
	}
	s := out.String()
	first := strings.Index(s, "https://box.example.ts.net/auth?token=tok")
	if first < 0 || first > strings.Index(s, "http://127.0.0.1:7681") || !strings.Contains(s, "https://192.0.2.10:7682") || !strings.Contains(s, "█") {
		t.Fatalf("%s", s)
	}

	out.Reset()
	if code := runURL(&out, []string{"-no-qr"}); code != 0 || strings.Contains(out.String(), "█") {
		t.Fatalf("-no-qr: %s", out.String())
	}
}

// palmterm が動いていなければ（控えが無ければ）、そう知らせて 1 を返す。
func TestRunURLWithoutServer(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	var out bytes.Buffer
	if code := runURL(&out, nil); code != 1 {
		t.Fatalf("code=%d %s", code, out.String())
	}
}
