package main

import (
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
		http.Error(w, "tmux の履歴を取れませんでした: "+err.Error(), http.StatusInternalServerError)
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
