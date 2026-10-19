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
	"encoding/binary"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// テスト用のプッシュの受け口（ブラウザの会社の受け口の代わり）。届いた中身を端末の鍵で復号し、
// VAPID の署名を確かめてから覚える。status を変えると、その状態コードを返す。
type pushReceiver struct {
	t      *testing.T
	srv    *httptest.Server
	uaKey  *ecdh.PrivateKey
	auth   []byte
	mu     sync.Mutex
	got    []pushPayload
	status int
}

func newPushReceiver(t *testing.T) *pushReceiver {
	t.Helper()
	r := &pushReceiver{t: t, status: http.StatusCreated, auth: make([]byte, 16)}
	rand.Read(r.auth)
	var err error
	if r.uaKey, err = ecdh.P256().GenerateKey(rand.Reader); err != nil {
		t.Fatal(err)
	}
	r.srv = httptest.NewTLSServer(http.HandlerFunc(r.serve))
	t.Cleanup(r.srv.Close)
	return r
}

func (r *pushReceiver) subscription(path string) pushSubscription {
	return pushSubscription{Endpoint: r.srv.URL + path, P256dh: b64.EncodeToString(r.uaKey.PublicKey().Bytes()), Auth: b64.EncodeToString(r.auth)}
}

func (r *pushReceiver) serve(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	if err := r.check(req, body); err != nil {
		r.t.Errorf("受け口: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(r.status)
}

type checkErr string

func (e checkErr) Error() string { return string(e) }

func (r *pushReceiver) check(req *http.Request, body []byte) error {
	if req.Method != http.MethodPost || req.Header.Get("Content-Encoding") != "aes128gcm" {
		return checkErr("POST と Content-Encoding: aes128gcm のはず")
	}
	if ttl, err := strconv.Atoi(req.Header.Get("TTL")); err != nil || ttl <= 0 {
		return checkErr("TTL が無い")
	}
	if err := verifyVAPID(req.Header.Get("Authorization"), "https://"+req.Host); err != nil {
		return err
	}
	plain, err := decryptPush(body, r.uaKey, r.auth)
	if err != nil {
		return err
	}
	var p pushPayload
	if err := json.Unmarshal(plain, &p); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status/100 == 2 {
		r.got = append(r.got, p)
	}
	return nil
}

func (r *pushReceiver) received() []pushPayload {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]pushPayload(nil), r.got...)
}

// RFC 8292 の Authorization（vapid t=JWT, k=公開鍵）を確かめる：ES256 の署名が k で合い、aud が受け口の
// オリジン、exp が 24 時間以内の先、sub がある。
func verifyVAPID(h, origin string) error {
	rest, ok := strings.CutPrefix(h, "vapid ")
	if !ok {
		return checkErr("vapid の Authorization でない: " + h)
	}
	var tok, k string
	for _, part := range strings.Split(rest, ",") {
		part = strings.TrimSpace(part)
		if v, ok := strings.CutPrefix(part, "t="); ok {
			tok = v
		}
		if v, ok := strings.CutPrefix(part, "k="); ok {
			k = v
		}
	}
	f := strings.Split(tok, ".")
	if len(f) != 3 {
		return checkErr("JWT でない")
	}
	kb, err := b64.DecodeString(k)
	if err != nil {
		return err
	}
	x, y := elliptic.Unmarshal(elliptic.P256(), kb)
	if x == nil {
		return checkErr("k が P-256 の点でない")
	}
	sig, err := b64.DecodeString(f[2])
	if err != nil || len(sig) != 64 {
		return checkErr("署名の形が違う")
	}
	digest := sha256.Sum256([]byte(f[0] + "." + f[1]))
	if !ecdsa.Verify(&ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, digest[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		return checkErr("署名が合わない")
	}
	hb, _ := b64.DecodeString(f[0])
	var head map[string]string
	if json.Unmarshal(hb, &head) != nil || head["alg"] != "ES256" {
		return checkErr("alg が ES256 でない")
	}
	cb, _ := b64.DecodeString(f[1])
	var claims struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(cb, &claims); err != nil {
		return err
	}
	now := time.Now().Unix()
	if claims.Aud != origin || claims.Exp <= now || claims.Exp > now+24*3600 || claims.Sub == "" {
		return checkErr("JWT の中身が違う: " + string(cb))
	}
	return nil
}

// RFC 8291 の aes128gcm を、端末（ブラウザ）の側として復号する。
func decryptPush(body []byte, uaKey *ecdh.PrivateKey, auth []byte) ([]byte, error) {
	if len(body) < 21 {
		return nil, checkErr("短すぎる")
	}
	salt, rs, idlen := body[:16], binary.BigEndian.Uint32(body[16:20]), int(body[20])
	if len(body) < 21+idlen || rs < 18 {
		return nil, checkErr("見出しが壊れている")
	}
	asPubBytes, ct := body[21:21+idlen], body[21+idlen:]
	if uint32(len(ct)) > rs {
		return nil, checkErr("記録が大きすぎる")
	}
	asPub, err := ecdh.P256().NewPublicKey(asPubBytes)
	if err != nil {
		return nil, err
	}
	secret, err := uaKey.ECDH(asPub)
	if err != nil {
		return nil, err
	}
	ikm, _ := hkdf.Key(sha256.New, secret, auth, "WebPush: info\x00"+string(uaKey.PublicKey().Bytes())+string(asPubBytes), 32)
	cek, _ := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	nonce, _ := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, err
	}
	plain = bytes.TrimRight(plain, "\x00")
	if len(plain) == 0 || plain[len(plain)-1] != 2 {
		return nil, checkErr("最後の記録の印が無い")
	}
	return plain[:len(plain)-1], nil
}

