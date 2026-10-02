package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func readClipboard(t *testing.T, w *httptest.ResponseRecorder) []clipItem {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var got struct{ Items []clipItem }
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got.Items
}

func clipTexts(items []clipItem) []string {
	var texts []string
	for _, it := range items {
		texts = append(texts, it.Text)
	}
	return texts
}

// tmux のバッファを新しい順に返し、同じ中身はまとめる。消すとまとめたバッファが全部消える。
func TestClipboardListsAndDeletesBuffers(t *testing.T) {
	isolateTmux(t, "")
	s := newTestServer(t)
	get := func() []clipItem {
		return readClipboard(t, do(s.handleClipboard, httptest.NewRequest("GET", "/api/clipboard", nil)))
	}
	// tmux のサーバーが無ければ空。
	if items := get(); len(items) != 0 {
		t.Fatalf("%v", items)
	}

	runTmux(t, "-f", "/dev/null", "new-session", "-d", "-s", "t1")
	runTmux(t, "set-buffer", "same")
	runTmux(t, "set-buffer", "first\nline 2")
	runTmux(t, "set-buffer", "same")
	runTmux(t, "set-buffer", "日本語")
	items := get()
	if got := strings.Join(clipTexts(items), "|"); got != "日本語|same|first\nline 2" {
		t.Fatalf("%q", got)
	}
	if len(items[1].Names) != 2 || items[1].Created == 0 {
		t.Fatalf("%+v", items[1])
	}

	body, _ := json.Marshal(map[string][]string{"names": items[1].Names})
	w := do(s.handleClipboardDelete, httptest.NewRequest("POST", "/api/clipboard/delete", strings.NewReader(string(body))))
	if got := strings.Join(clipTexts(readClipboard(t, w)), "|"); got != "日本語|first\nline 2" {
		t.Fatalf("%q", got)
	}
	if w := do(s.handleClipboardDelete, httptest.NewRequest("POST", "/api/clipboard/delete", strings.NewReader(`{}`))); w.Code != http.StatusBadRequest {
		t.Fatalf("%d", w.Code)
	}
}

// 端末をつなぐと tmux の set-clipboard を on にし、中のプログラムが OSC 52 でコピーした文字がバッファに入る。
// tmux がバッファにコピーしたら、画面に {"type":"clipboard"} を知らせる。
func TestTerminalCapturesOSC52(t *testing.T) {
	isolateTmux(t, "")
	s := newTestServer(t)
	s.captureClipboard = true
	srv := httptest.NewServer(http.HandlerFunc(s.handleTerminal))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/?cols=90&rows=20", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()

	waitFor(t, "set-clipboard が on になる", func() bool {
		out, _ := exec.Command("tmux", "show-options", "-sv", "set-clipboard").Output()
		return string(out) == "on\n"
	})
	// base64 で "from app"
	if err := c.Write(ctx, websocket.MessageBinary, []byte(`printf '\033]52;c;ZnJvbSBhcHA=\a'`+"\r")); err != nil {
		t.Fatal(err)
	}
	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("知らせが来ません: %v", err)
		}
		if typ == websocket.MessageText && string(data) == `{"type":"clipboard"}` {
			break
		}
	}
	items, err := listClipboard()
	if err != nil || len(items) != 1 || items[0].Text != "from app" {
		t.Fatalf("%+v %v", items, err)
	}
}
