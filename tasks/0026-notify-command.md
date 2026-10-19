---
id: 0026
status: todo        # todo / doing / blocked / done
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
spec: SPEC.md「〇〇」
created: 2026-10-19
---
# palmterm notify で、開いている palmterm の画面に通知を出せるようにする（タスクの終了などを知らせる）

## 目的
palmterm notify で、開いている palmterm の画面に通知を出せるようにする（タスクの終了などを知らせる）

## 完了の条件
- [ ] 確かめる手順: tmux の中で palmterm notify "ビルドが終わった" を打つ → つながっているすべての palmterm の画面の上に、その文と、打ったウィンドウの番号・名前の帯が出る。帯は × か押すまで残る（コピーの帯のように15秒で消えない）
- [ ] 確かめる手順: 帯を押すと、そのウィンドウに切り替わって帯が消える
- [ ] 確かめる手順: 文を省いて palmterm notify だけを打つと、「終わりました」（英語の設定なら英語）の帯が出る
- [ ] 確かめる手順: palmterm の画面が1つもつながっていない・サーバーが動いていないときは、標準エラーにそう出して終了コード 0 で終わる（Claude Code のフックや make; palmterm notify を止めないため）
- [ ] Go のテストと通しのテストで上を確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