// テスト用のサーバーに通知を用意し、受け口の TLS を信じるようにする。
func enablePush(t *testing.T, s *server, r *pushReceiver) {
	t.Helper()
	key, err := loadOrCreateVAPIDKey(filepath.Join(t.TempDir(), vapidKeyName))
	if err != nil {
		t.Fatal(err)
	}
	s.push = newPusher(key, r.srv.Client())
}

// ログインした端末を作り、Cookie の値と ID を返す。
func loginDevice(t *testing.T, s *server, ua string) (secret, id string) {
	t.Helper()
	secret, err := s.devices.login(ua, "")
	if err != nil {
		t.Fatal(err)
	}
	return secret, s.devices.lookup(secret).ID
}

// その端末の画面（帯を受ける WebSocket の代わり）。
func (r *notifyRecorder) clientOf(device string) *termClient {
	tc := r.client()
	tc.device = device
	return tc
}

func (r *notifyRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.msgs)
}

// プッシュは、中身を端末の鍵で暗号化し（RFC 8291 の aes128gcm）、VAPID（RFC 8292）の署名を添えて受け口に POST する。
// 受け口は端末の秘密鍵で復号でき、署名はサーバーの公開鍵で合う。題は打ったウィンドウ、本文は文。
func TestPushEncryptsAndSigns(t *testing.T) {
	r := newPushReceiver(t)
	s := newTestServer(t)
	enablePush(t, s, r)
	w := 1
	payload := newPushPayload(map[string]any{"text": "ビルドが終わった", "window": w, "windowName": "build", "windowId": "@3"})
	code, err := s.push.send(context.Background(), r.subscription("/push/abc"), mustJSON(t, payload))
	if err != nil || code != http.StatusCreated {
		t.Fatalf("code=%d err=%v", code, err)
	}
	got := r.received()
	if len(got) != 1 || got[0].Title != "1:build" || got[0].Body != "ビルドが終わった" || got[0].WindowID != "@3" || got[0].Window == nil || *got[0].Window != 1 {
		t.Fatalf("届いた中身: %+v", got)
	}

	// 文が空なら「終わりました」（設定の言語）。ウィンドウが無ければ題は palmterm。長い文は切る
	if p := newPushPayload(map[string]any{"text": ""}); p.Title != "palmterm" || p.Body != "Done" {
		t.Fatalf("文なし: %+v", p)
	}
	long := newPushPayload(map[string]any{"text": strings.Repeat("あ", maxNotifyRunes)})
	if n := len([]rune(long.Body)); n != maxPushBodyRunes {
		t.Fatalf("長い文: %d 文字", n)
	}
	if _, err := s.push.send(context.Background(), r.subscription("/long"), mustJSON(t, long)); err != nil {
		t.Fatalf("長い文も 4KB に収まって送れるはず: %v", err)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// サーバーの鍵は vapid.pem に本人だけが読める権限で置き、次に起動しても同じ鍵を使う。
func TestVAPIDKeyIsKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "palmterm", vapidKeyName)
	a, err := loadOrCreateVAPIDKey(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("権限: %v %v", info.Mode(), err)
	}
	b, err := loadOrCreateVAPIDKey(path)
	if err != nil || !a.Equal(b) {
		t.Fatalf("読み直した鍵が違う: %v", err)
	}
}

