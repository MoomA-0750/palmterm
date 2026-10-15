---
id: 0003
status: doing        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - README.md
  - README.ja.md
spec: SPEC.md「全体」
created: 2026-10-15
---
# README のオプションの表に `-version` が載っている

## 目的
0001 で足す `-version` を README から知れるようにする

## 完了の条件
- [ ] README.ja.md と README.md の「オプション」の表に `-version` の行があり、ほかの行と同じ書き方になっている
- [ ] 確かめる手順: `./palmterm -help` に出るオプションと、両方の README の表のオプションが一致する

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
