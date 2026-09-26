package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestValidOpenURL(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://github.com/login/device": true,
		"http://localhost:8080/x":         true,
		"javascript:alert(1)":             false,
		"file:///etc/passwd":              false,
		"https://":                        false,
		"":                                false,
		"https://x/" + strings.Repeat("a", maxOpenURLBytes): false,
	} {
		if validOpenURL(raw) != want {
			t.Errorf("%q", raw)
		}
	}
}

// ソケットに来た URL を、最後に入力があった画面に届ける。画面が無ければ none、使えない URL は届けない。
func TestOpenSocketRelaysToLatestClient(t *testing.T) {
	s := newTestServer(t)
	sock := filepath.Join(t.TempDir(), "open.sock")
	if err := s.serveOpenSocket(sock); err != nil {
		t.Fatal(err)
	}
	if sendToPalmterm(sock, "https://example.com/") {
		t.Fatal("画面が無いのに ok")
	}

	got := map[string][]string{}
	client := func(name string, last time.Time) *termClient {
		return &termClient{lastInput: last, send: func(_ context.Context, msg []byte) error {
			got[name] = append(got[name], string(msg))
			return nil
		}}
	}
	s.clients.add(client("old", time.Now().Add(-time.Minute)))
	phone := client("phone", time.Now())
	s.clients.add(phone)

	if !sendToPalmterm(sock, "https://github.com/login/device") {
		t.Fatal("届きませんでした")
	}
	if sendToPalmterm(sock, "javascript:alert(1)") {
		t.Fatal("使えない URL を届けました")
	}
	if len(got["old"]) != 0 || len(got["phone"]) != 1 || got["phone"][0] != `{"type":"open","url":"https://github.com/login/device"}` {
		t.Fatalf("%v", got)
	}
	s.clients.remove(phone)
	if !sendToPalmterm(sock, "https://example.com/") || len(got["old"]) != 1 {
		t.Fatalf("残った画面に届いていません: %v", got)
	}
}

// palmterm がいなければ（ソケットが無ければ）、この PC のブラウザで開く。
func TestRunOpenFallsBackToLocalBrowser(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	var opened []string
	old := openLocally
	openLocally = func(raw string) error { opened = append(opened, raw); return nil }
	t.Cleanup(func() { openLocally = old })
	if code := runOpen([]string{"https://example.com/"}); code != 0 || len(opened) != 1 {
		t.Fatalf("code=%d opened=%v", code, opened)
	}
}

// 端末につなぐと、tmux の環境に BROWSER が入り、palmterm-open からの URL がその画面に届く。
func TestTerminalReceivesOpenedURLs(t *testing.T) {
	isolateTmux(t, "t1")
	s := newTestServer(t)
	s.openCommand = "/tmp/palmterm-open-test"
	sock := filepath.Join(t.TempDir(), "open.sock")
	if err := s.serveOpenSocket(sock); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(s.handleTerminal))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/?cols=80&rows=20", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()

	waitFor(t, "BROWSER が入る", func() bool {
		out, _ := exec.Command("tmux", "show-environment", "-g", "BROWSER").Output()
		return string(out) == "BROWSER=/tmp/palmterm-open-test\n"
	})
	if !sendToPalmterm(sock, "https://github.com/login/device") {
		t.Fatal("届きませんでした")
	}
	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if typ != websocket.MessageText {
			continue // 端末の出力
		}
		var msg struct{ Type, URL string }
		json.Unmarshal(data, &msg)
		if msg.Type != "open" || msg.URL != "https://github.com/login/device" {
			t.Fatalf("%s", data)
		}
		return
	}
}
