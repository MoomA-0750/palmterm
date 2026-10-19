package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// 通知（Web Push）：palmterm notify の知らせを、画面を閉じている端末にも OS の通知で届ける。
// 端末は設定画面の「通知」でブラウザのプッシュの宛先（購読）を作ってサーバーに渡し、サーバーはそれを
// ログインした端末の記録（devices.json）に一緒に持つ。端末のログインを取り消せば宛先も消える。
// 送るときは中身を端末の鍵で暗号化し（RFC 8291 の aes128gcm）、サーバーの鍵で署名した VAPID（RFC 8292）を
// 添えて、ブラウザの会社のプッシュの受け口（endpoint）に POST する。サーバーの鍵は設定のフォルダの vapid.pem。
// 画面が見えていてつながっている端末には送らない（帯が出るので）。画面は見えている・隠れたを /api/push/presence で知らせる。

const (
	vapidKeyName = "vapid.pem"
	// VAPID の sub。プッシュの受け口の会社が困ったときの連絡先（Apple は sub が無いと受け付けない）。
	vapidSubject = "https://github.com/MoomA-0750/palmterm"
	pushTTL      = 24 * time.Hour // 端末が電源を切っていたら、この間は受け口に預けておく
	pushRecord   = 4096           // 暗号化の記録の大きさ（中身はこれより小さいので1つで済む）
	// 通知に出す文の長さ。受け口が受ける中身は 4KB までなので、帯（1000 文字）より短く切る。
	maxPushBodyRunes  = 500
	maxPushTitleRunes = 100
	// 見えていると知らせてきた画面を、この間は見えているとみなす（画面は見えている間 1 分ごとに知らせ直す）。
	presenceLifetime = 3 * time.Minute
	maxPresencePages = 16 // 1台の端末で覚えておく画面の数
)

// ブラウザのプッシュの宛先（PushSubscription）。鍵は base64url。
type pushSubscription struct {
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"` // 端末の公開鍵（P-256 の点、65 バイト）
	Auth     string `json:"auth"`   // 端末と分け合った秘密（16 バイト）
}

var b64 = base64.RawURLEncoding

// base64url を読む（末尾の = があってもよい）。
func decodeB64(s string) ([]byte, error) {
	return b64.DecodeString(strings.TrimRight(s, "="))
}

