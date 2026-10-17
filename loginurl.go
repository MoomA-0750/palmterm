package main

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"rsc.io/qr"
)

// palmterm url：ログイン用の URL と、スマホのカメラで読める QR コードを出す。
// ログイン用の URL のトークンは使い捨て（1回ログインしたら使えない。出してから5分で切れる）。
// palmterm url を打つたびに、動いている palmterm に操作用のソケットで新しいトークンを出させ、
// 実際に待ち受けているアドレスのログイン用の URL を受け取る。トークンはサーバーのメモリにだけあるので、
// 再起動すると、まだ使っていないトークンはすべて無効になる。
// tailscale serve が palmterm に中継していれば、その URL も先頭に足す（スマホからはこれが使いやすい）。

const (
	loginSocketName    = "login.sock"
	loginURLCommand    = "login-url"
	loginTokenLifetime = 5 * time.Minute
	staleLoginURLsFile = "login-urls" // 前の版が書いていた、固定のトークン入りの URL の控え
	qrQuietZone        = 2            // QR の周りの余白（マス）
)

// 使い捨てのログイン用トークン。ハッシュと期限だけを持つ。
type loginTokens struct {
	mu     sync.Mutex
	tokens map[string]time.Time // トークンのハッシュ → 期限
	now    func() time.Time     // テストで差し替える
}

func newLoginTokens() *loginTokens {
	return &loginTokens{tokens: map[string]time.Time{}, now: time.Now}
}

// 新しいトークンを出す。期限の切れたものはここで捨てる。
func (l *loginTokens) issue() string {
	tok := randomHex(24)
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	for h, exp := range l.tokens {
		if !now.Before(exp) {
			delete(l.tokens, h)
		}
	}
	l.tokens[hashSecret(tok)] = now.Add(loginTokenLifetime)
	return tok
}

// まだ使えるトークンか（使い切らない。ログインのページを出すときに使う）。
func (l *loginTokens) valid(tok string) bool {
	if tok == "" {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	exp, ok := l.tokens[hashSecret(tok)]
	return ok && l.now().Before(exp)
}

// トークンを使い切る。使えたら true。
func (l *loginTokens) use(tok string) bool {
	if tok == "" {
		return false
	}
	h := hashSecret(tok)
	l.mu.Lock()
	defer l.mu.Unlock()
	exp, ok := l.tokens[h]
	delete(l.tokens, h)
	return ok && l.now().Before(exp)
}

// 新しいトークンを1つ出し、待ち受けている各アドレスのログイン用の URL にする（どれで使っても使い切る）。
func (s *server) newLoginURLs() []string {
	tok := s.logins.issue()
	urls := make([]string, len(s.loginBases))
	for i, b := range s.loginBases {
		urls[i] = b + "/auth?token=" + tok
	}
	return urls
}

// 起動時の案内に、使い捨てのログイン用の URL を出す（先頭はこの PC の HTTP、続きは LAN）。
func (s *server) logLoginURLs() {
	for i, u := range s.newLoginURLs() {
		if i == 0 {
			log.Printf(tr("palmterm: %s を開いてください（5分以内に1回だけ使えます。palmterm url で新しく出せます）",
				"palmterm: open %s (works once, within 5 minutes; run palmterm url for a new one)"), u)
		} else {
			log.Printf(tr("palmterm（LAN）: %s を開いてください（最初は証明書の警告が出るので、先へ進む）",
				"palmterm (LAN): open %s (the browser warns about the certificate the first time; proceed)"), u)
		}
	}
}

// palmterm url の受け口を用意する。使えなくても palmterm 自体は動かす。
// 前の版が書いていた URL の控えは、もう使わないので消す。
func (s *server) setupLoginSocket() {
	dir, err := ensureRuntimeDir()
	if err == nil {
		os.Remove(filepath.Join(dir, staleLoginURLsFile))
		err = s.serveLoginSocket(filepath.Join(dir, loginSocketName))
	}
	if err != nil {
		log.Printf(tr("palmterm url の受け口を用意できませんでした: %v", "Could not set up the socket for palmterm url: %v"), err)
	}
}

// 1行に1つの命令を受け、login-url なら新しいトークンのログイン用の URL を1行に1つ返して閉じる。
func (s *server) serveLoginSocket(path string) error {
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
			go func() {
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(5 * time.Second))
				line, err := bufio.NewReader(io.LimitReader(conn, 256)).ReadString('\n')
				if err != nil && line == "" {
					return
				}
				if strings.TrimSpace(line) != loginURLCommand {
					fmt.Fprintln(conn, "bad")
					return
				}
				for _, u := range s.newLoginURLs() {
					fmt.Fprintln(conn, u)
				}
			}()
		}
	}()
	return nil
}

