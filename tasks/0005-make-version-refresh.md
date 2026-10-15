---
id: 0005
status: todo        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - Makefile
  - "*_test.go"
  - .gitignore
spec: SPEC.md「全体」（版）
created: 2026-10-15
---
# commit の後に `make` すると版が新しくなる

## 目的
0001 の Codex のレビュー: commit しても `make` が作り直さず、`-version` の版が古いまま残る

## 完了の条件
- [ ] commit を進めて `make` すると、`./palmterm -version` が新しい `git describe` の値を出す（`make -B` は要らない）
- [ ] 版もソースも変わっていなければ、`make` は `go build` をやり直さない
- [ ] 上の2つを、`make -n` の文字列ではなく、一時的な git リポジトリで実際に commit を進めて確かめるテストがあり、`make test` が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
