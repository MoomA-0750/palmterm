package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"unicode"
)

type tmuxWindow struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
	Panes  int    `json:"panes"`
	Zoomed bool   `json:"zoomed"`
}

// セッションのウィンドウ一覧（tmux パネルのタブ用）。
func (s *server) handleWindows(w http.ResponseWriter, r *http.Request) {
	session := s.session
	windows, err := listWindows(session)
	if err != nil {
		http.Error(w, tr("tmux のウィンドウを取れませんでした: ", "Could not list the tmux windows: ")+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string][]tmuxWindow{"windows": windows})
}

func listWindows(session string) ([]tmuxWindow, error) {
	out, err := exec.Command("tmux", "list-windows", "-t", session,
		"-F", "#{window_index}\t#{window_active}\t#{window_panes}\t#{window_zoomed_flag}\t#{window_name}").Output()
	if err != nil {
		return nil, err
	}
	windows := []tmuxWindow{}
	for _, l := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		f := strings.SplitN(l, "\t", 5)
		if len(f) != 5 {
			continue
		}
		index, _ := strconv.Atoi(f[0])
		panes, _ := strconv.Atoi(f[2])
		windows = append(windows, tmuxWindow{Index: index, Active: f[1] == "1", Panes: panes, Zoomed: f[3] == "1", Name: f[4]})
	}
	return windows, nil
}

// tmux パネルのボタンから来る操作。プレフィックスキーの割り当てに左右されないよう、tmux のコマンドで直接行う。
// 受け付ける操作は決め打ちにし、任意のコマンドは通さない。
func (s *server) handleTmuxAction(w http.ResponseWriter, r *http.Request) {
	session := s.session
	var req struct {
		Action string `json:"action"`
		Window int    `json:"window"` // select-window・kill-window・rename-window のとき
		Name   string `json:"name"`   // rename-window のとき
		Col    int    `json:"col"`    // select-pane-at のとき（端末の画面での位置、0 から）
		Row    int    `json:"row"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		http.Error(w, tr("操作を読めませんでした: ", "Could not read the request: ")+err.Error(), http.StatusBadRequest)
		return
	}
	var args []string
	if req.Action == "select-pane-at" {
		var err error
		if args, err = selectPaneAtArgs(session, req.Col, req.Row); err != nil {
			http.Error(w, tr("tmux のペインを取れませんでした: ", "Could not list the tmux panes: ")+err.Error(), http.StatusInternalServerError)
			return
		}
		if args == nil { // ペインの境目や、すでに選ばれているペイン
			s.handleWindows(w, r)
			return
		}
	} else if req.Action == "rename-window" {
		args = renameWindowArgs(session, req.Window, req.Name)
	} else {
		args = tmuxActionArgs(session, req.Action, req.Window)
	}
	if args == nil {
		http.Error(w, tr("知らない操作です: ", "Unknown action: ")+req.Action, http.StatusBadRequest)
		return
	}
	if out, err := exec.Command("tmux", args...).CombinedOutput(); err != nil {
		http.Error(w, strings.TrimSpace(string(out)), http.StatusConflict)
		return
	}
	s.handleWindows(w, r)
}

func tmuxActionArgs(session, action string, window int) []string {
	// 新しいウィンドウやペインは、今のペインと同じディレクトリで開く。
	here := []string{"-c", "#{pane_current_path}"}
	switch action {
	case "new-window":
		return append([]string{"new-window", "-t", session}, here...)
	case "select-window":
		return []string{"select-window", "-t", session + ":" + strconv.Itoa(window)}
	case "kill-window":
		return []string{"kill-window", "-t", session + ":" + strconv.Itoa(window)}
	case "split-h":
		return append([]string{"split-window", "-h", "-t", session}, here...)
	case "split-v":
		return append([]string{"split-window", "-v", "-t", session}, here...)
	case "pane-left":
		return []string{"select-pane", "-L", "-t", session}
	case "pane-right":
		return []string{"select-pane", "-R", "-t", session}
	case "pane-up":
		return []string{"select-pane", "-U", "-t", session}
	case "pane-down":
		return []string{"select-pane", "-D", "-t", session}
	case "pane-next":
		return []string{"select-pane", "-t", session + ":.+"}
	case "zoom":
		return []string{"resize-pane", "-Z", "-t", session}
	case "kill-pane":
		return []string{"kill-pane", "-t", session}
	case "copy-mode":
		return []string{"copy-mode", "-t", session}
	}
	return nil
}

const maxWindowNameRunes = 64

// ウィンドウの名前を変える。空なら tmux の自動の名前（動いているコマンド名）に戻す。
func renameWindowArgs(session string, window int, name string) []string {
	target := session + ":" + strconv.Itoa(window)
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(name))
	if name == "" {
		return []string{"set-option", "-w", "-t", target, "automatic-rename", "on"}
	}
	if rs := []rune(name); len(rs) > maxWindowNameRunes {
		name = string(rs[:maxWindowNameRunes])
	}
	// tmux は名前を書式として展開する（#{…} は置き換わり、#(…) はコマンドとして動く）。書いたとおりにするため # を重ねる。
	return []string{"rename-window", "-t", target, strings.ReplaceAll(name, "#", "##")}
}

// 画面のタップした位置（セル）にあるペインを選ぶ。tmux の mouse 設定が off でもタップでペインを移れるように。
// 何もしなくてよいとき（境目・選ばれているペイン・拡大中）は nil を返す。
func selectPaneAtArgs(session string, col, row int) ([]string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "-t", session,
		"#{status} #{status-position} #{window_zoomed_flag}").Output()
	if err != nil {
		return nil, err
	}
	f := strings.Fields(string(out))
	if len(f) != 3 || f[2] == "1" {
		return nil, nil
	}
	// ステータス行が上にあるときは、その分だけペインが下にずれる。
	if f[1] == "top" {
		switch lines, err := strconv.Atoi(f[0]); {
		case err == nil:
			row -= lines
		case f[0] == "on":
			row--
		}
	}
	out, err = exec.Command("tmux", "list-panes", "-t", session,
		"-F", "#{pane_id} #{pane_active} #{pane_left} #{pane_top} #{pane_right} #{pane_bottom}").Output()
	if err != nil {
		return nil, err
	}
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(l)
		if len(f) != 6 {
			continue
		}
		var b [4]int
		for i := range b {
			b[i], _ = strconv.Atoi(f[2+i])
		}
		if col >= b[0] && col <= b[2] && row >= b[1] && row <= b[3] {
			if f[1] == "1" {
				return nil, nil
			}
			return []string{"select-pane", "-t", f[0]}, nil
		}
	}
	return nil, nil
}
