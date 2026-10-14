package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// ログインした端末（ブラウザ）ごとの記録。/auth で正しいトークンを受け取るたびに1つ作り、ランダムな値を
// Cookie に入れる。サーバーには値のハッシュだけを持つので、記録のファイルが読まれても Cookie は作れない。
// 端末ごとに消せる（ログアウト・取り消し）。トークンを変えると、前のトークンで作った記録は全部捨てる。

const (
	loginLifetime = 365 * 24 * time.Hour
	// 最後に使った時刻は、この間隔より細かくはファイルに書かない（リクエストのたびに書かない）。
	lastSeenSaveInterval = time.Hour
)

type device struct {
	ID       string    `json:"id"`              // 一覧と取り消しで使う、外に出してよい番号
	Hash     string    `json:"hash"`            // Cookie の値の SHA-256
	Name     string    `json:"name"`            // ログインしたときの User-Agent から作った名前（例 "iPad · Safari"）
	Label    string    `json:"label,omitempty"` // 本人が付けた名前（空なら Name で見分ける）
	Created  time.Time `json:"created"`
	LastSeen time.Time `json:"lastSeen"`
	saved    time.Time // LastSeen を最後にファイルに書いた時刻
}

type deviceFile struct {
	Token   string    `json:"token"` // どのトークンで作った記録か（トークンから計算した値）
	Devices []*device `json:"devices"`
}

type deviceStore struct {
	mu      sync.Mutex
	path    string // 空ならファイルに書かない
	tokenID string
	devices map[string]*device // Hash から引く
	now     func() time.Time   // テストで差し替える
}

// path の記録を読む。別のトークンで作った記録と、期限の切れた記録は捨てる。
func newDeviceStore(path, token string) *deviceStore {
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte("palmterm devices v1"))
	d := &deviceStore{path: path, tokenID: hex.EncodeToString(mac.Sum(nil)), devices: map[string]*device{}, now: time.Now}
	if path == "" {
		return d
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Printf(tr("ログインした端末の記録を読めませんでした: %v", "Could not read the logged-in devices: %v"), err)
		}
		return d
	}
	var f deviceFile
	if err := json.Unmarshal(b, &f); err != nil {
		log.Printf(tr("ログインした端末の記録を読めませんでした: %v", "Could not read the logged-in devices: %v"), err)
		return d
	}
	if !hmac.Equal([]byte(f.Token), []byte(d.tokenID)) {
		return d // トークンが変わった：全端末を締め出す
	}
	for _, dev := range f.Devices {
		if dev.Hash != "" && dev.ID != "" && d.now().Sub(dev.Created) < loginLifetime {
			dev.saved = dev.LastSeen
			d.devices[dev.Hash] = dev
		}
	}
	return d
}

func hashSecret(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

// 新しい端末を記録し、Cookie に入れる値を返す。
func (d *deviceStore) login(userAgent string) (string, error) {
	secret := randomHex(32)
	now := d.now()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.devices[hashSecret(secret)] = &device{ID: randomHex(8), Hash: hashSecret(secret), Name: deviceName(userAgent), Created: now, LastSeen: now, saved: now}
	return secret, d.saveLocked()
}

// Cookie の値の端末を返し、最後に使った時刻を更新する。知らない値・期限切れなら nil。
func (d *deviceStore) lookup(secret string) *device {
	if secret == "" {
		return nil
	}
	h := hashSecret(secret)
	now := d.now()
	d.mu.Lock()
	defer d.mu.Unlock()
	dev := d.devices[h]
	if dev == nil {
		return nil
	}
	if now.Sub(dev.Created) >= loginLifetime {
		delete(d.devices, h)
		d.saveLocked()
		return nil
	}
	dev.LastSeen = now
	if now.Sub(dev.saved) >= lastSeenSaveInterval {
		dev.saved = now
		d.saveLocked()
	}
	return dev
}

// id の端末の記録を消す。消したら true。
func (d *deviceStore) remove(id string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for h, dev := range d.devices {
		if dev.ID == id {
			delete(d.devices, h)
			return true, d.saveLocked()
		}
	}
	return false, nil
}

// 端末に付けた名前の長さの上限（文字数）。
const maxDeviceLabel = 40

// id の端末に名前を付ける。前後の空白と制御文字は除き、空なら付けた名前を消す。端末が無ければ false。
func (d *deviceStore) rename(id, label string) (bool, error) {
	label = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, label))
	if utf8.RuneCountInString(label) > maxDeviceLabel {
		label = string([]rune(label)[:maxDeviceLabel])
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, dev := range d.devices {
		if dev.ID == id {
			dev.Label = label
			return true, d.saveLocked()
		}
	}
	return false, nil
}

// 新しく使った順の一覧（コピー）。
func (d *deviceStore) list() []device {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]device, 0, len(d.devices))
	for _, dev := range d.devices {
		out = append(out, *dev)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out
}

func (d *deviceStore) saveLocked() error {
	if d.path == "" {
		return nil
	}
	f := deviceFile{Token: d.tokenID, Devices: make([]*device, 0, len(d.devices))}
	for _, dev := range d.devices {
		f.Devices = append(f.Devices, dev)
	}
	sort.Slice(f.Devices, func(i, j int) bool { return f.Devices[i].Created.Before(f.Devices[j].Created) })
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(d.path), 0o700); err != nil {
		return err
	}
	// 書きかけのファイルを読まないよう、別の名前で書いてから置き換える。
	tmp := d.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, d.path)
}

// User-Agent から、一覧で見分けるための短い名前を作る（例 "iPhone · Safari"）。
func deviceName(ua string) string {
	osName := ""
	for _, c := range []struct{ key, name string }{
		{"iPad", "iPad"}, {"iPhone", "iPhone"}, {"Android", "Android"}, {"CrOS", "ChromeOS"},
		{"Windows", "Windows"}, {"Macintosh", "Mac"}, {"Linux", "Linux"},
	} {
		if strings.Contains(ua, c.key) {
			osName = c.name
			break
		}
	}
	browser := ""
	for _, c := range []struct{ key, name string }{
		{"EdgA/", "Edge"}, {"EdgiOS/", "Edge"}, {"Edg/", "Edge"}, {"FxiOS/", "Firefox"}, {"Firefox/", "Firefox"},
		{"CriOS/", "Chrome"}, {"SamsungBrowser/", "Samsung Internet"}, {"Chrome/", "Chrome"}, {"Safari/", "Safari"},
	} {
		if strings.Contains(ua, c.key) {
			browser = c.name
			break
		}
	}
	switch {
	case osName != "" && browser != "":
		return osName + " · " + browser
	case osName != "" || browser != "":
		return osName + browser
	}
	return tr("不明な端末", "Unknown device")
}