// 帯が届いて画面も見えている端末にはプッシュを送らず、帯だけを出す。画面が隠れている・画面がつながっていない
// 端末にはプッシュで送る。どちらかが届けば palmterm notify は標準エラーに何も出さない。
func TestNotifyPushesOnlyToDevicesNotShowing(t *testing.T) {
	isolateTmux(t, "t1")
	isolateRuntimeDir(t)
	r := newPushReceiver(t)
	s := newTestServer(t)
	enablePush(t, s, r)
	s.setupNotifySocket()
	t.Setenv("TMUX_PANE", "")
	_, phone := loginDevice(t, s, "iPhone Safari/1")
	s.devices.setPush(phone, ptr(r.subscription("/phone")))

	var errOut bytes.Buffer
	// 画面がつながっていない：プッシュだけで届く
	runNotify(&errOut, []string{"one"})
	if got := r.received(); len(got) != 1 || got[0].Body != "one" || errOut.Len() != 0 {
		t.Fatalf("つながっていないとき: %+v stderr=%q", got, errOut.String())
	}

	// 画面がつながっていて見えている：帯だけ
	var page notifyRecorder
	tc := page.clientOf(phone)
	s.clients.add(tc)
	s.push.setVisible(phone, "p1", true)
	runNotify(&errOut, []string{"two"})
	if len(r.received()) != 1 || page.count() != 1 {
		t.Fatalf("見えているとき: プッシュ %d 件、帯 %d 件", len(r.received()), page.count())
	}

	// 同じ端末のもう1つのタブが隠れても、見えているタブがあれば帯だけ
	s.push.setVisible(phone, "p2", false)
	runNotify(&errOut, []string{"three"})
	if len(r.received()) != 1 {
		t.Fatal("見えているタブがあるのにプッシュした")
	}

	// 隠れた（別のアプリに切り替えた・画面を消した）：帯とプッシュの両方
	s.push.setVisible(phone, "p1", false)
	runNotify(&errOut, []string{"four"})
	if got := r.received(); len(got) != 2 || got[1].Body != "four" || page.count() != 3 {
		t.Fatalf("隠れたとき: %+v 帯 %d 件", got, page.count())
	}

	// 見えていると知らせてから時間がたった（隠れたと知らせられずに止まった）画面は、見えているとみなさない
	s.push.setVisible(phone, "p1", true)
	s.push.now = func() time.Time { return time.Now().Add(presenceLifetime) }
	runNotify(&errOut, []string{"five"})
	if len(r.received()) != 3 {
		t.Fatal("古い「見えている」でプッシュを止めた")
	}
	s.push.now = time.Now

	// 見えていると知らせていても、画面がつながっていなければ（帯が届かない）プッシュする
	s.push.setVisible(phone, "p1", true)
	s.clients.remove(tc)
	runNotify(&errOut, []string{"six"})
	if len(r.received()) != 4 || errOut.Len() != 0 {
		t.Fatalf("つながっていないとき: %d 件 stderr=%q", len(r.received()), errOut.String())
	}
}

func ptr[T any](v T) *T { return &v }

// 設定画面で通知を切る・端末の一覧で取り消す・palmterm logout-all のどれでも、その端末には届かなくなる。
// どこにも届かなければ palmterm notify は標準エラーにそう出す（終了コードは 0）。
func TestNotifyPushStopsWhenTurnedOffOrLoggedOut(t *testing.T) {
	isolateTmux(t, "t1")
	isolateRuntimeDir(t)
	r := newPushReceiver(t)
	s := newTestServer(t)
	enablePush(t, s, r)
	s.setupNotifySocket()
	s.setupControlSocket()
	t.Setenv("TMUX_PANE", "")
	h := s.routes()
	key := b64.EncodeToString(r.uaKey.PublicKey().Bytes())
	auth := b64.EncodeToString(r.auth)

	subscribe := func(secret, path string) {
		t.Helper()
		body := `{"endpoint":"` + r.srv.URL + path + `","keys":{"p256dh":"` + key + `","auth":"` + auth + `"}}`
		if w := request(h, secret, "POST", "/api/push/subscribe", body); w.Code != http.StatusNoContent {
			t.Fatalf("subscribe: %d %s", w.Code, w.Body)
		}
	}
	notify := func(text string) (int, string) {
		t.Helper()
		before := len(r.received())
		var errOut bytes.Buffer
		if code := runNotify(&errOut, []string{text}); code != 0 {
			t.Fatalf("終了コード %d", code)
		}
		return len(r.received()) - before, errOut.String()
	}

	a, _ := loginDevice(t, s, "iPhone Safari/1")
	b, bID := loginDevice(t, s, "Android Chrome/1")
	subscribe(a, "/a")
	subscribe(b, "/b")
	if w := request(h, a, "GET", "/api/push", ""); !strings.Contains(w.Body.String(), `"subscribed":true`) || !strings.Contains(w.Body.String(), `"publicKey":"`) {
		t.Fatalf("GET /api/push: %s", w.Body)
	}
	if n, _ := notify("x"); n != 2 {
		t.Fatalf("2台に届くはず: %d", n)
	}

	// 設定画面で切る
	if w := request(h, a, "POST", "/api/push/unsubscribe", ""); w.Code != http.StatusNoContent {
		t.Fatalf("unsubscribe: %d", w.Code)
	}
	if w := request(h, a, "GET", "/api/push", ""); !strings.Contains(w.Body.String(), `"subscribed":false`) {
		t.Fatalf("切ったあと: %s", w.Body)
	}
	if n, _ := notify("x"); n != 1 {
		t.Fatalf("切った端末には届かないはず: %d", n)
	}

	// 端末の一覧で取り消す
	if w := request(h, a, "POST", "/api/devices/revoke", `{"id":"`+bID+`"}`); w.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d", w.Code)
	}
	n, stderr := notify("x")
	if n != 0 || !strings.Contains(stderr, "no palmterm page is open") {
		t.Fatalf("取り消した端末には届かず、標準エラーに出るはず: %d %q", n, stderr)
	}

	// palmterm logout-all
	subscribe(a, "/a")
	var out, errOut bytes.Buffer
	if code := runLogoutAll(&out, &errOut, nil); code != 0 {
		t.Fatalf("logout-all: %d %s", code, errOut.String())
	}
	if n, stderr := notify("x"); n != 0 || stderr == "" {
		t.Fatalf("logout-all のあとは届かないはず: %d %q", n, stderr)
	}
}

