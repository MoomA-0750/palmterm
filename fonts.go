package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

// 設定ファイルの [font]。family に PC に入っているフォントの名前を書くと、そのファイルを探して画面に配る。
// file にフォントのファイルを書くと、そのファイルを配る（相対パスは設定ファイルのあるフォルダから数える）。
// 配るのは /api/font の1つだけで、ここで決めたファイルのほかは配らない。
type fontConfig struct {
	Family string `toml:"family" json:"family,omitempty"`
	File   string `toml:"file" json:"file,omitempty"`
}

// 配るフォントのファイル（見つからなければ path は空）と、file を書いたのに使えないときの理由。
type fontChoice struct {
	path string
	err  error
}

var fontCache struct {
	mu     sync.Mutex
	key    string
	choice fontChoice
	at     time.Time
}

// 探し直すまでの時間。設定を変えたときはすぐ探し直す（フォントを入れたあとは、少し待つか設定ファイルを保存し直す）。
const fontCacheTTL = 30 * time.Second

// 設定の [font] から配るファイルを決める。PC を探すのは時間がかかるので、同じ設定なら少しのあいだ覚えておく。
func chooseFont(f fontConfig, configPath string) fontChoice {
	if f.File == "" && f.Family == "" {
		return fontChoice{}
	}
	key := f.Family + "\x00" + f.File + "\x00" + configPath
	fontCache.mu.Lock()
	defer fontCache.mu.Unlock()
	if fontCache.key == key && time.Since(fontCache.at) < fontCacheTTL {
		return fontCache.choice
	}
	var c fontChoice
	if f.File != "" {
		c.path, c.err = fontFile(f.File, configPath)
	} else {
		c.path = findFont(f.Family, fontDirs())
	}
	fontCache.key, fontCache.choice, fontCache.at = key, c, time.Now()
	return c
}

func forgetFont() {
	fontCache.mu.Lock()
	fontCache.key = ""
	fontCache.mu.Unlock()
}

// file に書いたパスを、フォントのファイルとして確かめる。
func fontFile(file, configPath string) (string, error) {
	path := file
	if rest, ok := strings.CutPrefix(file, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, rest)
		}
	} else if !filepath.IsAbs(file) {
		path = filepath.Join(filepath.Dir(configPath), file)
	}
	if _, err := fontFormat(path); err != nil {
		return "", err
	}
	return path, nil
}

// ファイルの先頭を見て、ブラウザが読めるフォントの形式を返す（Content-Type に使う）。
func fontFormat(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.IsDir() {
		return "", fmt.Errorf("%s: フォルダです / is a directory", path)
	}
	head := make([]byte, 4)
	if _, err := io.ReadFull(f, head); err != nil {
		return "", fmt.Errorf("%s: フォントのファイルではありません / not a font file", path)
	}
	switch string(head) {
	case "wOF2":
		return "font/woff2", nil
	case "wOFF":
		return "font/woff", nil
	case "OTTO":
		return "font/otf", nil
	case "ttcf":
		return "font/collection", nil
	case "\x00\x01\x00\x00", "true":
		return "font/ttf", nil
	}
	return "", fmt.Errorf("%s: フォントのファイルではありません / not a font file", path)
}

// フォントを探す場所。fontconfig の既定と、macOS のフォントの置き場所。
func fontDirs() []string {
	var dirs []string
	home, _ := os.UserHomeDir()
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" && home != "" {
		data = filepath.Join(home, ".local", "share")
	}
	if data != "" {
		dirs = append(dirs, filepath.Join(data, "fonts"))
	}
	if home != "" {
		dirs = append(dirs, filepath.Join(home, ".fonts"))
	}
	sys := os.Getenv("XDG_DATA_DIRS")
	if sys == "" {
		sys = "/usr/local/share:/usr/share"
	}
	for _, d := range filepath.SplitList(sys) {
		if d != "" {
			dirs = append(dirs, filepath.Join(d, "fonts"))
		}
	}
	if runtime.GOOS == "darwin" {
		if home != "" {
			dirs = append(dirs, filepath.Join(home, "Library", "Fonts"))
		}
		dirs = append(dirs, "/Library/Fonts", "/System/Library/Fonts")
	}
	return dirs
}

// family の名前のフォントのファイルを探す（見つからなければ空）。普通の太さで斜体でないものを選ぶ。
// dirs の中を先に探し、無ければ fontconfig（fc-list）に訊く。
func findFont(family string, dirs []string) string {
	var files []string
	for _, dir := range dirs {
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				switch strings.ToLower(filepath.Ext(path)) {
				case ".ttf", ".otf", ".ttc":
					files = append(files, path)
				}
			}
			return nil
		})
	}
	if best := bestFont(family, files); best != "" {
		return best
	}
	return bestFont(family, fcList(family))
}

