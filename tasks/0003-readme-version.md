---
id: 0003
status: done         # todo / doing / blocked / done
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
- [x] README.ja.md と README.md の「オプション」の表に `-version` の行があり、ほかの行と同じ書き方になっている
- [x] 確かめる手順: `./palmterm -help` に出るオプションと、両方の README の表のオプションが一致する

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
- 特になし

## 結果
- 何をしたか: README.md と README.ja.md の「Options」「オプション」の表の末尾（`-allow-origin` の次）に `-version` の行を足した。英語は「print the version (`palmterm <version>`) and exit」、日本語は「版（`palmterm <版>`）を出して終わる」
- どう確かめたか: `go build` した実行ファイルの `-help` からオプション名を抜き出して並べ、両方の README の表のオプション名と diff した。どちらも一致（`-allow-origin -capture-clipboard -config -lan -listen -relay-browser -session -token -upload-dir -version` の10個）
- 本人が見るべき点: 「既定」の欄は空にした（値を取らないオプションで、既定値を書く意味がないため。`-allow-origin` と同じ空欄の書き方）。`false` と書きたければ直す。SPEC.md は動きを変えていないので触っていない

## レビュー
Codex（2026-10-15、0b13482..2b7173e）:

指摘なし
