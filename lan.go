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
// 作り直すときは、前の証明書のアドレスも引き継ぐ（家と職場のように行き来しても、2回目からは作り直さない）。
func lanCertificate(dir string, ips []net.IP, host string) (tls.Certificate, error) {
	certPath, keyPath := filepath.Join(dir, lanCertFile), filepath.Join(dir, lanKeyFile)
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err == nil && certCovers(cert.Leaf, ips) {
		return cert, nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return tls.Certificate{}, fmt.Errorf("%s を読めませんでした: %w", certPath, err)
	}
	var addrs []net.IP
	add := func(ips ...net.IP) {
		for _, ip := range ips {
			if !slices.ContainsFunc(addrs, ip.Equal) {
				addrs = append(addrs, ip)
			}
		}
	}
	add(net.IPv4(127, 0, 0, 1))
	if err == nil && cert.Leaf != nil {
		add(cert.Leaf.IPAddresses...)
	}
	add(ips...)

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
		IPAddresses:           addrs,
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
// インターネットに開かないよう、LAN・リンクローカル・Tailscale のアドレスだけにする（isLANAddress）。
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
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && isLANAddress(n.IP) {
				ips = append(ips, n.IP.To4())
			}
		}
	}
	return ips
}

// Tailscale などが使う 100.64.0.0/10（CGNAT の範囲。インターネットからは届かない）。
var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// LAN（プライベート）・リンクローカル・Tailscale のアドレスか。グローバルなアドレスは false。
func isLANAddress(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLinkLocalUnicast() || cgnat.Contains(ip)
}

func isVirtualBridge(name string) bool {
	for _, p := range []string{"docker", "br-", "veth", "virbr", "podman", "cni"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// 実際に待ち受けるアドレス。ポートだけ（":7682" など）のときは、この PC のすべてのネットワーク
// （IPv6 のグローバルアドレスなども含む）ではなく、LAN のアドレスごとに待ち受ける。
func lanListenAddrs(listen string, ips []net.IP) []string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return nil
	}
	if host != "" && host != "0.0.0.0" && host != "::" {
		return []string{listen}
	}
	addrs := make([]string, len(ips))
	for i, ip := range ips {
		addrs[i] = net.JoinHostPort(ip.String(), port)
	}
	return addrs
}

// ブラウザで開くログイン用の URL（待ち受けるアドレスごとに1つ）。
func lanURLs(listen string, ips []net.IP, token string) []string {
	var urls []string
	for _, addr := range lanListenAddrs(listen, ips) {
		urls = append(urls, "https://"+addr+"/auth?token="+token)
	}
	return urls
}
