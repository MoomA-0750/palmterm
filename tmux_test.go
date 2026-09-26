package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func windowsOf(t *testing.T, w *httptest.ResponseRecorder) []tmuxWindow {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
	var resp struct{ Windows []tmuxWindow }
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp.Windows
}

func tmuxAction(t *testing.T, s *server, body string) *httptest.ResponseRecorder {
	t.Helper()
	return do(s.handleTmuxAction, httptest.NewRequest("POST", "/api/tmux", strings.NewReader(body)))
}

func activeWindow(ws []tmuxWindow) tmuxWindow {
	for _, w := range ws {
		if w.Active {
			return w
		}
	}
	return tmuxWindow{Index: -1}
}

func TestTmuxWindowsAndActions(t *testing.T) {
	isolateTmux(t, "t1")
	s := newTestServer(t)

	ws := windowsOf(t, do(s.handleWindows, httptest.NewRequest("GET", "/api/windows", nil)))
	if len(ws) != 1 || !ws[0].Active || ws[0].Index != 0 || ws[0].Panes != 1 {
		t.Fatalf("最初: %+v", ws)
	}

	ws = windowsOf(t, tmuxAction(t, s, `{"action":"new-window"}`))
	if len(ws) != 2 || activeWindow(ws).Index != 1 {
		t.Fatalf("new-window: %+v", ws)
	}
	ws = windowsOf(t, tmuxAction(t, s, `{"action":"select-window","window":0}`))
	if activeWindow(ws).Index != 0 {
		t.Fatalf("select-window: %+v", ws)
	}

	ws = windowsOf(t, tmuxAction(t, s, `{"action":"rename-window","window":1,"name":" 作業\u0007 "}`))
	if ws[1].Name != "作業" {
		t.Fatalf("rename-window（空白と制御文字は落とす）: %+v", ws)
	}
	if got := runTmux(t, "show-options", "-wv", "-t", "t1:1", "automatic-rename"); got != "off" {
		t.Fatalf("名前を付けたら automatic-rename は off のはず: %q", got)
	}
	windowsOf(t, tmuxAction(t, s, `{"action":"rename-window","window":1,"name":""}`))
	if got := runTmux(t, "show-options", "-wv", "-t", "t1:1", "automatic-rename"); got != "on" {
		t.Fatalf("空の名前で automatic-rename に戻るはず: %q", got)
	}

	ws = windowsOf(t, tmuxAction(t, s, `{"action":"split-h"}`))
	if activeWindow(ws).Panes != 2 {
		t.Fatalf("split-h: %+v", ws)
	}
	ws = windowsOf(t, tmuxAction(t, s, `{"action":"split-v"}`))
	if activeWindow(ws).Panes != 3 {
		t.Fatalf("split-v: %+v", ws)
	}
	ws = windowsOf(t, tmuxAction(t, s, `{"action":"zoom"}`))
	if !activeWindow(ws).Zoomed {
		t.Fatalf("zoom: %+v", ws)
	}
	ws = windowsOf(t, tmuxAction(t, s, `{"action":"zoom"}`))
	if activeWindow(ws).Zoomed {
		t.Fatalf("zoom をもう一度: %+v", ws)
	}
	before := runTmux(t, "display-message", "-p", "-t", "t1", "#{pane_id}")
	windowsOf(t, tmuxAction(t, s, `{"action":"pane-next"}`))
	if after := runTmux(t, "display-message", "-p", "-t", "t1", "#{pane_id}"); after == before {
		t.Fatal("pane-next でペインが変わりません")
	}
	for _, a := range []string{"pane-left", "pane-right", "pane-up", "pane-down"} {
		windowsOf(t, tmuxAction(t, s, `{"action":"`+a+`"}`))
	}
	ws = windowsOf(t, tmuxAction(t, s, `{"action":"kill-pane"}`))
	if activeWindow(ws).Panes != 2 {
		t.Fatalf("kill-pane: %+v", ws)
	}
	windowsOf(t, tmuxAction(t, s, `{"action":"copy-mode"}`))
	if got := runTmux(t, "display-message", "-p", "-t", "t1", "#{pane_in_mode}"); got != "1" {
		t.Fatalf("copy-mode: pane_in_mode=%q", got)
	}
	ws = windowsOf(t, tmuxAction(t, s, `{"action":"kill-window","window":0}`))
	if len(ws) != 1 || ws[0].Index != 1 {
		t.Fatalf("kill-window: %+v", ws)
	}

	if w := tmuxAction(t, s, `{"action":"run-shell"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("知らない操作: code=%d", w.Code)
	}
	if w := tmuxAction(t, s, `{"action":"select-window","window":42}`); w.Code != http.StatusConflict {
		t.Fatalf("無いウィンドウ: code=%d", w.Code)
	}
}

func TestSelectPaneAtPicksThePaneUnderTheTap(t *testing.T) {
	isolateTmux(t, "t1")
	s := newTestServer(t)
	runTmux(t, "split-window", "-h", "-t", "t1") // 左 0〜49、境目 50、右 51〜99
	left := runTmux(t, "display-message", "-p", "-t", "t1:0.0", "#{pane_id}")
	paneAt := func(col, row int) string {
		t.Helper()
		windowsOf(t, tmuxAction(t, s, `{"action":"select-pane-at","col":`+strconv.Itoa(col)+`,"row":`+strconv.Itoa(row)+`}`))
		return runTmux(t, "display-message", "-p", "-t", "t1", "#{pane_id}")
	}
	if got := paneAt(5, 3); got != left {
		t.Fatalf("左をタップ: %s", got)
	}
	right := paneAt(80, 3)
	if right == left {
		t.Fatal("右をタップしても左のままです")
	}
	if got := paneAt(50, 3); got != right {
		t.Fatalf("境目のタップで変わりました: %s", got)
	}
	// 拡大中は何もしない。
	runTmux(t, "resize-pane", "-Z", "-t", "t1")
	if got := paneAt(5, 3); got != right {
		t.Fatalf("拡大中なのに変わりました: %s", got)
	}
}

func TestHistoryAndPaneState(t *testing.T) {
	isolateTmux(t, "t1")
	s := newTestServer(t)
	runTmux(t, "send-keys", "-t", "t1", "echo palm$((40+2))", "Enter")
	waitFor(t, "palm42 が出る", func() bool {
		return strings.Contains(runTmux(t, "capture-pane", "-p", "-t", "t1"), "palm42\n")
	})

	w := do(s.handleHistory, httptest.NewRequest("GET", "/api/history?lines=100", nil))
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, "\npalm42\n") || strings.Contains(body, " \n") || !strings.HasSuffix(body, "\n") || strings.HasSuffix(body, "\n\n") {
		t.Fatalf("code=%d body=%q", w.Code, body)
	}

	var st paneState
	w = do(s.handlePane, httptest.NewRequest("GET", "/api/pane", nil))
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.AltScreen || st.Mouse || st.InMode {
		t.Fatalf("シェルのままのはず: %+v", st)
	}
	runTmux(t, "copy-mode", "-t", "t1")
	do(s.handlePane, httptest.NewRequest("GET", "/api/pane", nil))
	w = do(s.handlePane, httptest.NewRequest("GET", "/api/pane", nil))
	json.Unmarshal(w.Body.Bytes(), &st)
	if !st.InMode {
		t.Fatalf("コピーモードのはず: %+v", st)
	}
}

func TestTrimLines(t *testing.T) {
	if got := trimLines("a  \nb\n\n\n"); got != "a\nb\n" {
		t.Fatalf("%q", got)
	}
}

func TestScrollPane(t *testing.T) {
	isolateTmux(t, "t1")
	for i := 0; i < 60; i++ {
		runTmux(t, "send-keys", "-t", "t1", "echo line"+strconv.Itoa(i), "Enter")
	}
	waitFor(t, "line59 が出る", func() bool {
		return strings.Contains(runTmux(t, "capture-pane", "-p", "-t", "t1"), "line59\n")
	})
	mode := func() string {
		return runTmux(t, "display-message", "-p", "-t", "t1", "#{pane_in_mode} #{scroll_position}")
	}

	scrollPane("t1", -5)
	if got := mode(); got != "1 5" {
		t.Fatalf("上へ5行: %q", got)
	}
	scrollPane("t1", 2)
	if got := mode(); got != "1 3" {
		t.Fatalf("下へ2行: %q", got)
	}
	scrollPane("t1", 10) // 一番下まで戻したらコピーモードを抜ける
	if got := mode(); !strings.HasPrefix(got, "0") {
		t.Fatalf("抜けていません: %q", got)
	}
}

// WebSocket でつないだ端末：打った文字が tmux の中のシェルに届き、出力が返り、サイズ変更が効く。
func TestTerminalRelay(t *testing.T) {
	isolateTmux(t, "")
	s := newTestServer(t)
	srv := httptest.NewServer(http.HandlerFunc(s.handleTerminal))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/?cols=90&rows=20", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()

	var out bytes.Buffer
	readUntil := func(want string) {
		t.Helper()
		for !strings.Contains(out.String(), want) {
			_, data, err := c.Read(ctx)
			if err != nil {
				t.Fatalf("%q が来る前に: %v\n%s", want, err, out.String())
			}
			out.Write(data)
		}
	}
	waitFor(t, "セッションができる", func() bool {
		out, _ := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
		return string(out) == "t1\n"
	})
	if got := runTmux(t, "display-message", "-p", "-t", "t1", "#{window_width}x#{window_height}"); got != "90x19" {
		t.Fatalf("最初のサイズ（ステータス行の分 1 行少ない）: %s", got)
	}

	if err := c.Write(ctx, websocket.MessageBinary, []byte("echo relay$((6*7))\r")); err != nil {
		t.Fatal(err)
	}
	readUntil("relay42")

	if err := c.Write(ctx, websocket.MessageText, []byte(`{"type":"resize","cols":70,"rows":25}`)); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "70x24 になる", func() bool {
		return runTmux(t, "display-message", "-p", "-t", "t1", "#{window_width}x#{window_height}") == "70x24"
	})

	if err := c.Write(ctx, websocket.MessageText, []byte(`{"type":"scroll","lines":-3}`)); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "コピーモードに入る", func() bool {
		return runTmux(t, "display-message", "-p", "-t", "t1", "#{pane_in_mode}") == "1"
	})
}

// tmux に渡す環境：入れ子の扱いにならないよう TMUX を外し、端末の種類を決め、
// 起動元の端末の幅（COLUMNS・LINES）は渡さない（渡すと tmux の中のプログラムが画面の幅を取り違える）。
func TestTerminalEnv(t *testing.T) {
	t.Setenv("TMUX", "/tmp/x,1,0")
	t.Setenv("TMUX_PANE", "%1")
	t.Setenv("TERM", "dumb")
	t.Setenv("COLUMNS", "80")
	t.Setenv("LINES", "24")
	t.Setenv("PALMTERM_TEST_KEEP", "yes")
	env := strings.Join(terminalEnv(), "\n") + "\n"
	for _, gone := range []string{"TMUX=", "TMUX_PANE=", "TERM=dumb", "COLUMNS=", "LINES="} {
		if strings.Contains(env, "\n"+gone) || strings.HasPrefix(env, gone) {
			t.Errorf("%s が残っています", gone)
		}
	}
	for _, want := range []string{"TERM=xterm-256color\n", "COLORTERM=truecolor\n", "PALMTERM_TEST_KEEP=yes\n"} {
		if !strings.Contains(env, want) {
			t.Errorf("%s がありません", strings.TrimSpace(want))
		}
	}
}
