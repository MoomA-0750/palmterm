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
	Version  int          `toml:"version" json:"-"` // 書き方の版。書かなければ 1（configVersion より新しければ読める項目だけ使う）
	Language string       `toml:"language" json:"language"`
	Keys     []keyConfig  `toml:"keys" json:"keys,omitempty"` // 空なら画面側の既定の並び
	Theme    themeConfig  `toml:"theme" json:"theme"`
	Themes   customThemes `toml:"themes" json:"themes,omitempty"` // 自分のテーマ（[themes.名前]）
	Font     fontConfig   `toml:"font" json:"font"`               // 中身は fonts.go
}

// 画面の色。name で組み込みのテーマか [themes.名前] の自分のテーマを選ぶ（空なら default）。
// 色は [theme] には書かず、[themes.名前] に書く（書くと設定ファイルの書き間違いになる）。
type themeConfig struct {
	Name string `toml:"name" json:"name,omitempty"`
}

// 自分のテーマ（[themes.名前]）。base で組み込みのテーマを選び（書かなければ default）、書いた色だけ上書きする。
// 色は "#rrggbb" か "#rgb"。中身の確かめは画面側で行う（書き間違えた色は base の色に戻し、
// 知らない項目・知らない base・組み込みと同じ名前は知らせる）。そのため型を決めずにそのまま画面に渡す。
type customThemes map[string]map[string]any

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
// 版 2 で [font] の stack（フォントを優先順位付きで並べる）が増えた。版 1 の family・file も読める。
const configVersion = 2

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
		var themeKeys []string
		for i, k := range und {
			names[i] = k.String()
			if len(k) > 1 && k[0] == "theme" {
				themeKeys = append(themeKeys, k.String())
			}
		}
		if len(themeKeys) > 0 {
			return config{}, fmt.Errorf("[theme] に書けるのは name だけです。色は [themes.名前] に書き、[theme] の name でその名前を選んでください / [theme] only takes name; write colors in [themes.NAME] and pick it with name in [theme]: %s", strings.Join(themeKeys, ", "))
		}
		return config{}, fmt.Errorf("知らない項目があります / unknown keys: %s", strings.Join(names, ", "))
	}
	if md.IsDefined("font", "stack") && (md.IsDefined("font", "family") || md.IsDefined("font", "file")) {
		return config{}, fmt.Errorf("[font] の stack と family・file は一緒に書けません（stack に並べてください） / [font] stack cannot be used together with family or file (list them in stack)")
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
		Themes   toml.Primitive `toml:"themes"`
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
	if md.IsDefined("themes") {
		var themes customThemes
		if md.PrimitiveDecode(raw.Themes, &themes) == nil {
			cfg.Themes = themes
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
		// [font] から決めたフォントの並び（先頭から順に使う）。配るものは url、PC に無ければ family の名前でスマホのフォントを探す
		FontFaces []fontFace `json:"fontFaces,omitempty"`
		// [font] のファイルを使えなかった理由（画面で知らせる）
		FontError string `json:"fontError,omitempty"`
	}{config: cfg, Path: settings.path, Version: version}
	font := chooseFont(cfg.Font, settings.path)
	resp.FontFaces = fontFaces(font)
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
