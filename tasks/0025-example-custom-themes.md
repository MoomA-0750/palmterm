---
id: 0025
status: todo        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "config.example.toml"
spec: SPEC.md「〇〇」
created: 2026-10-17
---
# config.example.toml のテーマの説明を [themes.名前] の書き方に直す

## 目的
config.example.toml のテーマの説明を [themes.名前] の書き方に直す

## 完了の条件
- [ ] 確かめる手順: config.example.toml の [theme] には name だけがあり、色の例は [themes.名前] の形（base を含む）でコメントになっている。このファイルをそのまま設定に使って読み込んでも書き間違いが出ない
- [ ] make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
