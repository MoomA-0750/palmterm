package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/coder/websocket"
	"github.com/creack/pty"
)

// 画面からの制御メッセージ（テキストのフレーム）。キー入力はバイナリのフレームで生のまま届く。
type controlMessage struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// 接続ごとに tmux のクライアントを1つ起動する。切断してもセッションは tmux に残る。
func (s *server) handleTerminal(w http.ResponseWriter, r *http.Request) {
	session, err := s.sessionFrom(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	size := &pty.Winsize{Cols: queryUint16(r, "cols", 80), Rows: queryUint16(r, "rows", 24)}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: s.origins})
	if err != nil {
		log.Printf("WebSocket を開けませんでした: %v", err)
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(8 << 20) // 大きな貼り付けも1回で受ける

	cmd := exec.Command("tmux", "new-session", "-A", "-s", session)
	cmd.Env = terminalEnv()
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	ptmx, err := pty.StartWithSize(cmd, size)
	if err != nil {
		log.Printf("tmux を起動できませんでした: %v", err)
		conn.Close(websocket.StatusInternalError, "tmux を起動できませんでした")
		return
	}
	defer func() {
		ptmx.Close()
		cmd.Process.Signal(syscall.SIGHUP)
		cmd.Wait()
	}()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		defer cancel()
		buf := make([]byte, 32*1024)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				if werr := conn.Write(ctx, websocket.MessageBinary, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				conn.Close(websocket.StatusNormalClosure, "tmux が終了しました")
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
			if _, err := ptmx.Write(data); err != nil {
				return
			}
			continue
		}
		var msg controlMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0 {
			pty.Setsize(ptmx, &pty.Winsize{Cols: msg.Cols, Rows: msg.Rows})
		}
	}
}

// サーバーを tmux の中から起動しても入れ子の扱いにならないよう、TMUX を外す。
func terminalEnv() []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "TMUX=") || strings.HasPrefix(kv, "TMUX_PANE=") ||
			strings.HasPrefix(kv, "TERM=") || strings.HasPrefix(kv, "COLORTERM=") {
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
