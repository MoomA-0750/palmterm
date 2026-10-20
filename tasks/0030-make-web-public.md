---
id: 0030
status: doing        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "Makefile"
  - "*_test.go"
after: []           # 先に done になっていてほしい課題の id（例: [0003, 0005]）。それまで run は後回しにする
spec: SPEC.md「〇〇」
created: 2026-10-19
---
# web/public のファイルを変えたら、make で画面を作り直す

## 目的
web/public のファイルを変えたら、make で画面を作り直す

## 完了の条件
- [ ] 確かめる手順: web/public の中のファイル（例 sw.js）を変えて make → 画面を作り直した palmterm ができる（touch web/index.html は要らない）
- [ ] 確かめる手順: 何も変えずにもう一度 make → 作り直さない
- [ ] 上を version_test.go と同じ形の Go のテストで確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