// fontconfig が知っている、その名前のフォントのファイル。
func fcList(family string) []string {
	r := strings.NewReplacer(`\`, `\\`, `-`, `\-`, `:`, `\:`, `,`, `\,`, `=`, `\=`)
	out, err := exec.Command("fc-list", "--format", "%{file}\n", ":family="+r.Replace(family)).Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

func bestFont(family string, files []string) string {
	best, bestScore := "", -1
	for _, path := range files {
		for _, n := range sfntNames(path) {
			if !strings.EqualFold(n.family, family) {
				continue
			}
			score := 0
			style := strings.ToLower(n.style)
			if style == "regular" || style == "book" || style == "normal" || style == "roman" {
				score += 4
			}
			if !strings.Contains(style, "bold") && !strings.Contains(style, "italic") && !strings.Contains(style, "oblique") {
				score += 2
			}
			if !strings.EqualFold(filepath.Ext(path), ".ttc") {
				score++ // 1つのフォントだけのファイルのほうが、ブラウザで確実に読める
			}
			if score > bestScore {
				best, bestScore = path, score
			}
		}
	}
	return best
}

type sfntName struct{ family, style string }

// TrueType・OpenType（とその集まり）のファイルから、フォントの名前と太さ・斜体の名前を読む。
// 名前は家族名（name ID 1・16）ごとに1つずつ返す。読めないファイルは空。
func sfntNames(path string) []sfntName {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	head := make([]byte, 12)
	if _, err := f.ReadAt(head, 0); err != nil {
		return nil
	}
	offsets := []int64{0}
	if string(head[:4]) == "ttcf" {
		n := binary.BigEndian.Uint32(head[8:])
		if n > 64 {
			n = 64
		}
		buf := make([]byte, 4*n)
		if _, err := f.ReadAt(buf, 12); err != nil {
			return nil
		}
		offsets = offsets[:0]
		for i := range n {
			offsets = append(offsets, int64(binary.BigEndian.Uint32(buf[4*i:])))
		}
	}
	var names []sfntName
	for _, off := range offsets {
		names = append(names, sfntFontNames(f, off)...)
	}
	return names
}

func sfntFontNames(f io.ReaderAt, off int64) []sfntName {
	head := make([]byte, 12)
	if _, err := f.ReadAt(head, off); err != nil {
		return nil
	}
	switch string(head[:4]) {
	case "\x00\x01\x00\x00", "true", "OTTO":
	default:
		return nil
	}
	numTables := int(binary.BigEndian.Uint16(head[4:]))
	dir := make([]byte, 16*numTables)
	if _, err := f.ReadAt(dir, off+12); err != nil {
		return nil
	}
	var table []byte
	for i := range numTables {
		rec := dir[16*i:]
		if string(rec[:4]) != "name" {
			continue
		}
		length := binary.BigEndian.Uint32(rec[12:])
		if length > 1<<20 {
			return nil
		}
		table = make([]byte, length)
		if _, err := f.ReadAt(table, int64(binary.BigEndian.Uint32(rec[8:]))); err != nil {
			return nil
		}
	}
	if len(table) < 6 {
		return nil
	}
	count := int(binary.BigEndian.Uint16(table[2:]))
	strOff := int(binary.BigEndian.Uint16(table[4:]))
	byID := map[uint16][]string{}
	for i := range count {
		p := 6 + 12*i
		if p+12 > len(table) {
			break
		}
		rec := table[p:]
		platform, id := binary.BigEndian.Uint16(rec), binary.BigEndian.Uint16(rec[6:])
		length, at := int(binary.BigEndian.Uint16(rec[8:])), strOff+int(binary.BigEndian.Uint16(rec[10:]))
		if id != 1 && id != 2 && id != 16 && id != 17 || at+length > len(table) {
			continue
		}
		s := table[at : at+length]
		var text string
		switch platform {
		case 0, 3: // UTF-16BE
			u := make([]uint16, len(s)/2)
			for j := range u {
				u[j] = binary.BigEndian.Uint16(s[2*j:])
			}
			text = string(utf16.Decode(u))
		case 1: // Mac Roman（名前はほぼ ASCII）
			text = string(bytes.ToValidUTF8(s, nil))
		default:
			continue
		}
		byID[id] = append(byID[id], text)
	}
	// 新しいフォントは、太さ違いをまとめた名前（16・17）と、太さごとの名前（1・2）の両方を持つ。
	var names []sfntName
	for _, fam := range byID[16] {
		names = append(names, sfntName{family: fam, style: first(byID[17], byID[2])})
	}
	for _, fam := range byID[1] {
		names = append(names, sfntName{family: fam, style: first(byID[2])})
	}
	return names
}

func first(lists ...[]string) string {
	for _, l := range lists {
		if len(l) > 0 {
			return l[0]
		}
	}
	return ""
}

// 設定の [font] で決めたフォントのファイルを配る（ログインした端末だけ。requireAuth の内側に置く）。
// 配るのはこの1つだけで、URL の中身からほかのファイルを選ぶことはない。
func (s *server) handleFont(w http.ResponseWriter, r *http.Request) {
	cfg, _ := settings.get()
	c := chooseFont(cfg.Font, settings.path)
	if c.path == "" {
		http.NotFound(w, r)
		return
	}
	typ, err := fontFormat(c.path)
	if err != nil {
		forgetFont()
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(c.path)
	if err != nil {
		forgetFont()
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", typ)
	w.Header().Set("Cache-Control", "private, no-cache") // 更新時刻で確かめ直す（変わっていなければ 304）
	http.ServeContent(w, r, "", info.ModTime(), f)
}

// 画面に渡すフォントの URL（配るものが無ければ空）。中身が変われば URL も変わるようにする。
func fontURL(c fontChoice) string {
	if c.path == "" {
		return ""
	}
	info, err := os.Stat(c.path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("/api/font?v=%x", info.ModTime().UnixNano()^info.Size())
}
