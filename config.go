package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
)

// 設定ファイル（~/.config/palmterm/config.toml）。書き換えたら、画面を読み込み直すだけで反映する
// （ファイルの更新時刻が変わっていたら読み直す）。中身の見本は config.example.toml。
type config struct {
	Version  int         `toml:"version" json:"-"` // 書き方の版。書かなければ 1（configVersion より新しければ読める項目だけ使う）
	Language string      `toml:"language" json:"language"`
	Keys     []keyConfig `toml:"keys" json:"keys,omitempty"` // 空なら画面側の既定の並び
	Theme    themeConfig `toml:"theme" json:"theme"`
	Font     fontConfig  `toml:"font" json:"font"` // 中身は fonts.go
}

// 画面の色。name で組み込みのテーマを選び（空なら default）、書いた色だけ上書きする。
// 色は "#rrggbb" か "#rgb"。名前と色の確かめは画面側で行う（間違えた色だけ既定に戻して知らせる）。
type themeConfig struct {
	Name       string `toml:"name" json:"name,omitempty"`
	Background string `toml:"background" json:"background,omitempty"`
	Foreground string `toml:"foreground" json:"foreground,omitempty"`
	Cursor     string `toml:"cursor" json:"cursor,omitempty"`
	Accent     string `toml:"accent" json:"accent,omitempty"` // ボタンの強調の色（既定は緑）
	// 端末の16色（ANSI の 0〜15 の順）
	Black         string `toml:"black" json:"black,omitempty"`
	Red           string `toml:"red" json:"red,omitempty"`
	Green         string `toml:"green" json:"green,omitempty"`
	Yellow        string `toml:"yellow" json:"yellow,omitempty"`
	Blue          string `toml:"blue" json:"blue,omitempty"`
	Magenta       string `toml:"magenta" json:"magenta,omitempty"`
	Cyan          string `toml:"cyan" json:"cyan,omitempty"`
	White         string `toml:"white" json:"white,omitempty"`
	BrightBlack   string `toml:"bright_black" json:"bright_black,omitempty"`
	BrightRed     string `toml:"bright_red" json:"bright_red,omitempty"`
	BrightGreen   string `toml:"bright_green" json:"bright_green,omitempty"`
	BrightYellow  string `toml:"bright_yellow" json:"bright_yellow,omitempty"`
	BrightBlue    string `toml:"bright_blue" json:"bright_blue,omitempty"`
	BrightMagenta string `toml:"bright_magenta" json:"bright_magenta,omitempty"`
	BrightCyan    string `toml:"bright_cyan" json:"bright_cyan,omitempty"`
	BrightWhite   string `toml:"bright_white" json:"bright_white,omitempty"`
}

// キーバーのキー1つ。key・mod・text のどれか1つを書く（中身の確かめは画面側で行う）。
type keyConfig struct {
	Label  string `toml:"label" json:"label,omitempty"`
	Key    string `toml:"key" json:"key,omitempty"`   // "esc"、"ctrl+c"、"alt+shift+left" など
	Mod    string `toml:"mod" json:"mod,omitempty"`   // "ctrl"・"alt"・"shift"（1回押すと次のキーに1回、すばやく2回で固定）
	Text   string `toml:"text" json:"text,omitempty"` // そのまま送る文字
	Icon   string `toml:"icon" json:"icon,omitempty"`
	Repeat *bool  `toml:"repeat" json:"repeat,omitempty"` // 押し続けたら繰り返す（省略時はキーによる）
}

type configStore struct {
	path    string
	mu      sync.Mutex
	modTime time.Time
	cfg     config
	err     error
}

func newConfigStore(path string) *configStore {
	c := &configStore{path: path}
	c.get() // 起動時に一度読んで、書き間違いがあればログに出す
	return c
}

// 今の設定と、読めなかったときの理由（そのときは既定の設定を返す）。
func (c *configStore) get() (config, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	info, err := os.Stat(c.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		c.cfg, c.err, c.modTime = config{}, nil, time.Time{}
	case err != nil:
		c.cfg, c.err = config{}, err
	case !info.ModTime().Equal(c.modTime):
		c.modTime = info.ModTime()
		c.cfg, c.err = loadConfig(c.path)
		if c.err != nil {
			// 読めていないので言語がわからない。両方で出す。
			log.Printf("設定ファイルを読めませんでした（既定の設定を使います） / Could not read the config file (using the defaults): %v", c.err)
		} else if c.cfg.Version > configVersion {
			log.Printf("新しい版の設定ファイルです（版 %d）。読める項目だけ使います / The config file is from a newer version (%d); using only the settings this palmterm understands", c.cfg.Version, c.cfg.Version)
		}
	}
	cfg := c.cfg
	if cfg.Language == "" {
		cfg.Language = "en"
	}
	return cfg, c.err
}

