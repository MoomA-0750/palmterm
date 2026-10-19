---
id: 0000
status: todo        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - web/src/**
after: []           # 先に done になっていてほしい課題の id（例: [0003, 0005]）。それまで run は後回しにする
spec: SPEC.md「〇〇」
created: 2026-10-15
---
# 題（何ができるようになるか）

## 目的
（本人の一行メモをそのまま）

## 完了の条件
- [ ] テスト `XxxTest` が通る
- [ ] 確かめる手順: 〇〇を開く → △△する → □□になる

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
