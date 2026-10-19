package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

// 受けた知らせを覚えておく画面（WebSocket の代わり）。
type notifyRecorder struct {
	mu   sync.Mutex
	msgs []map[string]any
}

func (r *notifyRecorder) client() *termClient {
	return &termClient{send: func(_ context.Context, msg []byte) error {
		var m map[string]any
		json.Unmarshal(msg, &m)
		r.mu.Lock()
		defer r.mu.Unlock()
		r.msgs = append(r.msgs, m)
		return nil
	}}
}

func (r *notifyRecorder) last(t *testing.T) map[string]any {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.msgs) == 0 {
		t.Fatal("知らせが届いていません")
	}
	return r.msgs[len(r.msgs)-1]
}

// pane の中で打ったときの環境にする（tmux はペインのシェルに TMUX と TMUX_PANE を渡す）。
func insideTmux(t *testing.T, pane string) {
	t.Helper()
	t.Setenv("TMUX", runTmux(t, "display-message", "-p", "#{socket_path},#{pid},0"))
	t.Setenv("TMUX_PANE", pane)
}

// tmux の中で palmterm notify を打つと、つながっているすべての画面に、文と打ったウィンドウの番号・名前が届く。
// 文を省くと文は空で届く（画面が自分の言語で「終わりました」を出す）。標準エラーには何も出さず 0 で終わる。
func TestNotifyReachesEveryPageWithTheWindow(t *testing.T) {
	isolateTmux(t, "t1")
	isolateRuntimeDir(t)
	s := newTestServer(t)
	s.setupNotifySocket()
	runTmux(t, "new-window", "-t", "t1", "-n", "build")
	pane := runTmux(t, "display-message", "-p", "-t", "t1:1", "#{pane_id}")
	insideTmux(t, pane)

	var phone, pc notifyRecorder
	s.clients.add(phone.client())
	s.clients.add(pc.client())

	var errOut bytes.Buffer
	if code := runNotify(&errOut, []string{"ビルドが", "終わった\n"}); code != 0 || errOut.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
	for _, r := range []*notifyRecorder{&phone, &pc} {
		m := r.last(t)
		if m["type"] != "notify" || m["text"] != "ビルドが 終わった" || m["window"] != float64(1) || m["windowName"] != "build" {
			t.Fatalf("知らせ: %v", m)
		}
	}

	if code := runNotify(&errOut, nil); code != 0 || errOut.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
	if m := phone.last(t); m["text"] != "" || m["window"] != float64(1) {
		t.Fatalf("文を省いたときの知らせ: %v", m)
	}
}

// tmux の外で打ったときと、palmterm がつないでいないセッションで打ったときは、ウィンドウを添えない
// （押しても切り替えられないので）。
func TestNotifyWithoutWindow(t *testing.T) {
	isolateTmux(t, "t1")
	isolateRuntimeDir(t)
	s := newTestServer(t)
	s.setupNotifySocket()
	var phone notifyRecorder
	s.clients.add(phone.client())

	t.Setenv("TMUX_PANE", "")
	var errOut bytes.Buffer
	runNotify(&errOut, []string{"outside"})
	if m := phone.last(t); m["text"] != "outside" || m["window"] != nil || m["windowName"] != nil {
		t.Fatalf("tmux の外: %v", m)
	}

	runTmux(t, "new-session", "-d", "-s", "other")
	insideTmux(t, runTmux(t, "display-message", "-p", "-t", "other", "#{pane_id}"))
	runNotify(&errOut, []string{"other"})
	if m := phone.last(t); m["text"] != "other" || m["window"] != nil {
		t.Fatalf("ほかのセッション: %v", m)
	}
}

// 画面が1つもつながっていない・palmterm が動いていないときは、標準エラーにそう出して 0 で終わる。
func TestNotifyWithoutPagesOrServer(t *testing.T) {
	isolateTmux(t, "")
	isolateRuntimeDir(t)
	t.Setenv("TMUX_PANE", "")

	var errOut bytes.Buffer
	if code := runNotify(&errOut, []string{"x"}); code != 0 || !strings.Contains(errOut.String(), "palmterm is not running") {
		t.Fatalf("サーバーなし: code=%d stderr=%q", code, errOut.String())
	}

	s := newTestServer(t)
	s.setupNotifySocket()
	errOut.Reset()
	if code := runNotify(&errOut, []string{"x"}); code != 0 || !strings.Contains(errOut.String(), "no palmterm page is open") {
		t.Fatalf("画面なし: code=%d stderr=%q", code, errOut.String())
	}
}

func TestCleanNotifyText(t *testing.T) {
	for in, want := range map[string]string{
		"  done\n":                            "done",
		"a\tb\x1b[31mc\x07":                   "a b[31mc",
		strings.Repeat("あ", maxNotifyRunes+5): strings.Repeat("あ", maxNotifyRunes),
	} {
		if got := cleanNotifyText(in); got != want {
			t.Errorf("%q → %q", in, got)
		}
	}
}