// この palmterm が知っている設定ファイルの書き方の版。書き方を変えるときは古い書き方も読めるようにして、版を上げる。
const configVersion = 1

func loadConfig(path string) (config, error) {
	var head struct {
		Version int `toml:"version"`
	}
	md, err := toml.DecodeFile(path, &head)
	if err != nil {
		return config{}, err
	}
	switch {
	case !md.IsDefined("version"):
		head.Version = 1 // 版を書く前からの設定ファイル
	case head.Version < 1:
		return config{}, fmt.Errorf("version は 1 以上の整数 / version must be 1 or greater: %d", head.Version)
	case head.Version > configVersion:
		return loadNewerConfig(path, head.Version)
	}

	var cfg config
	md, err = toml.DecodeFile(path, &cfg)
	if err != nil {
		return config{}, err
	}
	cfg.Version = head.Version
	// 書き間違えた項目名は黙って無視されてしまうので知らせる。
	if und := md.Undecoded(); len(und) > 0 {
		names := make([]string, len(und))
		for i, k := range und {
			names[i] = k.String()
		}
		return config{}, fmt.Errorf("知らない項目があります / unknown keys: %s", strings.Join(names, ", "))
	}
	if cfg.Language != "" && cfg.Language != "ja" && cfg.Language != "en" {
		return config{}, fmt.Errorf("language は \"ja\" か \"en\" / language must be \"ja\" or \"en\": %q", cfg.Language)
	}
	return cfg, nil
}

// この palmterm より新しい版の設定ファイルは、読める項目だけ使う。知らない項目や、
// 書き方が変わって読めない項目は黙って飛ばす（新しい版では正しい書き方かもしれないため）。
// 新しい版であることは、画面を開いたときに知らせる（handleConfig）。
func loadNewerConfig(path string, version int) (config, error) {
	var raw struct {
		Language toml.Primitive `toml:"language"`
		Keys     toml.Primitive `toml:"keys"`
		Theme    toml.Primitive `toml:"theme"`
		Font     toml.Primitive `toml:"font"`
	}
	md, err := toml.DecodeFile(path, &raw)
	if err != nil {
		return config{}, err
	}
	cfg := config{Version: version}
	if md.IsDefined("language") {
		var lang string
		if md.PrimitiveDecode(raw.Language, &lang) == nil && (lang == "ja" || lang == "en") {
			cfg.Language = lang
		}
	}
	if md.IsDefined("keys") {
		var keys []keyConfig
		if md.PrimitiveDecode(raw.Keys, &keys) == nil {
			cfg.Keys = keys
		}
	}
	if md.IsDefined("theme") {
		var theme themeConfig
		if md.PrimitiveDecode(raw.Theme, &theme) == nil {
			cfg.Theme = theme
		}
	}
	if md.IsDefined("font") {
		var font fontConfig
		if md.PrimitiveDecode(raw.Font, &font) == nil {
			cfg.Font = font
		}
	}
	return cfg, nil
}

var settings *configStore

// 画面に出す文言を、設定の言語で選ぶ（既定は英語）。
func tr(ja, en string) string {
	if settings != nil {
		if cfg, _ := settings.get(); cfg.Language == "ja" {
			return ja
		}
	}
	return en
}

// 画面の設定（言語・キーバー・色・フォント）を返す。設定ファイルを読めなかったときは、既定の設定と理由を返す。
func (s *server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeConfigResponse(w)
}

// 今の設定を /api/config の形で返す（画面でテーマやフォントを選んだあとの応答にも使う）。
func writeConfigResponse(w http.ResponseWriter) {
	cfg, err := settings.get()
	resp := struct {
		config
		Path    string `json:"path"`
		Version string `json:"version"` // palmterm -version と同じ版（画面の tmux パネルに出す）
		Error   string `json:"error,omitempty"`
		// 設定ファイルの版がこの palmterm より新しいとき、その版（画面で知らせる）
		NewerConfig int `json:"newerConfig,omitempty"`
		// [font] のフォントを配るときの URL（PC に無ければ空。画面は family の名前でスマホのフォントを探す）
		FontURL string `json:"fontURL,omitempty"`
		// [font] の file を使えなかった理由（画面で知らせる）
		FontError string `json:"fontError,omitempty"`
	}{config: cfg, Path: settings.path, Version: version}
	font := chooseFont(cfg.Font, settings.path)
	resp.FontURL = fontURL(font)
	if font.err != nil {
		resp.FontError = font.err.Error()
	}
	if cfg.Version > configVersion {
		resp.NewerConfig = cfg.Version
	}
	if err != nil {
		resp.Error = err.Error()
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(resp)
}
