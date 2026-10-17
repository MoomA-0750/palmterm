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

// 画面で選んだ言語・テーマ・フォントを設定ファイルに書き戻す。ファイルを丸ごと書き直すのではなく、
// language と [theme] の name と [font] の file・family・stack の行だけを書き換えるので、本人が書いたコメント・ほかの項目・並び順は残る。
// 書き換えたあとに読み直して、変わったのがその1か所だけであることを確かめてから保存する。

// 組み込みのテーマの名前（web/src/theme.ts の THEMES と同じ。TestThemeNamesMatchTheScreen で確かめる）。
var themeNames = []string{"default", "light", "solarized-dark", "solarized-light"}

// 今の設定ファイルに [themes.名前] の自分のテーマがあるか。設定ファイルが読めないときは true にして、
// 名前ではなく「書き間違いがあるので書き戻さない」の方で断る。
func hasCustomTheme(name string) bool {
	cfg, err := settings.get()
	if err != nil {
		return true
	}
	_, ok := cfg.Themes[name]
	return ok
}

var configWriteMu sync.Mutex

// 画面からの変更。書いた項目だけ変える。Font は fonts/ の直下のファイルの名前で、空なら [font] の file を消す。
// SystemFont は PC に入っているフォントか組み込みのフォント（JetBrains Mono）の名前で、[font] の stack をその1つにする。
// Stack は画面の「使う順」の並びで、[font] の stack をその順にする（空なら stack の行を消して既定に戻す）。
type appearanceChange struct {
	Language   *string   `json:"language"`
	Theme      *string   `json:"theme"`
	Font       *string   `json:"font"`
	SystemFont *string   `json:"systemFont"`
	Stack      *[]string `json:"stack"`
}

// 「使う順」に並べられるのは16個まで。
const maxStack = 16

// 「使う順」の1つを確かめ、設定ファイルに書く形にする。fonts/ の直下のフォント、組み込みのフォント、PC のフォントの名前、
// それと今の stack に書いてあるもの（本人が手で書いたパスや、スマホにだけあるフォントの名前）だけを受け付ける。
func stackEntry(name string, current []string) (string, error) {
	if slices.Contains(current, name) {
		return name, nil
	}
	if file, ok := strings.CutPrefix(name, "fonts/"); ok {
		if err := checkUploadedFont(file); err != nil {
			return "", err
		}
		return name, nil
	}
	if strings.EqualFold(name, builtinFont) {
		return builtinFont, nil
	}
	if found, ok := findSystemFont(name); ok {
		return found, nil
	}
	return "", &httpError{http.StatusNotFound, fmt.Sprintf(tr("そのフォントはありません: %q", "No such font: %q"), name)}
}

type httpError struct {
	status int
	msg    string
}

func (e *httpError) Error() string { return e.msg }

// fonts/ の直下にある、画面から足したフォントか。
func checkUploadedFont(name string) error {
	if !uploadNamePattern.MatchString(name) || strings.Contains(name, "..") || !fontUploadExts[strings.ToLower(filepath.Ext(name))] {
		return &httpError{http.StatusBadRequest, tr("名前が正しくありません", "Invalid name")}
	}
	if info, err := os.Lstat(filepath.Join(fontsDir(), name)); err != nil || !info.Mode().IsRegular() {
		return &httpError{http.StatusNotFound, tr("そのフォントはありません", "No such font")}
	}
	return nil
}

