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

// commit を進めて make すると、作り直して新しい git describe の値が入る（make -B は要らない）。
// 版もファイルも変わっていなければ go build をやり直さない。
// 一時的な git リポジトリに本物の Makefile と小さな main.go を置き、実際に make して確かめる。
// 画面のビルドは偽の npm で済ませる。
func TestMakeRebuildsOnlyWhenVersionChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("実行ファイルを作るので -short では飛ばす")
	}
	for _, name := range []string{"make", "git", "go"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("%s がありません", name)
		}
	}
	makefile, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	bin := t.TempDir()
	write := func(path, content string, mode os.FileMode) {
		t.Helper()
		p := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("Makefile", string(makefile), 0o644)
	write("go.mod", "module example.com/palmterm\n\ngo 1.21\n", 0o644)
	write("main.go", `package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-version" {
		fmt.Println("palmterm " + version)
	}
}
`, 0o644)
	write("web/package.json", "{}\n", 0o644)
	write("web/index.html", "<html></html>\n", 0o644)
	write("web/src/main.ts", "\n", 0o644)
	write(".gitignore", "/palmterm\n/.version\n/web/node_modules/\n", 0o644)
	// web/node_modules は package.json より新しくして、npm install を走らせない。
	if err := os.MkdirAll(filepath.Join(dir, "web/node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "web/package.json"), past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "npm"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	env := append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	run := func(name string, args ...string) string {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
		}
		return string(out)
	}
	built := func(makeOut string) bool {
		for _, line := range strings.Split(makeOut, "\n") {
			if strings.HasPrefix(line, "go build") {
				return true
			}
		}
		return false
	}
	describe := func() string {
		return strings.TrimSpace(run("git", "describe", "--tags", "--always", "--dirty"))
	}

	run("git", "init", "-q")
	run("git", "add", ".")
	run("git", "commit", "-q", "-m", "first")

	// 1回目は作る。
	if out := run("make", "palmterm"); !built(out) {
		t.Fatalf("最初の make で go build していません:\n%s", out)
	}
	first := describe()
	if got := run("./palmterm", "-version"); got != "palmterm "+first+"\n" {
		t.Fatalf("-version = %q、期待 %q", got, "palmterm "+first+"\n")
	}
	// 版もファイルも同じなら作り直さない。
	if out := run("make", "palmterm"); built(out) {
		t.Errorf("何も変わっていないのに go build し直しました:\n%s", out)
	}

	// commit を進める（ファイルは同じ）と、make だけで新しい版になる。
	run("git", "commit", "-q", "--allow-empty", "-m", "second")
	second := describe()
	if second == first {
		t.Fatalf("commit を進めても git describe が変わりません: %s", first)
	}
	if out := run("make", "palmterm"); !built(out) {
		t.Fatalf("commit を進めた後の make で go build していません:\n%s", out)
	}
	if got := run("./palmterm", "-version"); got != "palmterm "+second+"\n" {
		t.Errorf("-version = %q、期待 %q", got, "palmterm "+second+"\n")
	}
	if out := run("make", "palmterm"); built(out) {
		t.Errorf("作り直した直後の make で、また go build しました:\n%s", out)
	}

	// タグ名に ' " $ が入っても、版はそのまま入り、変わらなければ作り直さない。
	tag := `v1'a"b$c`
	run("git", "tag", tag)
	if out := run("make", "palmterm"); !built(out) {
		t.Fatalf("タグを打った後の make で go build していません:\n%s", out)
	}
	if got := run("./palmterm", "-version"); got != "palmterm "+tag+"\n" {
		t.Errorf("-version = %q、期待 %q", got, "palmterm "+tag+"\n")
	}
	if out := run("make", "palmterm"); built(out) {
		t.Errorf("タグ名に ' などがあると、変わっていないのに go build し直しました:\n%s", out)
	}
}
