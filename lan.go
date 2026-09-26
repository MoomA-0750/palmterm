package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// LAN から直接つなぐときの HTTPS 用の、自分で署名した証明書。
// 平文の HTTP だと LAN の中でトークンが見えてしまい、ブラウザも貼り付け（クリップボード）を許さないので、
// LAN 側は HTTPS にする。ブラウザは最初の1回だけ警告を出す。
// 一度作ったものを使い続け、この PC の IP アドレスが証明書に入っていなければ作り直す。
const (
	lanCertFile = "lan-cert.pem"
	lanKeyFile  = "lan-key.pem"
	lanCertDays = 825
)

// dir の証明書を読む。無い・期限が近い・ips を含まないときは作り直して保存する。
func lanCertificate(dir string, ips []net.IP, host string) (tls.Certificate, error) {
	certPath, keyPath := filepath.Join(dir, lanCertFile), filepath.Join(dir, lanKeyFile)
	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil && certCovers(cert.Leaf, ips) {
		return cert, nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return tls.Certificate{}, fmt.Errorf("%s を読めませんでした: %w", certPath, err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "palmterm (" + host + ")"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(0, 0, lanCertDays),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           append([]net.IP{net.IPv4(127, 0, 0, 1)}, ips...),
		DNSNames:              []string{"localhost", host},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return tls.Certificate{}, err
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return tls.Certificate{}, err
	}
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

// 期限まで30日以上あり、ips がすべて入っているか。
func certCovers(leaf *x509.Certificate, ips []net.IP) bool {
	if leaf == nil || time.Until(leaf.NotAfter) < 30*24*time.Hour {
		return false
	}
	for _, ip := range ips {
		if !slices.ContainsFunc(leaf.IPAddresses, ip.Equal) {
			return false
		}
	}
	return true
}

// LAN から見えるこの PC の IPv4 アドレス（ループバックと、Docker などの仮想の橋渡しは除く）。
func lanAddresses() []net.IP {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var ips []net.IP
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 || isVirtualBridge(ifc.Name) {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
				ips = append(ips, n.IP.To4())
			}
		}
	}
	return ips
}

func isVirtualBridge(name string) bool {
	for _, p := range []string{"docker", "br-", "veth", "virbr", "podman", "cni"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// 待ち受けるアドレスから、ブラウザで開く URL を作る。すべてのアドレスで待つとき（":7682" など）は、
// LAN のアドレスごとに1つずつ。
func lanURLs(listen string, ips []net.IP, token string) []string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return nil
	}
	hosts := []string{host}
	if host == "" || host == "0.0.0.0" || host == "::" {
		hosts = nil
		for _, ip := range ips {
			hosts = append(hosts, ip.String())
		}
	}
	urls := make([]string, len(hosts))
	for i, h := range hosts {
		urls[i] = "https://" + net.JoinHostPort(h, port) + "/auth?token=" + token
	}
	return urls
}
