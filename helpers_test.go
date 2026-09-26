package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testToken = "testtoken"

// テスト用のサーバー。設定ファイルは一時ディレクトリの中（最初は無い）を見る。
func newTestServer(t *testing.T) *server {
	t.Helper()
	dir := t.TempDir()
	settings = newConfigStore(filepath.Join(dir, "config.toml"))
	t.Cleanup(func() { settings = nil })
	return &server{token: testToken, defaultSession: "t1", uploadDir: filepath.Join(dir, "uploads")}
}

var configWrites int

// 設定ファイルを書く。更新時刻で読み直すので、前の書き込みと時刻が重ならないようにずらす。
func writeConfig(t *testing.T, body string) {
	t.Helper()
	if err := os.WriteFile(settings.path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	configWrites++
	future := time.Now().Add(time.Duration(configWrites) * time.Second)
	if err := os.Chtimes(settings.path, future, future); err != nil {
		t.Fatal(err)
	}
}

// ふだん使っている tmux には触れないよう、このテストだけの tmux サーバーを使う
// （TMUX_TMPDIR でソケットの場所を分け、設定ファイルは読まず、シェルは /bin/sh）。
// session のセッションを 100x30 で作っておく。
func isolateTmux(t *testing.T, session string) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux がありません")
	}
	dir, err := os.MkdirTemp("", "palmterm-tmux-") // ソケットのパスは短くないといけない
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMUX_TMPDIR", dir)
	t.Setenv("TMUX", "")
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("ENV", "")
	t.Setenv("PS1", "$ ")
	if session != "" {
		runTmux(t, "-f", "/dev/null", "new-session", "-d", "-s", session, "-x", "100", "-y", "30")
	}
	t.Cleanup(func() {
		exec.Command("tmux", "kill-server").Run()
		os.RemoveAll(dir)
	})
}

func runTmux(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("tmux", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("tmux %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimRight(string(out), "\n")
}

// cond が満たされるまで待つ（tmux の中のシェルの反応を待つ用）。
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("待っても %s になりませんでした", what)
}

func do(h http.HandlerFunc, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h(w, r)
	return w
}
