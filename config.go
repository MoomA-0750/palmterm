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
	Language string      `toml:"language" json:"language"`
	Keys     []keyConfig `toml:"keys" json:"keys,omitempty"` // 空なら画面側の既定の並び
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
			log.Printf("設定ファイルを読めませんでした（既定の設定を使います）: %v", c.err)
		}
	}
	cfg := c.cfg
	if cfg.Language == "" {
		cfg.Language = "en"
	}
	return cfg, c.err
}

func loadConfig(path string) (config, error) {
	var cfg config
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return config{}, err
	}
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

// 画面の設定（言語とキーバー）を返す。設定ファイルを読めなかったときは、既定の設定と理由を返す。
func (s *server) handleConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := settings.get()
	resp := struct {
		config
		Path  string `json:"path"`
		Error string `json:"error,omitempty"`
	}{config: cfg, Path: settings.path}
	if err != nil {
		resp.Error = err.Error()
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(resp)
}