// 動いている palmterm に新しいログイン用の URL を出させる。
func requestLoginURLs() ([]string, error) {
	conn, err := net.DialTimeout("unix", filepath.Join(runtimeDir(), loginSocketName), time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	fmt.Fprintln(conn, loginURLCommand)
	b, err := io.ReadAll(io.LimitReader(conn, 64<<10))
	if err != nil {
		return nil, err
	}
	urls := strings.Fields(string(b))
	if len(urls) == 0 || !strings.Contains(urls[0], "/auth?token=") {
		return nil, fmt.Errorf("unexpected reply: %q", strings.TrimSpace(string(b)))
	}
	return urls, nil
}

// tailscale serve の設定（JSON）。テストで差し替える。
var tailscaleServeStatus = func() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "tailscale", "serve", "status", "--json").Output()
}

// tailscale serve の設定から、listen（palmterm の HTTP の待ち受け）に中継している公開先の URL を取り出す。
func tailscaleLoginURLs(statusJSON []byte, listen, token string) []string {
	var status struct {
		Web map[string]struct {
			Handlers map[string]struct{ Proxy string }
		}
	}
	if json.Unmarshal(statusJSON, &status) != nil {
		return nil
	}
	_, port, err := net.SplitHostPort(listen)
	if err != nil {
		return nil
	}
	var urls []string
	for hostport, web := range status.Web {
		h, ok := web.Handlers["/"]
		if !ok {
			continue
		}
		target, err := url.Parse(h.Proxy)
		if err != nil || target.Port() != port || (target.Hostname() != "127.0.0.1" && target.Hostname() != "localhost") {
			continue
		}
		host := strings.TrimSuffix(hostport, ":443")
		urls = append(urls, "https://"+host+"/auth?token="+token)
	}
	// map の順番は決まらないので、並べ直す（443 番の短い URL が先）。
	slices.SortFunc(urls, func(a, b string) int { return cmp.Or(len(a)-len(b), strings.Compare(a, b)) })
	return urls
}

// 端末に QR を描く。上下2つのマスを1文字（▀▄█ と空白）にし、明るいマスを █ の側にする
// （暗い背景の端末で、ふつうの QR と同じ白黒になる）。
func renderQR(code *qr.Code) string {
	n := code.Size + 2*qrQuietZone
	light := func(x, y int) bool {
		x, y = x-qrQuietZone, y-qrQuietZone
		return x < 0 || y < 0 || x >= code.Size || y >= code.Size || !code.Black(x, y)
	}
	var b strings.Builder
	for y := 0; y < n; y += 2 {
		for x := 0; x < n; x++ {
			top, bottom := light(x, y), y+1 >= n || light(x, y+1)
			switch {
			case top && bottom:
				b.WriteRune('█')
			case top:
				b.WriteRune('▀')
			case bottom:
				b.WriteRune('▄')
			default:
				b.WriteRune(' ')
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// palmterm url [-no-qr]
func runURL(out io.Writer, args []string) int {
	fs := flag.NewFlagSet("url", flag.ContinueOnError)
	fs.SetOutput(out)
	noQR := fs.Bool("no-qr", false, "do not print a QR code")
	if fs.Parse(args) != nil {
		return 2
	}
	urls, err := requestLoginURLs()
	if err != nil {
		fmt.Fprintf(out, "palmterm is not running (could not get a login URL from %s: %v).\n", filepath.Join(runtimeDir(), loginSocketName), err)
		return 1
	}
	// 最初の URL は 127.0.0.1 の HTTP の待ち受け。そこから Tailscale の公開先を探す。
	if first, err := url.Parse(urls[0]); err == nil {
		if status, err := tailscaleServeStatus(); err == nil {
			urls = append(tailscaleLoginURLs(status, first.Host, first.Query().Get("token")), urls...)
		}
	}
	for _, u := range urls {
		fmt.Fprintln(out, u)
	}
	if *noQR {
		return 0
	}
	code, err := qr.Encode(urls[0], qr.L)
	if err != nil {
		fmt.Fprintln(out, err)
		return 1
	}
	fmt.Fprintf(out, "\nScan with your phone to log in (%s; works once, within 5 minutes):\n\n%s", strings.SplitN(urls[0], "/auth?", 2)[0], renderQR(code))
	return 0
}
