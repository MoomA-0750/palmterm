package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
)

// 画面で選んだテーマとフォントを設定ファイルに書き戻す。ファイルを丸ごと書き直すのではなく、
// [theme] の name と [font] の file の行だけを書き換えるので、本人が書いたコメント・ほかの項目・並び順は残る。
// 書き換えたあとに読み直して、変わったのがその1か所だけであることを確かめてから保存する。

// 組み込みのテーマの名前（web/src/theme.ts の THEMES と同じ。TestThemeNamesMatchTheScreen で確かめる）。
var themeNames = []string{"default", "light", "solarized-dark", "solarized-light"}

var configWriteMu sync.Mutex

// 画面からの変更。書いた項目だけ変える。Font は fonts/ の直下のファイルの名前で、空なら [font] の file を消す。
type appearanceChange struct {
	Theme *string `json:"theme"`
	Font  *string `json:"font"`
}

func (s *server) handleAppearance(w http.ResponseWriter, r *http.Request) {
	var req appearanceChange
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Theme == nil && req.Font == nil {
		http.Error(w, tr("変える項目がありません", "Nothing to change"), http.StatusBadRequest)
		return
	}
	if req.Theme != nil && !slices.Contains(themeNames, *req.Theme) {
		http.Error(w, fmt.Sprintf(tr("知らないテーマです: %q", "Unknown theme: %q"), *req.Theme), http.StatusBadRequest)
		return
	}
	if req.Font != nil && *req.Font != "" {
		name := *req.Font
		if !uploadNamePattern.MatchString(name) || strings.Contains(name, "..") || !fontUploadExts[strings.ToLower(filepath.Ext(name))] {
			http.Error(w, tr("名前が正しくありません", "Invalid name"), http.StatusBadRequest)
			return
		}
		if info, err := os.Lstat(filepath.Join(fontsDir(), name)); err != nil || !info.Mode().IsRegular() {
			http.Error(w, tr("そのフォントはありません", "No such font"), http.StatusNotFound)
			return
		}
	}

	var edits []configEdit
	if req.Theme != nil {
		edits = append(edits, configEdit{table: "theme", key: "name", value: *req.Theme})
	}
	if req.Font != nil {
		e := configEdit{table: "font", key: "file", value: "fonts/" + *req.Font}
		if *req.Font == "" {
			e = configEdit{table: "font", key: "file", remove: true}
		}
		edits = append(edits, e)
	}
	if err := settings.update(edits); err != nil {
		var bad *configUnreadableError
		if errors.As(err, &bad) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, fmt.Sprintf(tr("設定ファイル（%s）に書けませんでした: %v", "Could not write the config file (%s): %v"), settings.path, err), http.StatusConflict)
		return
	}
	forgetFont()
	writeConfigResponse(w)
}

// 設定ファイルが読めない（書き間違いがある）ので書き戻さなかった。
type configUnreadableError struct{ err error }

func (e *configUnreadableError) Error() string {
	return fmt.Sprintf(tr("設定ファイル（%s）に書き間違いがあるので、書き戻しませんでした。直してからもう一度選んでください: %v",
		"The config file (%s) has a mistake, so it was not changed. Fix it and choose again: %v"), settings.path, e.err)
}

// 設定ファイルの1か所の書き換え。[table] の key を value（文字列）にする。remove なら key の行を消す。
type configEdit struct {
	table, key, value string
	remove            bool
}

// 設定ファイルを書き換える。無ければ作る。読めないファイルは書き換えない。
func (c *configStore) update(edits []configEdit) error {
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	if _, err := c.get(); err != nil {
		return &configUnreadableError{err}
	}
	// 設定ファイルが別の場所へのリンクなら、リンクはそのままにして、リンクの先を書き換える。
	path := c.path
	if p, err := filepath.EvalSymlinks(path); err == nil {
		path = p
	}
	src, err := os.ReadFile(path)
	perm := fs.FileMode(0o600)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		src = []byte(fmt.Sprintf("version = %d\n", configVersion))
	case err != nil:
		return err
	default:
		if info, err := os.Stat(path); err == nil {
			perm = info.Mode().Perm()
		}
	}
	out, err := editConfig(string(src), edits)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// 途中で止まっても壊れたファイルが残らないよう、同じフォルダに書いてから置き換える。
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(out); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	c.mu.Lock()
	c.modTime = time.Time{} // 更新時刻が前と同じに見えても読み直す
	c.mu.Unlock()
	return nil
}

