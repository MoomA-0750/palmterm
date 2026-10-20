---
id: 0030
status: done         # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "Makefile"
  - "*_test.go"
after: []           # 先に done になっていてほしい課題の id（例: [0003, 0005]）。それまで run は後回しにする
spec: SPEC.md「全体」（make が画面を作り直す条件）
created: 2026-10-19
---
# web/public のファイルを変えたら、make で画面を作り直す

## 目的
web/public のファイルを変えたら、make で画面を作り直す

## 完了の条件
- [x] 確かめる手順: web/public の中のファイル（例 sw.js）を変えて make → 画面を作り直した palmterm ができる（touch web/index.html は要らない）
- [x] 確かめる手順: 何も変えずにもう一度 make → 作り直さない
- [x] 上を version_test.go と同じ形の Go のテストで確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
- テストで書き換えた sw.js の更新時刻を1分先にずらしたら、make が「時刻が未来」と言って次の make でも作り直した。ずらすのはやめた（ナノ秒の更新時刻で、ずらさなくても前のビルドより新しくなる）
- ファイルを書き換えると git describe に -dirty が付いて版が変わり、それだけで作り直してしまう。テストでは `make palmterm VERSION=test` で版を固定して、web/public の変更だけを見た

## 結果
- 何をしたか: Makefile の palmterm の依存（`$(shell find ...)`）に `web/public` を足した。version_test.go の一時リポジトリを作る部分を `newMakeRepo` に切り出し（`web/public/sw.js` も置く）、`TestMakeRebuildsWhenWebPublicChanges` を足した。SPEC.md「全体」に確定の項目を、docs/decisions.md に理由を書いた（0028 の「touch web/index.html してから make」の記録には、今は要らないと書き添えた）
- どう確かめたか: 新しいテストは元の Makefile では落ち（`'palmterm' is up to date.`）、直した後は通る。実リポジトリで `make` → もう一度 `make` は `'palmterm' is up to date.`、`touch web/public/sw.js` 後の `make -n` は `npm run build` と `go build` を出す。`make test` は Go・単体（165）・通し（45）とも通った（1回目の make test では通しのテストが1件落ちたが、その後の単独3回と make test の再実行ではすべて通った。今回の変更とは関係しない揺れとみている）
- 本人が見るべき点: `web/public` の下にファイルが無い・ディレクトリが無いときは find が標準エラーに出すが、make は続く（今のリポジトリでは常にある）

## レビュー
Codex（2026-10-20、f22b84f..666e881）:

- [中] `Makefile:11` — `find` の結果を依存ファイルにするだけでは、`web/public` のファイル削除を検知できません。削除したファイルは次回の依存一覧から消えるため、`make` が画面を再ビルドせず、削除済みファイルが実行ファイルに残ります。`version_test.go:237` も既存ファイルの上書きしか試しておらず、このケースを検出しません。`SPEC.md:20` の「画面の元のどれかが変わっていれば作り直す」と食い違います。
