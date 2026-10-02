package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/creack/pty"
)

// 画面からの制御メッセージ（テキストのフレーム）。キー入力はバイナリのフレームで生のまま届く。
type controlMessage struct {
	Type  string `json:"type"`
	Cols  uint16 `json:"cols"`
	Rows  uint16 `json:"rows"`
	Lines int    `json:"lines"` // scroll: 負なら古い方（上）へ、正なら新しい方（下）へ
}

// 接続ごとに tmux のクライアントを1つ起動する。切断してもセッションは tmux に残る。
func (s *server) handleTerminal(w http.ResponseWriter, r *http.Request) {
	s.terminals.Add(1)
	defer s.terminals.Done()
	session := s.session
	size := &pty.Winsize{Cols: queryUint16(r, "cols", 80), Rows: queryUint16(r, "rows", 24)}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: s.origins})
	if err != nil {
		log.Printf(tr("WebSocket を開けませんでした: %v", "Could not open the WebSocket: %v"), err)
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(8 << 20) // 大きな貼り付けも1回で受ける

	// 前の palmterm などが tmux のサーバーを起動していて、サーバーの環境にトークンが残っていたら消す
	// （以後に作るウィンドウのシェルに渡さない）。サーバーがまだ無ければ何もしない。
	exec.Command("tmux", "set-environment", "-g", "-u", "PALMTERM_TOKEN").Run()

	startTmuxOutsideService(session, s.tmuxEnv())
	cmd := exec.Command("tmux", "new-session", "-A", "-s", session)
	cmd.Env = s.tmuxEnv()
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	ptmx, err := pty.StartWithSize(cmd, size)
	if err != nil {
		log.Printf(tr("tmux を起動できませんでした: %v", "Could not start tmux: %v"), err)
		conn.Close(websocket.StatusInternalError, tr("tmux を起動できませんでした", "Could not start tmux"))
		return
	}
	defer func() {
		// 先に tmux のクライアントに切断を知らせてから閉じる。閉じれば読み込みが止まるのは Linux の実装に
		// 頼っていて（ほかの OS では Close が読み込みの終わりを待つことがある）、そうなると SIGHUP が送られない。
		cmd.Process.Signal(syscall.SIGHUP)
		ptmx.Close()
		cmd.Wait()
	}()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// この画面を、ブラウザで開く URL を届ける先として登録する。tmux のサーバーの環境にも BROWSER を入れる
	// （以後に作るウィンドウ・ペインのプログラムが使う）。
	tc := &termClient{
		send:      func(ctx context.Context, msg []byte) error { return conn.Write(ctx, websocket.MessageText, msg) },
		lastInput: time.Now(),
	}
	s.clients.add(tc)
	defer s.clients.remove(tc)
	if s.openCommand != "" {
		exec.Command("tmux", "set-environment", "-g", "BROWSER", s.openCommand).Run()
	}

	go func() {
		defer cancel()
		buf := make([]byte, 32*1024)
		started := false
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				// tmux のサーバーが動き出してから（最初の出力が来てから）設定する。
				if !started && s.captureClipboard {
					captureClipboard()
				}
				started = true
				if werr := conn.Write(ctx, websocket.MessageBinary, buf[:n]); werr != nil {
					return
				}
				// tmux がバッファにコピーした（OSC 52 を送ってきた）ら、画面に知らせる。画面は一覧を読み直す。
				// 読み込みの切れ目で OSC 52 が分かれたときは知らせ損なうが、一覧には入っている。
				if bytes.Contains(buf[:n], osc52) {
					conn.Write(ctx, websocket.MessageText, []byte(`{"type":"clipboard"}`))
				}
			}
			if err != nil {
				conn.Close(websocket.StatusNormalClosure, tr("tmux が終了しました", "tmux exited"))
				return
			}
		}
	}()

	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if typ == websocket.MessageBinary {
			s.clients.touch(tc)
			if _, err := ptmx.Write(data); err != nil {
				return
			}
			continue
		}
		var msg controlMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch {
		case msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0:
			pty.Setsize(ptmx, &pty.Winsize{Cols: msg.Cols, Rows: msg.Rows})
		case msg.Type == "scroll" && msg.Lines != 0:
			scrollPane(session, msg.Lines)
		}
	}
}

// サーバーを tmux の中から起動しても入れ子の扱いにならないよう、TMUX を外す。
// 起動元の端末の幅（COLUMNS・LINES）も渡さない。tmux の中のプログラムが画面の幅を取り違えるため。
// ログイン用のトークン（PALMTERM_TOKEN）も、tmux の中のシェルの環境に残さない。
func terminalEnv() []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch name {
		case "TMUX", "TMUX_PANE", "TERM", "COLORTERM", "COLUMNS", "LINES", "PALMTERM_TOKEN":
			continue
		}
		env = append(env, kv)
	}
	return append(env, "TERM=xterm-256color", "COLORTERM=truecolor")
}

func queryUint16(r *http.Request, key string, def uint16) uint16 {
	v, err := strconv.ParseUint(r.URL.Query().Get(key), 10, 16)
	if err != nil || v == 0 {
		return def
	}
	return uint16(v)
}

// tmux のコピーモードでペインを行単位でスクロールする（スワイプ用）。
// 上へはコピーモードに入ってから動かす。-e を付けるので、一番下まで戻すとコピーモードを抜ける。
// 下へはコピーモード中だけ動く（入っていなければ tmux がエラーを返すので無視する）。
func scrollPane(session string, lines int) {
	n := strconv.Itoa(min(abs(lines), 500))
	var args []string
	if lines < 0 {
		args = []string{"copy-mode", "-e", "-t", session, ";", "send-keys", "-X", "-N", n, "-t", session, "scroll-up"}
	} else {
		args = []string{"send-keys", "-X", "-N", n, "-t", session, "scroll-down"}
	}
	exec.Command("tmux", args...).Run()
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// systemd のサービスとして動いているときは、tmux のサーバーをサービスの外（別の scope）で起動する。
// Fedora などの tmux は、ペインのシェルを tmux のサーバーが属する単位に結びついた scope に入れるので、
// サービスの中でサーバーを起動すると、サービスを止めたり再起動したりしたときにペインが全部止められ、
// セッションが消えてしまう。セッションがもうあるとき、systemd の外で動いているときは何もしない。
func startTmuxOutsideService(session string, env []string) {
	if os.Getenv("INVOCATION_ID") == "" {
		return
	}
	if exec.Command("tmux", "has-session", "-t", "="+session).Run() == nil {
		return
	}
	cmd := exec.Command("systemd-run", tmuxScopeArgs(session)...)
	cmd.Env = env
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		// 使えなければ、今までどおりサービスの中で起動する（new-session -A が作る）。
		log.Printf(tr("tmux をサービスの外で起動できませんでした: %v %s", "Could not start tmux outside the service: %v %s"), err, out)
	}
}

func tmuxScopeArgs(session string) []string {
	return []string{"--user", "--scope", "--quiet", "--collect", "--description=tmux server started by palmterm",
		"tmux", "new-session", "-d", "-s", session}
}