// 宛先が使える形か。endpoint は https の URL、鍵は P-256 の点と 16 バイトの秘密。
func (sub pushSubscription) valid() error {
	u, err := url.Parse(sub.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || len(sub.Endpoint) > 2048 {
		return errors.New("endpoint must be an https URL")
	}
	pub, err := decodeB64(sub.P256dh)
	if err != nil {
		return errors.New("bad p256dh")
	}
	if _, err := ecdh.P256().NewPublicKey(pub); err != nil {
		return errors.New("bad p256dh")
	}
	if auth, err := decodeB64(sub.Auth); err != nil || len(auth) != 16 {
		return errors.New("bad auth")
	}
	return nil
}

type pusher struct {
	key    *ecdsa.PrivateKey
	client *http.Client
	now    func() time.Time

	mu      sync.Mutex
	visible map[string]map[string]time.Time // 端末の ID → 画面の ID → 見えていると最後に知らせてきた時刻
}

func newPusher(key *ecdsa.PrivateKey, client *http.Client) *pusher {
	return &pusher{key: key, client: client, now: time.Now, visible: map[string]map[string]time.Time{}}
}

// サーバーの鍵（VAPID）を読む。無ければ作って本人だけが読める権限で置く。
func loadOrCreateVAPIDKey(path string) (*ecdsa.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		block, _ := pem.Decode(b)
		if block == nil {
			return nil, fmt.Errorf("%s: not a PEM file", path)
		}
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		ek, ok := k.(*ecdsa.PrivateKey)
		if !ok || ek.Curve != elliptic.P256() {
			return nil, fmt.Errorf("%s: not a P-256 key", path)
		}
		return ek, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

// 通知を用意する。鍵を用意できなければ通知だけ使わない（帯はそのまま出る）。
func (s *server) setupPush(confDir string) {
	key, err := loadOrCreateVAPIDKey(filepath.Join(confDir, vapidKeyName))
	if err != nil {
		log.Printf(tr("通知（Web Push）の鍵を用意できませんでした: %v", "Could not prepare the key for notifications (Web Push): %v"), err)
		return
	}
	s.push = newPusher(key, &http.Client{Timeout: 10 * time.Second})
}

// サーバーの公開鍵（applicationServerKey と VAPID の k）。
func (p *pusher) publicKey() []byte {
	pub, err := p.key.PublicKey.ECDH()
	if err != nil {
		panic(err) // P-256 の鍵なので起きない
	}
	return pub.Bytes()
}

// 中身を端末の鍵で暗号化する（RFC 8291）。asKey と salt は送るたびに作る（テストでは決めたものを渡す）。
func encryptPush(plain []byte, sub pushSubscription, asKey *ecdh.PrivateKey, salt []byte) ([]byte, error) {
	uaBytes, err := decodeB64(sub.P256dh)
	if err != nil {
		return nil, err
	}
	uaPub, err := ecdh.P256().NewPublicKey(uaBytes)
	if err != nil {
		return nil, err
	}
	auth, err := decodeB64(sub.Auth)
	if err != nil {
		return nil, err
	}
	secret, err := asKey.ECDH(uaPub)
	if err != nil {
		return nil, err
	}
	asPub := asKey.PublicKey().Bytes()
	keyInfo := "WebPush: info\x00" + string(uaBytes) + string(asPub)
	ikm, err := hkdf.Key(sha256.New, secret, auth, keyInfo, 32)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(plain)+1+gcm.Overhead() > pushRecord {
		return nil, errors.New("push payload too large")
	}
	// 見出し：salt（16）・記録の大きさ（4）・鍵の長さ（1）・送る側の公開鍵。
	var head bytes.Buffer
	head.Write(salt)
	binary.Write(&head, binary.BigEndian, uint32(pushRecord))
	head.WriteByte(byte(len(asPub)))
	head.Write(asPub)
	// 記録は1つだけなので、中身の後ろに「最後の記録」の印（0x02）を付ける。
	return gcm.Seal(head.Bytes(), nonce, append(append([]byte{}, plain...), 2), nil), nil
}

// VAPID の Authorization の値（RFC 8292）。aud は受け口のオリジン、期限は 12 時間。
func (p *pusher) vapidAuthorization(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	header := b64.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))
	claims, _ := json.Marshal(map[string]any{
		"aud": u.Scheme + "://" + u.Host,
		"exp": p.now().Add(12 * time.Hour).Unix(),
		"sub": vapidSubject,
	})
	input := header + "." + b64.EncodeToString(claims)
	digest := sha256.Sum256([]byte(input))
	r, sv, err := ecdsa.Sign(rand.Reader, p.key, digest[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	sv.FillBytes(sig[32:])
	return "vapid t=" + input + "." + b64.EncodeToString(sig) + ", k=" + b64.EncodeToString(p.publicKey()), nil
}

// 1つの宛先に送り、受け口の返した状態コードを返す。
func (p *pusher) send(ctx context.Context, sub pushSubscription, payload []byte) (int, error) {
	asKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return 0, err
	}
	salt := make([]byte, 16)
	rand.Read(salt)
	body, err := encryptPush(payload, sub, asKey, salt)
	if err != nil {
		return 0, err
	}
	auth, err := p.vapidAuthorization(sub.Endpoint)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("TTL", fmt.Sprint(int(pushTTL/time.Second)))
	req.Header.Set("Urgency", "high")
	req.Header.Set("Authorization", auth)
	res, err := p.client.Do(req)
	if err != nil {
		return 0, err
	}
	io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	res.Body.Close()
	return res.StatusCode, nil
}

// 画面が見えている・隠れたを覚える。page は画面ごとのランダムな ID（同じ端末で複数のタブを開いたときに分ける）。
func (p *pusher) setVisible(device, page string, visible bool) {
	now := p.now()
	p.mu.Lock()
	defer p.mu.Unlock()
	pages := p.visible[device]
	for id, at := range pages {
		if now.Sub(at) >= presenceLifetime {
			delete(pages, id)
		}
	}
	if !visible {
		delete(pages, page)
	} else {
		if pages == nil {
			pages = map[string]time.Time{}
			p.visible[device] = pages
		}
		if _, ok := pages[page]; ok || len(pages) < maxPresencePages {
			pages[page] = now
		}
	}
	if len(pages) == 0 {
		delete(p.visible, device)
	}
}

// 端末のどれかの画面が見えているか（見えていると知らせてきてから presenceLifetime の間）。
func (p *pusher) showing(device string) bool {
	now := p.now()
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, at := range p.visible[device] {
		if now.Sub(at) < presenceLifetime {
			return true
		}
	}
	return false
}

// 通知の中身（Service Worker が読む）。
type pushPayload struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	Window   *int   `json:"window,omitempty"`
	WindowID string `json:"windowId,omitempty"`
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

// 帯に送る知らせ（relayNotify の msg）から通知の中身を作る。題はウィンドウ（1:build）、無ければ palmterm。
// 文が空なら設定の言語の「終わりました」。
func newPushPayload(msg map[string]any) pushPayload {
	p := pushPayload{Title: "palmterm", Body: msg["text"].(string)}
	if p.Body == "" {
		p.Body = tr("終わりました", "Done")
	}
	p.Body = truncateRunes(p.Body, maxPushBodyRunes)
	if w, ok := msg["window"].(int); ok {
		p.Window = &w
		p.Title = fmt.Sprint(w)
		if name, _ := msg["windowName"].(string); name != "" {
			p.Title += ":" + name
		}
		p.Title = truncateRunes(p.Title, maxPushTitleRunes)
		p.WindowID, _ = msg["windowId"].(string)
	}
	return p
}

// 通知を有効にしている端末のうち、shown（帯が届いて画面も見えている端末）でないものに送り、届いた数を返す。
// 受け口が「もう無い」（404・410）と返した宛先は消す。
func (s *server) pushNotify(ctx context.Context, payload pushPayload, shown map[string]bool) int {
	if s.push == nil {
		return 0
	}
	body, _ := json.Marshal(payload)
	var wg sync.WaitGroup
	var mu sync.Mutex
	n := 0
	for id, sub := range s.devices.pushTargets() {
		if shown[id] {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, err := s.push.send(ctx, sub, body)
			switch {
			case err != nil:
				log.Printf(tr("通知を送れませんでした: %v", "Could not send a notification: %v"), err)
			case code == http.StatusNotFound || code == http.StatusGone:
				// 端末で通知を切った・ブラウザのデータを消したなど。この宛先にはもう届かない。
				if err := s.devices.dropPush(id, sub.Endpoint); err != nil {
					log.Printf(tr("ログインした端末の記録を書けませんでした: %v", "Could not save the logged-in devices: %v"), err)
				}
			case code/100 != 2:
				log.Printf(tr("通知を送れませんでした（受け口の応答 %d）", "Could not send a notification (push service replied %d)"), code)
			default:
				mu.Lock()
				n++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return n
}

// GET /api/push：サーバーの公開鍵と、この端末が通知を有効にしているか。
func (s *server) handlePushState(w http.ResponseWriter, r *http.Request) {
	if s.push == nil {
		http.Error(w, tr("このサーバーでは通知を使えません", "Notifications are not available on this server"), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]any{
		"publicKey":  b64.EncodeToString(s.push.publicKey()),
		"subscribed": s.devices.pushOf(deviceID(r)) != nil,
	})
}

// POST /api/push/subscribe：ブラウザの PushSubscription（toJSON の形）を、この端末の宛先にする。
func (s *server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	if s.push == nil {
		http.Error(w, tr("このサーバーでは通知を使えません", "Notifications are not available on this server"), http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sub := pushSubscription{Endpoint: req.Endpoint, P256dh: req.Keys.P256dh, Auth: req.Keys.Auth}
	if err := sub.valid(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.setDevicePush(w, r, &sub)
}

// POST /api/push/unsubscribe：この端末の宛先を消す（通知を切る）。
func (s *server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	s.setDevicePush(w, r, nil)
}

func (s *server) setDevicePush(w http.ResponseWriter, r *http.Request, sub *pushSubscription) {
	ok, err := s.devices.setPush(deviceID(r), sub)
	if err != nil {
		log.Printf(tr("ログインした端末の記録を書けませんでした: %v", "Could not save the logged-in devices: %v"), err)
	}
	if !ok {
		http.Error(w, tr("その端末はログインしていません", "That device is not logged in"), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/push/presence：{"page": 画面の ID, "visible": 見えているか}。見えている端末には通知を送らない。
func (s *server) handlePushPresence(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Page    string `json:"page"`
		Visible bool   `json:"visible"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil || req.Page == "" || len(req.Page) > 64 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if s.push != nil {
		s.push.setVisible(deviceID(r), req.Page, req.Visible)
	}
	w.WriteHeader(http.StatusNoContent)
}
