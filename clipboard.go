package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
)

// palmterm のクリップボード：tmux のペーストバッファ（tmux のコピーモードや、Claude Code などがコピーした文字）を
// 新しい順に一覧にして画面に出す。画面の「コピー」のボタンを押せば、スマホのクリップボードに入る
// （ブラウザは、押されていないのにクリップボードへ書くことを許さないため、押してもらう）。
// tmux の中のプログラムが OSC 52 でコピーした文字もバッファに入るよう、tmux の set-clipboard を on にする
// （tmux の既定の external では、中のプログラムの OSC 52 を捨てる）。

const (
	maxClipItems = 50
	maxClipBytes = 1 << 20 // 1つの項目で返す長さ（これより長い所は切る）
)

// OSC 52（クリップボードへの書き込み）の始まり。tmux はバッファにコピーしたとき、つながっている端末にこれを送る。
var osc52 = []byte("\x1b]52;")

type clipItem struct {
	Names     []string `json:"names"` // 同じ中身の tmux のバッファ（消すときは全部消す）
	Created   int64    `json:"created"`
	Text      string   `json:"text"`
	Truncated bool     `json:"truncated,omitempty"`
}

func (s *server) handleClipboard(w http.ResponseWriter, r *http.Request) {
	items, err := listClipboard()
	if err != nil {
		http.Error(w, tr("tmux のバッファを取れませんでした: ", "Could not list the tmux buffers: ")+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string][]clipItem{"items": items})
}

// 一覧から消す（同じ中身のバッファをまとめて消す）。消したあとの一覧を返す。
func (s *server) handleClipboardDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Names []string `json:"names"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil || len(req.Names) == 0 {
		http.Error(w, tr("消すバッファを読めませんでした", "Could not read the buffers to delete"), http.StatusBadRequest)
		return
	}
	for _, name := range req.Names {
		if name == "" || len(name) > 256 {
			continue
		}
		// 他の画面がもう消していることがあるので、失敗しても続ける。
		exec.Command("tmux", "delete-buffer", "-b", name).Run()
	}
	s.handleClipboard(w, r)
}

// tmux のバッファを新しい順に返す。同じ中身は新しい方にまとめる。tmux のサーバーが動いていなければ空。
func listClipboard() ([]clipItem, error) {
	out, err := exec.Command("tmux", "list-buffers", "-F", "#{buffer_created}\t#{buffer_name}").Output()
	if err != nil {
		if exec.Command("tmux", "has-session").Run() != nil {
			return []clipItem{}, nil
		}
		return nil, err
	}
	items := []clipItem{}
	byText := map[string]int{}
	for _, l := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		created, name, ok := strings.Cut(l, "\t")
		if !ok {
			continue
		}
		text, err := exec.Command("tmux", "show-buffer", "-b", name).Output()
		if err != nil {
			continue // 読む間に消された
		}
		truncated := len(text) > maxClipBytes
		if truncated {
			text = bytes.ToValidUTF8(text[:maxClipBytes], nil)
		}
		if i, ok := byText[string(text)]; ok {
			items[i].Names = append(items[i].Names, name)
			continue
		}
		if len(items) == maxClipItems {
			continue
		}
		at, _ := strconv.ParseInt(created, 10, 64)
		byText[string(text)] = len(items)
		items = append(items, clipItem{Names: []string{name}, Created: at, Text: string(text), Truncated: truncated})
	}
	return items, nil
}

// tmux の中のプログラムが OSC 52 でコピーした文字も、バッファに入れるようにする（tmux のサーバーの設定）。
func captureClipboard() {
	exec.Command("tmux", "set-option", "-s", "set-clipboard", "on").Run()
}