func (s *server) handleAppearance(w http.ResponseWriter, r *http.Request) {
	var req appearanceChange
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Language == nil && req.Theme == nil && req.Font == nil && req.SystemFont == nil && req.Stack == nil {
		http.Error(w, tr("変える項目がありません", "Nothing to change"), http.StatusBadRequest)
		return
	}
	if req.Language != nil && *req.Language != "ja" && *req.Language != "en" {
		http.Error(w, fmt.Sprintf(tr("知らない言語です: %q", "Unknown language: %q"), *req.Language), http.StatusBadRequest)
		return
	}
	if req.Theme != nil && !slices.Contains(themeNames, *req.Theme) && !hasCustomTheme(*req.Theme) {
		http.Error(w, fmt.Sprintf(tr("知らないテーマです: %q", "Unknown theme: %q"), *req.Theme), http.StatusBadRequest)
		return
	}
	if req.Font != nil && *req.Font != "" {
		if err := checkUploadedFont(*req.Font); err != nil {
			var he *httpError
			errors.As(err, &he)
			http.Error(w, he.msg, he.status)
			return
		}
	}
	if (req.Font != nil || req.SystemFont != nil) && req.Stack != nil {
		http.Error(w, tr("stack は font・systemFont と一緒に選べません", "stack cannot be chosen together with font or systemFont"), http.StatusBadRequest)
		return
	}
	if req.Stack != nil {
		if len(*req.Stack) > maxStack {
			http.Error(w, fmt.Sprintf(tr("並べられるのは %d 個までです", "At most %d fonts can be listed"), maxStack), http.StatusBadRequest)
			return
		}
		cfg, _ := settings.get()
		var stack []string
		for _, name := range *req.Stack {
			entry, err := stackEntry(name, cfg.Font.Stack)
			if err != nil {
				var he *httpError
				errors.As(err, &he)
				http.Error(w, he.msg, he.status)
				return
			}
			if slices.ContainsFunc(stack, func(s string) bool { return strings.EqualFold(s, entry) }) {
				http.Error(w, fmt.Sprintf(tr("同じフォントが2回あります: %q", "The same font is listed twice: %q"), entry), http.StatusBadRequest)
				return
			}
			stack = append(stack, entry)
		}
		req.Stack = &stack
	}
	if req.SystemFont != nil {
		if req.Font != nil {
			http.Error(w, tr("font と systemFont は一緒に選べません", "font and systemFont cannot be chosen together"), http.StatusBadRequest)
			return
		}
		name, ok := builtinFont, strings.EqualFold(*req.SystemFont, builtinFont)
		if !ok {
			name, ok = findSystemFont(*req.SystemFont)
		}
		if !ok {
			http.Error(w, fmt.Sprintf(tr("そのフォントは PC にありません: %q", "No such font on the PC: %q"), *req.SystemFont), http.StatusNotFound)
			return
		}
		req.SystemFont = &name
	}

	var edits []configEdit
	if req.Language != nil {
		edits = append(edits, configEdit{key: "language", value: *req.Language})
	}
	if req.Theme != nil {
		edits = append(edits, configEdit{table: "theme", key: "name", value: *req.Theme})
	}
	if req.Font != nil {
		e := configEdit{table: "font", key: "file", value: "fonts/" + *req.Font}
		// stack の書き方なら、並びをそのフォント1つにする（既定に戻すなら stack の行を消す）
		if cfg, _ := settings.get(); cfg.Font.Stack != nil {
			e = configEdit{table: "font", key: "stack", value: []string{"fonts/" + *req.Font}}
		}
		if *req.Font == "" {
			e = configEdit{key: e.key, table: "font", remove: true}
		}
		edits = append(edits, e)
	}
	if req.SystemFont != nil {
		// 版 1 の書き方（family・file）は stack と一緒に書けないので、その行を消して stack にする
		if cfg, _ := settings.get(); cfg.Font.Stack == nil {
			edits = append(edits, configEdit{table: "font", key: "file", remove: true}, configEdit{table: "font", key: "family", remove: true})
		}
		edits = append(edits, configEdit{table: "font", key: "stack", value: []string{*req.SystemFont}})
	}
	if req.Stack != nil {
		// 版 1 の書き方（family・file）は stack と一緒に書けないので消す。並びが空なら stack の行も消し、既定（JetBrains Mono）に戻す
		edits = append(edits, configEdit{table: "font", key: "file", remove: true}, configEdit{table: "font", key: "family", remove: true})
		if len(*req.Stack) == 0 {
			edits = append(edits, configEdit{table: "font", key: "stack", remove: true})
		} else {
			edits = append(edits, configEdit{table: "font", key: "stack", value: *req.Stack})
		}
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

// 設定ファイルの1か所の書き換え。[table] の key を value（文字列か、文字列の並び）にする。remove なら key の行を消す。
// table が空なら、最初のテーブルより前に書く項目（language など）。
type configEdit struct {
	table, key string
	value      any // string か []string
	remove     bool
}

// 読み直した TOML と比べるための形（並びは []any になる）。
func (e configEdit) decoded() any {
	if list, ok := e.value.([]string); ok {
		out := make([]any, len(list))
		for i, v := range list {
			out[i] = v
		}
		return out
	}
	return e.value
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
		return "", fmt.Errorf(tr("書き換えると読めなくなるので、書き換えませんでした（%s を手で直してください）: %v",
			"Not changed, because the result would not be readable (edit %s by hand): %v"), tableNames(edits), err)
	}
	for _, e := range edits {
		if e.table == "" {
			if e.remove {
				delete(before, e.key)
			} else {
				before[e.key] = e.decoded()
			}
			continue
		}
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
			t[e.key] = e.decoded()
		}
	}
	if !reflect.DeepEqual(before, after) {
		return "", fmt.Errorf(tr("%s の書き方を読み取れないので、書き換えませんでした（手で直してください）",
			"Could not find where to write %s, so it was not changed (edit it by hand)"), tableNames(edits))
	}
	return out, nil
}

