package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// palmterm notify：開いている palmterm の画面すべてに、知らせの帯を出させる（ビルドやタスクの終わりなど）。
// tmux の中で打つと、打ったウィンドウの番号と名前も添える（帯を押すとそのウィンドウに切り替わる）。
// 切り替えはウィンドウの ID（@N）で指すので、ウィンドウを閉じて番号がずれても打ったウィンドウに移る。
// サーバーは実行時のフォルダに notify.sock を置き、このコマンドはそこに1行の JSON を渡す。
// 届けられなくても終了コードは 0（Claude Code のフックや `make; palmterm notify` を止めない）。

const (
	notifyCommand    = "notify"
	notifySocketName = "notify.sock"
	maxNotifyRunes   = 1000     // 帯に出す文の長さ（これより長い所は切る）
	maxNotifyBytes   = 16 << 10 // ソケットで受ける1行の長さ（送る側で文を切ってから送るので、これを超えない）
)

// notify.sock でやり取りする知らせ。Window が nil なら、どのウィンドウからかは分からない（tmux の外で打った）。
type notifyRequest struct {
	Text       string `json:"text"`
	Session    string `json:"session,omitempty"`
	Window     *int   `json:"window,omitempty"`
	WindowName string `json:"windowName,omitempty"`
	WindowID   string `json:"windowId,omitempty"` // tmux のウィンドウの ID（@N）。閉じるまで変わらない
}

// tmux のウィンドウの ID（@ と数字）か。
func isWindowID(id string) bool {
	if len(id) < 2 || id[0] != '@' {
		return false
	}
	for _, c := range id[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// 帯に出す文にする：改行やタブは空白に、ほかの制御文字は除き、前後の空白を落として長さを切る。
func cleanNotifyText(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n == maxNotifyRunes {
			break
		}
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			r = ' '
		case unicode.IsControl(r):
			continue
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}

// 知らせをつながっているすべての画面に届け、届いた画面の数を返す。文が空なら、画面が自分の言語で
// 「終わりました」を出す。ほかのセッションのウィンドウは、押しても切り替えられないので添えない。
func (s *server) relayNotify(ctx context.Context, req notifyRequest) int {
	msg := map[string]any{"type": "notify", "text": cleanNotifyText(req.Text)}
	if req.Window != nil && *req.Window >= 0 && (req.Session == "" || req.Session == s.session) {
		msg["window"] = *req.Window
		msg["windowName"] = cleanNotifyText(req.WindowName)
		if isWindowID(req.WindowID) {
			msg["windowId"] = req.WindowID
		}
	}
	b, _ := json.Marshal(msg)
	n := 0
	for _, tc := range s.clients.all() {
		if tc.send(ctx, b) == nil {
			n++
		}
	}
	return n
}

// palmterm notify からの知らせを受けるソケット（本人だけがつなげる）。1行に1つの JSON を受け、
// 届けた画面の数を "ok <数>"、画面が無ければ "none"、読めなければ "bad" で返す。
func (s *server) serveNotifySocket(path string) error {
	os.Remove(path) // 前に動いていたときの残り
	ln, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return err
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.handleNotifyConn(conn)
		}
	}()
	return nil
}

func (s *server) handleNotifyConn(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	line, err := bufio.NewReader(io.LimitReader(conn, maxNotifyBytes)).ReadString('\n')
	if err != nil && line == "" {
		return
	}
	var req notifyRequest
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		fmt.Fprintln(conn, "bad")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if n := s.relayNotify(ctx, req); n > 0 {
		fmt.Fprintf(conn, "ok %d\n", n)
	} else {
		fmt.Fprintln(conn, "none")
	}
}

// 知らせの受け口を用意する。使えなくても palmterm 自体は動かす。
func (s *server) setupNotifySocket() {
	dir, err := ensureRuntimeDir()
	if err == nil {
		err = s.serveNotifySocket(filepath.Join(dir, notifySocketName))
	}
	if err != nil {
		log.Printf(tr("palmterm notify の受け口を用意できませんでした: %v", "Could not set up the socket for palmterm notify: %v"), err)
	}
}

// 打ったペインのセッション・ウィンドウの番号・ID・名前（tmux の中で打ったとき）。分からなければ ok は false。
func notifyOrigin() (session string, window int, id, name string, ok bool) {
	pane := os.Getenv("TMUX_PANE")
	if pane == "" || os.Getenv("TMUX") == "" {
		return "", 0, "", "", false
	}
	out, err := exec.Command("tmux", "display-message", "-p", "-t", pane, "#{session_name}\t#{window_index}\t#{window_id}\t#{window_name}").Output()
	if err != nil {
		return "", 0, "", "", false
	}
	f := strings.SplitN(strings.TrimRight(string(out), "\n"), "\t", 4)
	if len(f) != 4 {
		return "", 0, "", "", false
	}
	window, err = strconv.Atoi(f[1])
	if err != nil {
		return "", 0, "", "", false
	}
	return f[0], window, f[2], f[3], true
}

// palmterm notify を実行する。引数は空白でつないで1つの文にする（省けば画面が「終わりました」を出す）。
// 届かなかったときは標準エラーにそう出すが、いつも 0 を返す。
func runNotify(errOut io.Writer, args []string) int {
	// 長い文はここで切っておく（そのまま送ると受け口の1行の長さを超え、読めずに捨てられる）。
	req := notifyRequest{Text: cleanNotifyText(strings.Join(args, " "))}
	if session, window, id, name, ok := notifyOrigin(); ok {
		req.Session, req.Window, req.WindowID, req.WindowName = session, &window, id, cleanNotifyText(name)
	}
	line, _ := json.Marshal(req)
	sock := filepath.Join(runtimeDir(), notifySocketName)
	conn, err := net.DialTimeout("unix", sock, time.Second)
	if err != nil {
		fmt.Fprintln(errOut, "palmterm notify: palmterm is not running (could not connect to "+sock+"), so the notification was not shown.")
		return 0
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	conn.Write(append(line, '\n'))
	reply, _ := bufio.NewReader(conn).ReadString('\n')
	reply = strings.TrimSpace(reply)
	switch {
	case strings.HasPrefix(reply, "ok "):
	case reply == "none":
		fmt.Fprintln(errOut, "palmterm notify: no palmterm page is open, so the notification was not shown.")
	default:
		fmt.Fprintf(errOut, "palmterm notify: palmterm did not show the notification (reply: %q).\n", reply)
	}
	return 0
}
