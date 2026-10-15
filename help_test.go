package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// palmterm -help のオプションの説明は設定の言語で出る。設定が無い・en なら英語のまま。
func TestHelpLanguage(t *testing.T) {
	if testing.Short() {
		t.Skip("実行ファイルを作るので -short では飛ばす")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "palmterm")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	writeConfig := func(name, lang string) string {
		p := filepath.Join(dir, name, "palmterm", "config.toml")
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("language = \""+lang+"\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	jaPath := writeConfig("ja", "ja")
	writeConfig("en", "en")
	// 設定の置き場所を conf にして -help を出す（args が無ければ -help だけ）。終了コード 0 で、標準エラーに出る。
	help := func(t *testing.T, conf string, args ...string) string {
		t.Helper()
		if len(args) == 0 {
			args = []string{"-help"}
		}
		cmd := exec.Command(bin, args...)
		cmd.Env = append(os.Environ(), "HOME="+dir, "XDG_CONFIG_HOME="+conf, "XDG_CACHE_HOME="+dir)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("palmterm -help: %v\n%s", err, stderr.String())
		}
		return stderr.String()
	}

	// 英語の説明（flag の定義のまま）。設定が無いときの出力を基準にする。
	none := help(t, filepath.Join(dir, "none"))
	enUsages := []string{
		"address to listen on (HTTP)",
		"tmux session to attach to",
		"login token",
		"extra WebSocket origins",
		"where uploaded files are saved",
		"configuration file",
		"open URLs that programs in tmux try to open",
		"set tmux's set-clipboard to on",
		"also listen on this address over HTTPS",
		"print the version and exit",
	}
	for _, u := range enUsages {
		if !strings.Contains(none, u) {
			t.Errorf("設定が無いときの -help に英語の説明 %q がありません:\n%s", u, none)
		}
	}
	if !strings.HasPrefix(none, "Usage of "+bin+":\n") || !strings.Contains(none, `(default "127.0.0.1:7681")`) {
		t.Errorf("設定が無いときの -help が flag の既定の形ではありません:\n%s", none)
	}
	if en := help(t, filepath.Join(dir, "en")); en != none {
		t.Errorf("language = \"en\" の -help が設定の無いときと違います:\n%s\n---\n%s", en, none)
	}

	check := func(t *testing.T, out string) {
		t.Helper()
		for name, u := range flagUsageJa {
			if !strings.Contains(out, "  -"+name) || !strings.Contains(out, u) {
				t.Errorf("-%s の日本語の説明 %q がありません", name, u)
			}
		}
		for _, u := range enUsages {
			if strings.Contains(out, u) {
				t.Errorf("英語の説明 %q が残っています", u)
			}
		}
		if !strings.Contains(out, `（既定: "127.0.0.1:7681"）`) || strings.Contains(out, "(default") {
			t.Errorf("既定値が日本語で出ていません")
		}
		if t.Failed() {
			t.Logf("出力:\n%s", out)
		}
	}
	t.Run("ja", func(t *testing.T) { check(t, help(t, filepath.Join(dir, "ja"))) })
	// -config で指した設定の言語も、-help の前でも後でも効く。
	t.Run("ja-config-before", func(t *testing.T) { check(t, help(t, filepath.Join(dir, "none"), "-config", jaPath, "-help")) })
	t.Run("ja-config-after", func(t *testing.T) { check(t, help(t, filepath.Join(dir, "none"), "-help", "--config="+jaPath)) })

	// flag の定義と日本語の説明の数が合っている（新しいオプションの訳し忘れを見つける）。
	if n := strings.Count(none, "\n  -"); n != len(flagUsageJa) {
		t.Errorf("オプションは %d 個、日本語の説明は %d 個", n, len(flagUsageJa))
	}
}