func tableNames(edits []configEdit) string {
	var names []string
	for _, e := range edits {
		name := "[" + e.table + "]"
		if e.table == "" {
			name = e.key
		}
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
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
	if e.table == "" {
		return editTopLine(lines, nl, e)
	}
	header := regexp.MustCompile(`^\s*\[\s*` + regexp.QuoteMeta(e.table) + `\s*\]\s*(#.*)?\r?\n?$`)
	keyStart := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(e.key) + `\s*=`)
	newLine := e.key + " = " + tomlValue(e.value) + nl

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
		return replaceValue(lines, i, e)
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

// 1行の文字列の値（並びなら、1行に書いた文字列の並び）だけを書き換える。後ろのコメントと改行はそのまま。
func replaceValue(lines []string, i int, e configEdit) (string, error) {
	str := `"(?:[^"\\\r\n]|\\.)*"|'[^'\r\n]*'`
	value, ja, en := str, "%s が1行の文字列ではないので", "%s is not a one-line string"
	if _, ok := e.value.([]string); ok {
		value = `\[\s*(?:(?:` + str + `)\s*,\s*)*(?:(?:` + str + `)\s*)?\]`
		ja, en = "%s が1行に書いた文字列の並びではないので", "%s is not a one-line list of strings"
	}
	keyLine := regexp.MustCompile(`^(\s*` + regexp.QuoteMeta(e.key) + `\s*=\s*)(` + value + `)(\s*(?:#.*)?\r?\n?)$`)
	m := keyLine.FindStringSubmatch(lines[i])
	if m == nil {
		name, nameEn := "["+e.table+"] の "+e.key, "["+e.table+"] "+e.key
		if e.table == "" {
			name, nameEn = e.key, e.key
		}
		return "", fmt.Errorf(tr(ja+"、書き換えませんでした（手で直してください）",
			en+", so it was not changed (edit it by hand)"), tr(name, nameEn))
	}
	lines[i] = m[1] + tomlValue(e.value) + m[3]
	return strings.Join(lines, ""), nil
}

// 最初のテーブルより前の key の行を書き換える。無ければ、最初のテーブルより前の最後の項目の後ろに足す
// （項目が無ければファイルの先頭のコメントの後ろ、テーブルの直前のコメントや空行より前）。
func editTopLine(lines []string, nl string, e configEdit) (string, error) {
	keyStart := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(e.key) + `\s*=`)
	end := len(lines)
	for i, l := range lines {
		if tableHeader.MatchString(l) {
			end = i
			break
		}
	}
	for i := 0; i < end; i++ {
		if !keyStart.MatchString(lines[i]) {
			continue
		}
		if e.remove {
			return strings.Join(slices.Delete(lines, i, i+1), ""), nil
		}
		return replaceValue(lines, i, e)
	}
	if e.remove {
		return strings.Join(lines, ""), nil
	}
	at := -1
	for i := end - 1; i >= 0; i-- {
		if !commentLine.MatchString(lines[i]) {
			at = i + 1
			break
		}
	}
	if at < 0 {
		// 項目が無い。ファイルの先頭のコメントのまとまり（空行まで）の後ろに置く。
		at = 0
		for at < end && strings.TrimSpace(lines[at]) != "" {
			at++
		}
		if at == end && end < len(lines) {
			at = 0 // 空行が無く、コメントがテーブルにつながっている
		}
	}
	newLine := e.key + " = " + tomlValue(e.value) + nl
	if at == 0 && len(lines) > 0 && tableHeader.MatchString(lines[0]) {
		newLine += nl // テーブルしか無ければ、見出しとのあいだを1行空ける
	}
	if at > 0 && !strings.HasSuffix(lines[at-1], "\n") {
		lines[at-1] += nl
	}
	lines = slices.Insert(lines, at, newLine)
	return strings.Join(lines, ""), nil
}

// TOML の値。文字列か、文字列の並び（1行に書く）。
func tomlValue(v any) string {
	if list, ok := v.([]string); ok {
		out := make([]string, len(list))
		for i, s := range list {
			out[i] = tomlString(s)
		}
		return "[" + strings.Join(out, ", ") + "]"
	}
	s, _ := v.(string) // 消すときは値が無い
	return tomlString(s)
}

// TOML の基本の文字列。値は言語・テーマの名前とフォントのファイル名だけなので、\ と " だけ気にすればよい。
func tomlString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
