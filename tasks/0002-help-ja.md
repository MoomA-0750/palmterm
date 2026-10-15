---
id: 0002
status: doing        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "*.go"
spec: SPEC.md「全体」（画面の言語）
created: 2026-10-15
---
# `palmterm -help` の説明が、設定の言語で出る

## 目的
設定を日本語にしているなら、-help のオプションの説明も日本語で読みたい

## 完了の条件
- [ ] 設定ファイルで `language = "ja"` のとき、`palmterm -help` の各オプションの説明が日本語で出る
- [ ] 設定が無い・`en` のときは今と同じ英語のまま
- [ ] 上の2つを Go のテストで確かめ、`make test` が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
