---
id: 0026
status: done         # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "main.go"
  - "notify*.go"
  - "open.go"
  - "help.go"
  - "*_test.go"
  - "web/src/**"
  - "web/index.html"
  - "web/e2e/**"
  - "README*.md"
spec: SPEC.md「終わったことを知らせる（`palmterm notify`）」
created: 2026-10-19
---
# palmterm notify で、開いている palmterm の画面に通知を出せるようにする（タスクの終了などを知らせる）

## 目的
palmterm notify で、開いている palmterm の画面に通知を出せるようにする（タスクの終了などを知らせる）

## 完了の条件
- [x] 確かめる手順: tmux の中で palmterm notify "ビルドが終わった" を打つ → つながっているすべての palmterm の画面の上に、その文と、打ったウィンドウの番号・名前の帯が出る。帯は × か押すまで残る（コピーの帯のように15秒で消えない）
- [x] 確かめる手順: 帯を押すと、そのウィンドウに切り替わって帯が消える
- [x] 確かめる手順: 文を省いて palmterm notify だけを打つと、「終わりました」（英語の設定なら英語）の帯が出る
- [x] 確かめる手順: palmterm の画面が1つもつながっていない・サーバーが動いていないときは、標準エラーにそう出して終了コード 0 で終わる（Claude Code のフックや make; palmterm notify を止めないため）
- [x] Go のテストと通しのテストで上を確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
- テストで TMUX に適当な値を入れると、tmux がそれをソケットのパスとして使ってつながらなくなる。テストではペインの中と同じく `#{socket_path},#{pid},0` を入れる
- 受け口は control.sock（logoutall.go、scope の外）に足さず、notify.sock を別に作った（docs/decisions.md）

## 結果
- 何をしたか
  - `palmterm notify [文]` を足した（notify.go）。`TMUX_PANE` から打ったウィンドウの番号・名前を tmux に訊き、実行時のフォルダの `notify.sock` でサーバーに渡す。サーバーはつながっているすべての画面に `{"type":"notify",…}` を送る。届かなくても標準エラーにそう出して終了コード 0
  - 画面は端末の上に帯を新しい順に積む（web/src/notices.ts、5つまで）。`番号:名前` と文を出し、帯を押すとそのウィンドウに切り替えて消え、× で消える。時間では消えない。文が空なら画面の言語で「終わりました」/「Done」
  - SPEC.md に「終わったことを知らせる（`palmterm notify`）」の節（確定）、README に使い方と Claude Code の Stop フックの例、docs/decisions.md に判断を書いた
- どう確かめたか
  - `make test` が通った（Go: notify_test.go の4つ、単体: notices.test.ts、通しのテスト「palmterm notify」で、2つの画面に帯が出る・押すとウィンドウ 1 に移る・× で消える・英語で Done・日本語で「終わりました」・動いていないとき 0 で終わる、を確かめた）
  - 15 秒で消えないことは、帯に時間切れを付けていないこと（コードと単体テスト）で確かめた。通しのテストで 15 秒待つことはしていない
- 本人が見るべき点
  - 実機（スマホ）では見ていない。`make install` して palmterm を再起動し、帯の見た目（強調の色の番号の札＋文、右に ×）を確かめてほしい
  - ほかのセッション（`-session` と違う）で打った知らせはウィンドウを添えず文だけにした
  - 帯は最後に使った画面だけでなく全画面に出し、1つの画面で消してもほかの画面には残る

## レビュー
（task-loop が Codex の指摘を書き足す）
