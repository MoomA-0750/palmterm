package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
)

const (
	defaultHistoryLines = 3000
	maxHistoryLines     = 50000
)

// 今のペインの履歴を、スクロールバックも含めて普通のテキストで返す（コピーモード用）。
// tmux の中では画面が描き直されるだけでブラウザ側に履歴がたまらないので、tmux から取る。
func (s *server) handleHistory(w http.ResponseWriter, r *http.Request) {
	session, err := s.sessionFrom(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	lines := defaultHistoryLines
	if v, err := strconv.Atoi(r.URL.Query().Get("lines")); err == nil && v > 0 {
		lines = min(v, maxHistoryLines)
	}
	// -J は折り返された行をつなぐ。代わりに行末の空白が残るので後で落とす。
	out, err := exec.Command("tmux", "capture-pane", "-p", "-J", "-S", "-"+strconv.Itoa(lines), "-t", session).Output()
	if err != nil {
		http.Error(w, tr("tmux の履歴を取れませんでした: ", "Could not read the tmux history: ")+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(trimLines(string(out))))
}

func trimLines(s string) string {
	ls := strings.Split(s, "\n")
	for i, l := range ls {
		ls[i] = strings.TrimRight(l, " ")
	}
	return strings.TrimRight(strings.Join(ls, "\n"), "\n") + "\n"
}

type paneState struct {
	AltScreen bool `json:"altScreen"` // 全画面のアプリ（vim・less など）が動いている
	Mouse     bool `json:"mouse"`     // 中のアプリがマウスを受け取っている
	InMode    bool `json:"inMode"`    // tmux のコピーモードなどに入っている
	Width     int  `json:"width"`
}

// 今のペインの状態。スワイプを始めたときに、画面側がスクロールのしかたを決めるのに使う。
// ブラウザからは tmux 自身のモードしか見えず、中のアプリの状態はわからないので tmux に聞く。
func (s *server) handlePane(w http.ResponseWriter, r *http.Request) {
	session, err := s.sessionFrom(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	out, err := exec.Command("tmux", "display-message", "-p", "-t", session,
		"#{alternate_on} #{mouse_any_flag} #{pane_in_mode} #{pane_width}").Output()
	if err != nil {
		http.Error(w, tr("tmux の状態を取れませんでした: ", "Could not get the tmux state: ")+err.Error(), http.StatusInternalServerError)
		return
	}
	f := strings.Fields(string(out))
	if len(f) != 4 {
		http.Error(w, tr("tmux の状態を読めませんでした: ", "Could not parse the tmux state: ")+string(out), http.StatusInternalServerError)
		return
	}
	width, _ := strconv.Atoi(f[3])
	st := paneState{AltScreen: f[0] == "1", Mouse: f[1] == "1", InMode: f[2] != "0", Width: width}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(st)
}
