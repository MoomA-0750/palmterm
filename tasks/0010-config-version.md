---
id: 0010
status: doing        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "config.go"
  - "config.example.toml"
  - "*_test.go"
  - "web/src/**"
  - "README*.md"
spec: SPEC.md「〇〇」
created: 2026-10-16
---
# 設定ファイルに版を持たせる

## 目的
設定ファイルに版を持たせる

## 完了の条件
- [ ] 確かめる手順: config.toml に version = 1 と書ける。書いていなければ 1 とみなし、今までの設定ファイルはそのまま読める
- [ ] 確かめる手順: version が palmterm の知らない新しい値なら、読める項目だけ使い、画面を開いたときに「新しい版の設定ファイルです」と知らせる（今の書き間違いの知らせと同じ出し方）
- [ ] 確かめる手順: config.example.toml に version = 1 が入っていて、そのまま読める
- [ ] SPEC.md の「設定ファイル」に方針を書く：好みの設定は ~/.config/palmterm/ の中に置き、このフォルダを写せば別の PC でも同じになる。書き方を変えるときは古い書き方も読み、版を上げる
- [ ] Go のテストで上の1〜3を確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
