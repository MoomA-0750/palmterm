---
id: 0004
status: todo        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - web/src/**
spec: SPEC.md「全体」
created: 2026-10-15
---
# 画面からも palmterm の版が分かる

## 目的
スマホで開いている画面から、つないでいるサーバーの版を確かめたい

## 完了の条件
- [ ] 画面の設定・情報を出すところに、サーバーの版（`palmterm -version` と同じ文字）が出る
- [ ] 画面の単体テストで確かめ、`make test` が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