var (
	tableHeader = regexp.MustCompile(`^\s*\[`)
	commentLine = regexp.MustCompile(`^\s*(#.*)?\r?\n?$`)
)

// TOML の文章の [table] の key の行だけを書き換える。書き換えたあと読み直して、変わったのが
// edits の所だけでなければ（インラインのテーブルやドットの付いた書き方などで、行を見つけられなかったとき）断る。
func editConfig(src string, edits []configEdit) (string, error) {
	var before map[string]any
	if _, err := toml.Decode(src, &before); err != nil {
		return "", &configUnreadableError{err}
	}
	out := src
	for _, e := range edits {
		var err error
		if out, err = editLine(out, e); err != nil {
			return "", err
		}
	}
	var after map[string]any
	if _, err := toml.Decode(out, &after); err != nil {
		return "", fmt.Errorf(tr("書き換えると読めなくなるので、書き換えませんでした（[%s] を手で直してください）: %v",
			"Not changed, because the result would not be readable (edit [%s] by hand): %v"), tableNames(edits), err)
	}
	for _, e := range edits {
		t, ok := before[e.table].(map[string]any)
		if !ok {
			if _, exists := before[e.table]; exists {
				return "", fmt.Errorf(tr("%s がテーブルではないので書き換えません", "%s is not a table, so it was not changed"), e.table)
			}
			t = map[string]any{}
			before[e.table] = t
		}
		if e.remove {
			delete(t, e.key)
		} else {
			t[e.key] = e.value
		}
	}
	if !reflect.DeepEqual(before, after) {
		return "", fmt.Errorf(tr("[%s] の書き方を読み取れないので、書き換えませんでした（手で直してください）",
			"Could not find where to write in [%s], so it was not changed (edit it by hand)"), tableNames(edits))
	}
	return out, nil
}

func tableNames(edits []configEdit) string {
	var names []string
	for _, e := range edits {
		if !slices.Contains(names, e.table) {
			names = append(names, e.table)
		}
	}
	return strings.Join(names, "], [")
}

func editLine(src string, e configEdit) (string, error) {
	nl := "\n"
	if strings.Contains(src, "\r\n") {
		nl = "\r\n"
	}
	lines := strings.SplitAfter(src, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	header := regexp.MustCompile(`^\s*\[\s*` + regexp.QuoteMeta(e.table) + `\s*\]\s*(#.*)?\r?\n?$`)
	keyLine := regexp.MustCompile(`^(\s*` + regexp.QuoteMeta(e.key) + `\s*=\s*)("(?:[^"\\\r\n]|\\.)*"|'[^'\r\n]*')(\s*(?:#.*)?\r?\n?)$`)
	keyStart := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(e.key) + `\s*=`)
	newLine := e.key + " = " + tomlString(e.value) + nl

	start := slices.IndexFunc(lines, func(l string) bool { return header.MatchString(l) })
	if start < 0 {
		if e.remove {
			return src, nil
		}
		out := src
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += nl
		}
		if strings.TrimSpace(out) != "" {
			out += nl
		}
		return out + "[" + e.table + "]" + nl + newLine, nil
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if tableHeader.MatchString(lines[i]) {
			end = i
			break
		}
	}
	for i := start + 1; i < end; i++ {
		if !keyStart.MatchString(lines[i]) {
			continue
		}
		if e.remove {
			return strings.Join(slices.Delete(lines, i, i+1), ""), nil
		}
		m := keyLine.FindStringSubmatch(lines[i])
		if m == nil {
			return "", fmt.Errorf(tr("[%s] の %s が1行の文字列ではないので、書き換えませんでした（手で直してください）",
				"[%s] %s is not a one-line string, so it was not changed (edit it by hand)"), e.table, e.key)
		}
		lines[i] = m[1] + tomlString(e.value) + m[3] // 後ろのコメントと改行はそのまま
		return strings.Join(lines, ""), nil
	}
	if e.remove {
		return src, nil
	}
	// テーブルの最後の項目の後ろに足す（次のテーブルの前のコメントや空行より前）。
	at := start + 1
	for i := end - 1; i > start; i-- {
		if !commentLine.MatchString(lines[i]) {
			at = i + 1
			break
		}
	}
	if at > 0 && !strings.HasSuffix(lines[at-1], "\n") {
		lines[at-1] += nl
	}
	lines = slices.Insert(lines, at, newLine)
	return strings.Join(lines, ""), nil
}

// TOML の基本の文字列。値はテーマの名前とフォントのファイル名だけなので、\ と " だけ気にすればよい。
func tomlString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
