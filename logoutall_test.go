package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// 実行時のフォルダを、このテストだけの短いパスにする（ソケットのパスは長くできない）。
func isolateRuntimeDir(t *testing.T) string {
	t.Helper()
	base, err := os.MkdirTemp("", "palmterm-run-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(base) })
	t.Setenv("XDG_RUNTIME_DIR", base)
	return filepath.Join(base, "palmterm")
}

// palmterm logout-all：記録した全端末が取り消され、開いている接続は取り消しと同じ切れ方（1008。画面は
// ログインし直す案内を出す）ですぐ切れる。取り消した台数を出して 0 で終わる。実行時のフォルダは 0700 に直す。
func TestLogoutAllRevokesEveryDeviceAndClosesConnections(t *testing.T) {
	isolateTmux(t, "t1")
	dir := isolateRuntimeDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.Chmod(dir, 0o755) // 緩い権限で残っていても直す
	s := newTestServer(t)
	s.setupControlSocket()
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("実行時のフォルダの権限: %v %v", info.Mode(), err)
	}
	if info, err := os.Stat(filepath.Join(dir, controlSocketName)); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("ソケット: %v", err)
	}

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
	conns := []*websocket.Conn{dial(phone), dial(pc)}
	for _, c := range conns {
		if _, _, err := c.Read(ctx); err != nil { // つながって、端末の出力が来るまで待つ
			t.Fatal(err)
		}
	}

	var out, errOut bytes.Buffer
	if code := runLogoutAll(&out, &errOut, nil); code != 0 {
		t.Fatalf("code=%d %s %s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "2") {
		t.Fatalf("台数が出ていません: %q", out.String())
	}
	for i, c := range conns {
		for {
			if _, _, err := c.Read(ctx); err != nil {
				if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
					t.Fatalf("接続 %d の切れ方: %v", i, err)
				}
				break
			}
		}
	}
	for _, c := range []*http.Cookie{phone, pc} {
		if w := authed(s, "GET", "/api/devices", "", c); w.Code != http.StatusUnauthorized {
			t.Fatalf("取り消した端末が通りました: code=%d", w.Code)
		}
	}
	// 再起動しても戻らない（記録のファイルからも消えている）。
	if n := len(newDeviceStore(s.devices.path, testToken).list()); n != 0 {
		t.Fatalf("記録に %d 台残っています", n)
	}
	// 端末が無くても 0 台として終わる。
	out.Reset()
	if code := runLogoutAll(&out, &errOut, nil); code != 0 || !strings.Contains(out.String(), "0") {
		t.Fatalf("2回目: code=%d %q", code, out.String())
	}
}

// palmterm が動いていなければ（ソケットが無い・前の残りで誰も聞いていない）、そう知らせて 0 以外で終わる。
func TestLogoutAllWithoutServer(t *testing.T) {
	dir := isolateRuntimeDir(t)
	check := func(when string) {
		var out, errOut bytes.Buffer
		if code := runLogoutAll(&out, &errOut, nil); code == 0 || !strings.Contains(errOut.String(), "not running") {
			t.Fatalf("%s: code=%d out=%q err=%q", when, code, out.String(), errOut.String())
		}
	}
	check("フォルダも無い")

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", filepath.Join(dir, controlSocketName))
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	ln.Close() // ソケットのファイルだけ残る
	check("前の残りのソケット")
}
