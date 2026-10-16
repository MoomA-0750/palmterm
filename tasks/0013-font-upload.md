---
id: 0013
status: doing        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "config.go"
  - "fonts*.go"
  - "main.go"
  - "*_test.go"
  - "web/src/**"
  - "web/e2e/**"
  - "README*.md"
spec: SPEC.md「〇〇」
created: 2026-10-16
---
# 画面からフォントファイルを足せるようにする

## 目的
画面からフォントファイルを足せるようにする

## 完了の条件
- [ ] 確かめる手順: tmux パネルの「フォント」から .ttf・.otf・.woff・.woff2 を選ぶと、~/.config/palmterm/fonts/ に保存される
- [ ] 確かめる手順: 保存したフォントは、[font] の file = "fonts/〇〇.woff2" で使える
- [ ] 確かめる手順: フォントでないファイルや大きすぎるファイルは保存せず、知らせる。ファイル名に ../ などを入れても、fonts/ の外には書かない
- [ ] 確かめる手順: 画面から保存したフォントの一覧を出し、削除できる（2回押し）
- [ ] Go のテストと通しのテストで上の1〜4を確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
