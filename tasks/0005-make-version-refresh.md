---
id: 0005
status: done         # todo / doing / blocked / done
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
- [x] commit を進めて `make` すると、`./palmterm -version` が新しい `git describe` の値を出す（`make -B` は要らない）
- [x] 版もソースも変わっていなければ、`make` は `go build` をやり直さない
- [x] 上の2つを、`make -n` の文字列ではなく、一時的な git リポジトリで実際に commit を進めて確かめるテストがあり、`make test` が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
- テストで web/node_modules の時刻を未来にすると、make が時計のずれとして毎回作り直した。package.json の時刻を過去にする形に直した

## 結果
- 何をしたか: Makefile で、読むたびに `git describe` の値を `.version` と比べ、違うときだけ書き換えるようにした。`palmterm` は `.version` に依存する。`.version` は .gitignore に足した
- どう確かめたか: `TestMakeRebuildsOnlyWhenVersionChanges`（version_test.go）を足した。一時的な git リポジトリに本物の Makefile と小さな main.go を置き（画面のビルドは偽の npm）、make → 変えずに make（go build しない）→ 空の commit → make（go build し、`-version` が新しい describe）→ make（しない）を実際に行う。修正前の Makefile ではこのテストが落ち、修正後は通ることを確かめた。`make test`（Go・画面の単体・通し）も通った
- 本人が見るべき点: `make` のたびに（`make install` や `make -n` でも）`.version` を書き換えうる。版の値に `'` が入ると壊れるが、git describe の値には入らない。`cmp` が無い環境では毎回作り直す（壊れはしない）

## レビュー
Codex（2026-10-15、0bb1c41..c67dd40）:

- 重要: `Makefile:5` — Git のタグ名には `'` を使用できますが、`VERSION` をシングルクォートへ無加工で埋め込んでいます。たとえば `release'one` では比較コマンドが崩れて `.version` が毎回書き換わり、変更がなくても再ビルドされます。`SPEC.md` の「版もファイルも変わっていなければ作り直さない」と完了条件を満たしません。`version_test.go:195` はタグなしの commit しか検証しないため、このケースを検出できません。
