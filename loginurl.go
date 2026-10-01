package main

import (
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"rsc.io/qr"
)

// palmterm url：ログイン用の URL と、スマホのカメラで読める QR コードを出す。
// サービスとして動かしているとログの URL を探しにくいので、サーバーは起動したときに、実際に待ち受けている
// ログイン用の URL を実行時のフォルダ（本人だけが読める）に書いておき、このコマンドがそれを読む。
// tailscale serve が palmterm に中継していれば、その URL も先頭に足す（スマホからはこれが使いやすい）。

const (
	loginURLsFile = "login-urls"
	qrQuietZone   = 2 // QR の周りの余白（マス）
)

// サーバーが待ち受けているログイン用の URL を書いておく（1行に1つ。トークンを含むので本人だけが読める）。
func writeLoginURLs(urls []string) error {
	dir := runtimeDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, loginURLsFile), []byte(strings.Join(urls, "\n")+"\n"), 0o600)
}

func readLoginURLs() ([]string, error) {
	b, err := os.ReadFile(filepath.Join(runtimeDir(), loginURLsFile))
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(b)), nil
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
	urls, err := readLoginURLs()
	if err != nil || len(urls) == 0 {
		fmt.Fprintln(out, "palmterm is not running (no login URLs found in "+runtimeDir()+").")
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
	fmt.Fprintf(out, "\nScan with your phone to log in (%s):\n\n%s", strings.SplitN(urls[0], "/auth?", 2)[0], renderQR(code))
	return 0
}
