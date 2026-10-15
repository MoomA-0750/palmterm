---
id: 0001
status: doing        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "*.go"
  - Makefile
spec: SPEC.md「全体」
created: 2026-10-15
---
# `palmterm -version` で版が分かる

## 目的
いま動いている palmterm がどの版か、コマンドで確かめたい

## 完了の条件
- [ ] `palmterm -version` が `palmterm <版>` を1行出し、終了コード 0 で終わる。サーバーは起動しない
- [ ] 版はビルド時に `-ldflags "-X main.version=..."` で埋め込める。埋め込まなければ `dev`
- [ ] `make` で作る実行ファイルには `git describe --tags --always --dirty` の値が入る
- [ ] 上の動きを Go のテストで確かめ、`make test` が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
