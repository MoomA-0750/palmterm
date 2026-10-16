package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// palmterm logout-all：動いている palmterm に、記録した全端末のログインを取り消させ、開いている接続も切らせる。
// サーバーは実行時のフォルダ（本人だけが入れる）に操作用のソケットを置き、このコマンドはそこにつなぐ。
// トークンを変えて再起動しなくても、なくした端末などを一度に締め出せる。

const (
	controlSocketName = "control.sock"
	logoutAllCommand  = "logout-all"
)

// 実行時のフォルダを用意する。本人のフォルダで、権限が 0700 であることを確かめる（緩ければ直す）。
// ソケットとログイン用の URL の控えを置くので、ほかの人が入れてはいけない。
func ensureRuntimeDir() (string, error) {
	dir := runtimeDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", dir)
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return "", fmt.Errorf("%s is owned by another user", dir)
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// 操作用のソケットで待ち受ける。1行に1つの命令を受け、logout-all なら全端末を取り消して "ok <台数>" を返す。
func (s *server) serveControlSocket(path string) error {
	os.Remove(path) // 前に動いていたときの残り
	ln, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return err
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.handleControlConn(conn)
		}
	}()
	return nil
}

func (s *server) handleControlConn(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	line, err := bufio.NewReader(io.LimitReader(conn, 256)).ReadString('\n')
	if err != nil && line == "" {
		return
	}
	switch strings.TrimSpace(line) {
	case logoutAllCommand:
		n, err := s.devices.removeAll()
		if err != nil {
			log.Printf(tr("ログインした端末の記録を書けませんでした: %v", "Could not save the logged-in devices: %v"), err)
		}
		s.clients.closeAll()
		log.Printf(tr("palmterm logout-all: %d 台の端末のログインを取り消しました", "palmterm logout-all: logged out %d devices"), n)
		fmt.Fprintf(conn, "ok %d\n", n)
	default:
		fmt.Fprintln(conn, "bad")
	}
}

// 操作用のソケットを用意する。使えなくても palmterm 自体は動かす。
func (s *server) setupControlSocket() {
	dir, err := ensureRuntimeDir()
	if err == nil {
		err = s.serveControlSocket(filepath.Join(dir, controlSocketName))
	}
	if err != nil {
		log.Printf(tr("palmterm logout-all の受け口を用意できませんでした: %v", "Could not set up the socket for palmterm logout-all: %v"), err)
	}
}

// palmterm logout-all を実行する。取り消した台数を出して 0、palmterm が動いていなければ 1 を返す。
func runLogoutAll(out, errOut io.Writer, args []string) int {
	if len(args) > 0 {
		fmt.Fprintln(errOut, "usage: palmterm logout-all")
		return 2
	}
	sock := filepath.Join(runtimeDir(), controlSocketName)
	conn, err := net.DialTimeout("unix", sock, time.Second)
	if err != nil {
		fmt.Fprintln(errOut, "palmterm is not running (could not connect to "+sock+").")
		return 1
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	fmt.Fprintln(conn, logoutAllCommand)
	reply, _ := bufio.NewReader(conn).ReadString('\n')
	n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(reply), "ok "))
	if !strings.HasPrefix(reply, "ok ") || err != nil {
		fmt.Fprintf(errOut, "palmterm did not log out the devices (reply: %q).\n", strings.TrimSpace(reply))
		return 1
	}
	fmt.Fprintf(out, "Logged out %d devices.\n", n)
	return 0
}
