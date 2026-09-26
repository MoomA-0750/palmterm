package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// tmux の中のプログラム（gh など）がブラウザを開こうとしたときに、その URL を palmterm を開いている
// ブラウザに渡す。palmterm は tmux の環境に BROWSER=<palmterm-open> を設定し、palmterm-open（palmterm 自身）が
// サーバーのソケットに URL を渡す。サーバーは最後に入力があった画面に届け、画面は「開く」の知らせを出す
// （ブラウザは、押されていないのに新しいタブを開くことを許さないため）。画面がつながっていなければ、
// palmterm-open は今までどおり xdg-open で開く。

const (
	openCommandName = "palmterm-open"
	maxOpenURLBytes = 8 << 10
)

// つながっている画面。最後に入力があった画面に URL を届ける。
type termClient struct {
	send      func(ctx context.Context, msg []byte) error
	lastInput time.Time
}

type clientSet struct {
	mu      sync.Mutex
	clients map[*termClient]struct{}
}

func (c *clientSet) add(tc *termClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.clients == nil {
		c.clients = map[*termClient]struct{}{}
	}
	c.clients[tc] = struct{}{}
}

func (c *clientSet) remove(tc *termClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.clients, tc)
}

func (c *clientSet) touch(tc *termClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	tc.lastInput = time.Now()
}

// 最後に入力があった画面。つながっている画面がなければ nil。
func (c *clientSet) latest() *termClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	var best *termClient
	for tc := range c.clients {
		if best == nil || tc.lastInput.After(best.lastInput) {
			best = tc
		}
	}
	return best
}

// ブラウザで開いてよい URL か（http と https だけ）。
func validOpenURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && len(raw) <= maxOpenURLBytes
}

// URL を最後に入力があった画面に届ける。届けられたら true。
func (s *server) relayOpen(ctx context.Context, raw string) bool {
	tc := s.clients.latest()
	if tc == nil {
		return false
	}
	msg, _ := json.Marshal(map[string]string{"type": "open", "url": raw})
	return tc.send(ctx, msg) == nil
}

// palmterm-open からの URL を受けるソケット（本人だけがつなげる）。1行に1つの URL を受け、
// 画面に届けたら "ok"、届けられなければ "none"、使えない URL なら "bad" を返す。
func (s *server) serveOpenSocket(path string) error {
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
			go s.handleOpenConn(conn)
		}
	}()
	return nil
}

func (s *server) handleOpenConn(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	line, err := bufio.NewReader(&limitedReader{conn, maxOpenURLBytes + 1}).ReadString('\n')
	if err != nil && line == "" {
		return
	}
	raw := strings.TrimSpace(line)
	switch {
	case !validOpenURL(raw):
		fmt.Fprintln(conn, "bad")
	case s.relayOpen(context.Background(), raw):
		fmt.Fprintln(conn, "ok")
	default:
		fmt.Fprintln(conn, "none")
	}
}

type limitedReader struct {
	r net.Conn
	n int
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.n <= 0 {
		return 0, errors.New("too long")
	}
	if len(p) > l.n {
		p = p[:l.n]
	}
	n, err := l.r.Read(p)
	l.n -= n
	return n, err
}

// ブラウザで開く URL を受けるソケットと palmterm-open を用意する。使えなくても palmterm 自体は動かす。
func (s *server) setupBrowserRelay() {
	dir := runtimeDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Printf(tr("ブラウザの受け渡しを用意できませんでした: %v", "Could not set up the browser relay: %v"), err)
		return
	}
	if err := s.serveOpenSocket(filepath.Join(dir, "open.sock")); err != nil {
		log.Printf(tr("ブラウザの受け渡しを用意できませんでした: %v", "Could not set up the browser relay: %v"), err)
		return
	}
	link, err := prepareOpenCommand(dir)
	if err != nil {
		log.Printf(tr("ブラウザの受け渡しを用意できませんでした: %v", "Could not set up the browser relay: %v"), err)
		return
	}
	s.openCommand = link
}

// tmux に渡す環境。ブラウザの受け渡しを使うときは BROWSER を palmterm-open にする。
func (s *server) tmuxEnv() []string {
	env := terminalEnv()
	if s.openCommand == "" {
		return env
	}
	out := env[:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, "BROWSER=") {
			out = append(out, kv)
		}
	}
	return append(out, "BROWSER="+s.openCommand)
}

// palmterm の実行時のファイルを置く場所（ソケットと palmterm-open）。
func runtimeDir() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return filepath.Join(d, "palmterm")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("palmterm-%d", os.Getuid()))
}

// palmterm-open（palmterm 自身へのリンク）を作り、そのパスを返す。BROWSER にはこのパスを入れる
// （引数を付けずに呼べる1つのコマンドにしておく）。
func prepareOpenCommand(dir string) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	link := filepath.Join(dir, openCommandName)
	os.Remove(link)
	return link, os.Symlink(self, link)
}

// palmterm-open：URL を palmterm の画面に渡す。渡せなければ xdg-open で開く。
func runOpen(urls []string) int {
	status := 0
	for _, raw := range urls {
		if sendToPalmterm(filepath.Join(runtimeDir(), "open.sock"), raw) {
			continue
		}
		if err := openLocally(raw); err != nil {
			fmt.Fprintln(os.Stderr, err)
			status = 1
		}
	}
	return status
}

// サーバーのソケットに URL を渡し、画面に届いたら true。
func sendToPalmterm(sock, raw string) bool {
	conn, err := net.DialTimeout("unix", sock, time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	fmt.Fprintln(conn, raw)
	reply, _ := bufio.NewReader(conn).ReadString('\n')
	return strings.TrimSpace(reply) == "ok"
}

// palmterm の画面がないときは、この PC のブラウザで開く。xdg-open は BROWSER を見るので、
// palmterm-open に戻ってこないよう BROWSER を外して呼ぶ。
var openLocally = func(raw string) error {
	cmd := exec.Command("xdg-open", raw)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "BROWSER=") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	return cmd.Run()
}
