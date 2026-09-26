package main

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLANCertificateIsCreatedReusedAndRenewedForNewAddresses(t *testing.T) {
	dir := t.TempDir()
	lan := net.ParseIP("192.0.2.10")
	c1, err := lanCertificate(dir, []net.IP{lan}, "myhost")
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(c1.Certificate[0])
	if err := leaf.VerifyHostname("192.0.2.10"); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"127.0.0.1", "localhost", "myhost"} {
		if err := leaf.VerifyHostname(h); err != nil {
			t.Errorf("%s: %v", h, err)
		}
	}
	if info, _ := os.Stat(filepath.Join(dir, lanKeyFile)); info.Mode().Perm() != 0o600 {
		t.Fatalf("鍵のファイルの権限: %v", info.Mode())
	}

	// 同じアドレスなら作り直さない（ブラウザで一度許した証明書をそのまま使う）。
	c2, err := lanCertificate(dir, []net.IP{lan}, "myhost")
	if err != nil {
		t.Fatal(err)
	}
	if string(c2.Certificate[0]) != string(c1.Certificate[0]) {
		t.Fatal("作り直しました")
	}

	// アドレスが増えたら作り直す。
	c3, err := lanCertificate(dir, []net.IP{lan, net.ParseIP("10.0.0.5")}, "myhost")
	if err != nil {
		t.Fatal(err)
	}
	leaf3, _ := x509.ParseCertificate(c3.Certificate[0])
	if leaf3.VerifyHostname("10.0.0.5") != nil || leaf3.VerifyHostname("192.0.2.10") != nil {
		t.Fatal("新しいアドレスが入っていません")
	}
}

// 作り直すときは前のアドレスも引き継ぐ。家と職場を行き来しても、2回目からは作り直さない（警告も出ない）。
func TestLANCertificateKeepsPreviousAddresses(t *testing.T) {
	dir := t.TempDir()
	home, work := net.ParseIP("192.0.2.10"), net.ParseIP("198.51.100.20")
	if _, err := lanCertificate(dir, []net.IP{home}, "h"); err != nil {
		t.Fatal(err)
	}
	c2, err := lanCertificate(dir, []net.IP{work}, "h")
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(c2.Certificate[0])
	if leaf.VerifyHostname("192.0.2.10") != nil || leaf.VerifyHostname("198.51.100.20") != nil {
		t.Fatalf("前のアドレスを引き継いでいません: %v", leaf.IPAddresses)
	}
	c3, err := lanCertificate(dir, []net.IP{home}, "h")
	if err != nil {
		t.Fatal(err)
	}
	if string(c3.Certificate[0]) != string(c2.Certificate[0]) {
		t.Fatal("家に戻っただけで作り直しました")
	}
}

// ポートだけ（":7682"）なら、全部のネットワークではなく LAN のアドレスごとに待ち受ける。
func TestLANListenAddrs(t *testing.T) {
	ips := []net.IP{net.ParseIP("192.0.2.10"), net.ParseIP("198.51.100.2")}
	for _, addr := range []string{":7682", "0.0.0.0:7682", "[::]:7682"} {
		got := lanListenAddrs(addr, ips)
		if strings.Join(got, " ") != "192.0.2.10:7682 198.51.100.2:7682" {
			t.Errorf("%s: %v", addr, got)
		}
	}
	if got := lanListenAddrs("192.0.2.10:9000", ips); strings.Join(got, " ") != "192.0.2.10:9000" {
		t.Errorf("アドレスを書いたらそのまま: %v", got)
	}
	if got := lanListenAddrs(":7682", nil); len(got) != 0 {
		t.Errorf("LAN のアドレスが無ければ待ち受けない: %v", got)
	}
}

// 自動で待ち受けるのは LAN・リンクローカル・Tailscale（100.64.0.0/10）のアドレスだけ。グローバルなアドレスは選ばない。
func TestIsLANAddress(t *testing.T) {
	for addr, want := range map[string]bool{
		"192.168.1.20": true, "10.1.2.3": true, "172.16.5.4": true, "169.254.1.1": true, "100.100.1.2": true,
		"8.8.8.8": false, "203.0.113.5": false, "100.128.0.1": false, "172.32.0.1": false,
	} {
		if got := isLANAddress(net.ParseIP(addr)); got != want {
			t.Errorf("%s: %v", addr, got)
		}
	}
}

// 待ち受けは開けたアドレスだけで行い、開けなかったアドレスは理由を返す（1つの失敗で全体を止めない）。重なりはまとめる。
func TestListenAll(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	freeAddr := free.Addr().String()
	free.Close()

	lns, errs := listenAll([]string{busy.Addr().String(), freeAddr, freeAddr})
	for _, ln := range lns {
		defer ln.Close()
	}
	if len(lns) != 1 || lns[0].Addr().String() != freeAddr {
		t.Fatalf("開けたもの: %v", lns)
	}
	if len(errs) != 1 {
		t.Fatalf("開けなかったもの: %v", errs)
	}
}

func TestIsVirtualBridge(t *testing.T) {
	for name, want := range map[string]bool{"docker0": true, "br-3cbaf0fe6026": true, "veth12": true, "ens18": false, "wlan0": false, "tailscale0": false} {
		if isVirtualBridge(name) != want {
			t.Errorf("%s", name)
		}
	}
}

// HTTPS でもログインでき、Cookie には Secure が付き、その Cookie で画面が開ける。
func TestLoginOverHTTPS(t *testing.T) {
	s := newTestServer(t)
	cert, err := lanCertificate(t.TempDir(), nil, "localhost")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(s.routes())
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	srv.StartTLS()
	defer srv.Close()

	client := srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Get(srv.URL + "/auth?token=" + testToken)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	cookies := resp.Cookies()
	if resp.StatusCode != http.StatusSeeOther || len(cookies) != 1 || !cookies[0].Secure {
		t.Fatalf("code=%d cookies=%+v", resp.StatusCode, cookies)
	}

	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.AddCookie(cookies[0])
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "<title>palmterm</title>") {
		t.Fatalf("code=%d", resp.StatusCode)
	}
}

// 見出しを送りきらずに居座る接続を切る時間切れを付ける。WebSocket を切ってしまう読み書き全体の時間切れは付けない。
func TestNewHTTPServerHasTimeouts(t *testing.T) {
	srv := newHTTPServer(":0", http.NotFoundHandler())
	if srv.ReadHeaderTimeout == 0 || srv.IdleTimeout == 0 {
		t.Fatalf("時間切れがありません: %+v", srv)
	}
	if srv.ReadTimeout != 0 || srv.WriteTimeout != 0 {
		t.Fatal("読み書き全体の時間切れは付けない（WebSocket が切れる）")
	}
}
