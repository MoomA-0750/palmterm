package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// palmterm -version は版を1行出して 0 で終わり、サーバーは起動しない。
// 版は -ldflags "-X main.version=..." で埋め込め、埋め込まなければ dev。
func TestVersionFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("実行ファイルを作るので -short では飛ばす")
	}
	dir := t.TempDir()
	for _, c := range []struct {
		name, ldflags, want string
	}{
		{"embedded", "-X main.version=v1.2.3-4-gabcdef", "palmterm v1.2.3-4-gabcdef\n"},
		{"default", "", "palmterm dev\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			bin := filepath.Join(dir, "palmterm-"+c.name)
			build := exec.Command("go", "build", "-ldflags", c.ldflags, "-o", bin, ".")
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("go build: %v\n%s", err, out)
			}
			// サーバーが起動したら終わらないので、時間切れで失敗にする。設定やトークンを書かないよう、
			// 置き場所は一時ディレクトリに向ける。
			cmd := exec.Command(bin, "-version")
			cmd.Env = append(os.Environ(), "HOME="+dir, "XDG_CONFIG_HOME="+dir, "XDG_CACHE_HOME="+dir)
			var stderr strings.Builder
			cmd.Stderr = &stderr
			done := make(chan struct{})
			var out []byte
			var err error
			go func() { out, err = cmd.Output(); close(done) }()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				cmd.Process.Kill()
				<-done
				t.Fatal("palmterm -version が終わりません（サーバーが起動した？）")
			}
			if err != nil {
				t.Fatalf("終了コードが 0 ではありません: %v\n%s", err, stderr.String())
			}
			if string(out) != c.want {
				t.Errorf("出力 = %q、期待 %q", out, c.want)
			}
			if stderr.Len() != 0 {
				t.Errorf("標準エラーに出力があります: %q", stderr.String())
			}
		})
	}
}

// make で作る実行ファイルには git describe --tags --always --dirty の値が入る。
func TestMakefileEmbedsGitDescribe(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make がありません")
	}
	desc, err := exec.Command("git", "describe", "--tags", "--always", "--dirty").Output()
	if err != nil {
		t.Skipf("git describe が使えません: %v", err)
	}
	want := `-X main.version=` + strings.TrimSpace(string(desc))
	// -B -n で、実際には作らずに palmterm を作るコマンドだけを出させる。
	out, err := exec.Command("make", "-B", "-n", "palmterm").CombinedOutput()
	if err != nil {
		t.Fatalf("make -n: %v\n%s", err, out)
	}
	var build string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "go build") {
			build = line
		}
	}
	if !strings.Contains(build, want) || !strings.Contains(build, "-o palmterm") {
		t.Errorf("make の go build に %q がありません:\n%s", want, out)
	}
}