// 受け口が「もう無い」（410）と返した宛先は消し、次からは送らない。ほかの失敗（500）では消さない。
// プッシュが届かず帯も無ければ、標準エラーにそう出す。
func TestPushDropsGoneSubscriptions(t *testing.T) {
	isolateTmux(t, "t1")
	isolateRuntimeDir(t)
	r := newPushReceiver(t)
	s := newTestServer(t)
	enablePush(t, s, r)
	s.setupNotifySocket()
	t.Setenv("TMUX_PANE", "")
	_, id := loginDevice(t, s, "iPhone Safari/1")
	s.devices.setPush(id, ptr(r.subscription("/a")))

	var errOut bytes.Buffer
	r.status = http.StatusInternalServerError
	runNotify(&errOut, []string{"x"})
	if !strings.Contains(errOut.String(), "no palmterm page is open") || s.devices.pushOf(id) == nil {
		t.Fatalf("500: stderr=%q 宛先=%v", errOut.String(), s.devices.pushOf(id))
	}
	r.status = http.StatusGone
	runNotify(&errOut, []string{"x"})
	if s.devices.pushOf(id) != nil {
		t.Fatal("410 の宛先が残っている")
	}
}

// 宛先は https の URL と P-256 の鍵・16 バイトの秘密だけ受け付ける。ログインしていなければ使えない。
func TestPushSubscribeValidates(t *testing.T) {
	r := newPushReceiver(t)
	s := newTestServer(t)
	enablePush(t, s, r)
	h := s.routes()
	secret, id := loginDevice(t, s, "iPhone Safari/1")
	key := b64.EncodeToString(r.uaKey.PublicKey().Bytes())
	auth := b64.EncodeToString(r.auth)
	for _, body := range []string{
		`{"endpoint":"http://push.example/a","keys":{"p256dh":"` + key + `","auth":"` + auth + `"}}`,
		`{"endpoint":"https://push.example/a","keys":{"p256dh":"AAAA","auth":"` + auth + `"}}`,
		`{"endpoint":"https://push.example/a","keys":{"p256dh":"` + key + `","auth":"AAAA"}}`,
		`not json`,
	} {
		if w := request(h, secret, "POST", "/api/push/subscribe", body); w.Code != http.StatusBadRequest {
			t.Errorf("%s → %d", body, w.Code)
		}
	}
	if s.devices.pushOf(id) != nil {
		t.Fatal("おかしな宛先を覚えた")
	}
	for _, path := range []string{"/api/push", "/api/push/subscribe", "/api/push/presence"} {
		method := "POST"
		if path == "/api/push" {
			method = "GET"
		}
		if w := request(h, "", method, path, "{}"); w.Code != http.StatusUnauthorized {
			t.Errorf("ログインなしの %s → %d", path, w.Code)
		}
	}
	// Service Worker はログインなしでも返す（キャッシュさせない）
	if w := request(h, "", "GET", "/sw.js", ""); w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("/sw.js: %d %q", w.Code, w.Header().Get("Cache-Control"))
	}
}

func request(h http.Handler, secret, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if secret != "" {
		req.AddCookie(&http.Cookie{Name: cookieName, Value: secret})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}
