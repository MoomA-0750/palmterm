package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
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
	session, err := s.sessionFrom(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	windows, err := listWindows(session)
	if err != nil {
		http.Error(w, "tmux のウィンドウを取れませんでした: "+err.Error(), http.StatusInternalServerError)
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
	session, err := s.sessionFrom(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req struct {
		Action string `json:"action"`
		Window int    `json:"window"` // select-window・kill-window のとき
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		http.Error(w, "操作を読めませんでした: "+err.Error(), http.StatusBadRequest)
		return
	}
	args := tmuxActionArgs(session, req.Action, req.Window)
	if args == nil {
		http.Error(w, "知らない操作です: "+req.Action, http.StatusBadRequest)
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
	case "next-window":
		return []string{"next-window", "-t", session}
	case "previous-window":
		return []string{"previous-window", "-t", session}
	case "last-window":
		return []string{"last-window", "-t", session}
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
