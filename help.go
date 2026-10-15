package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// -help に出すオプションの説明の日本語。英語は main の flag の定義に書く。
var flagUsageJa = map[string]string{
	"listen":            "待ち受けるアドレス（HTTP）",
	"session":           "つなぐ tmux のセッション",
	"token":             "ログイン用のトークン（既定は ~/.config/palmterm/token。最初の起動で作る）",
	"allow-origin":      "WebSocket を受け付けるほかの Origin。カンマ区切り（Vite の開発用サーバーなど）",
	"upload-dir":        "送ったファイルの保存先（既定は ~/.cache/palmterm/uploads）",
	"config":            "設定ファイル（既定は ~/.config/palmterm/config.toml）",
	"relay-browser":     "tmux の中のプログラムがブラウザ（BROWSER）で開こうとした URL を、代わりに palmterm の画面で開く",
	"capture-clipboard": "tmux の set-clipboard を on にし、tmux の中のプログラムがコピーした文字（OSC 52）も palmterm の画面に並ぶ tmux のバッファに入れる",
	"lan":               "このアドレスでも LAN 向けに HTTPS で待ち受ける。例 :7682（自己署名の証明書は ~/.config/palmterm に置く）",
	"version":           "版を出して終わる",
}

// -help を出すときの言語。-help は flag.Parse の途中で出るので、設定ファイルの場所は引数から自分で探し、
// その言語を読む。読めなければ既定の英語。
func helpLanguage(args []string) string {
	path := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if !strings.HasPrefix(a, "-") || name != "config" {
			continue
		}
		if hasValue {
			path = value
		} else if i+1 < len(args) {
			path = args[i+1]
			i++
		}
	}
	if path == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "en"
		}
		path = filepath.Join(dir, "palmterm", "config.toml")
	}
	if cfg, err := loadConfig(path); err == nil && cfg.Language == "ja" {
		return "ja"
	}
	return "en"
}

// -help の出力。英語は flag パッケージの既定の出し方のまま。日本語は同じ形で、説明と既定値を日本語にする。
func printUsage(fs *flag.FlagSet, lang string) {
	w := fs.Output()
	if lang != "ja" {
		fmt.Fprintf(w, "Usage of %s:\n", fs.Name())
		fs.PrintDefaults()
		return
	}
	fmt.Fprintf(w, "%s の使い方:\n", fs.Name())
	fs.VisitAll(func(f *flag.Flag) {
		usage, ok := flagUsageJa[f.Name]
		if !ok {
			usage = f.Usage
		}
		line := "  -" + f.Name
		typ, _ := flag.UnquoteUsage(f)
		if typ != "" {
			line += " " + typ
		}
		line += "\n    \t" + strings.ReplaceAll(usage, "\n", "\n    \t")
		if f.DefValue != "" && f.DefValue != "false" {
			if typ == "string" {
				line += fmt.Sprintf("（既定: %q）", f.DefValue)
			} else {
				line += fmt.Sprintf("（既定: %v）", f.DefValue)
			}
		}
		fmt.Fprintln(w, line)
	})
}
