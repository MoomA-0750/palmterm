---
id: 0025
status: done         # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "config.example.toml"
spec: SPEC.md「〇〇」
created: 2026-10-17
---
# config.example.toml のテーマの説明を [themes.名前] の書き方に直す

## 目的
config.example.toml のテーマの説明を [themes.名前] の書き方に直す

## 完了の条件
- [x] 確かめる手順: config.example.toml の [theme] には name だけがあり、色の例は [themes.名前] の形（base を含む）でコメントになっている。このファイルをそのまま設定に使って読み込んでも書き間違いが出ない
- [x] make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
- 特になし（動きは変えていないので SPEC.md・docs/decisions.md は直していない）

## 結果
- 何をしたか: config.example.toml の [theme] から色の行（コメント）を外し、[theme] には name だけを残した。色の例は `[themes.my-dark]` の形にして `base = "solarized-dark"` から始め、background・foreground・cursor・accent・16色の書き方をコメントで載せた。最後に「使うときは [theme] を name = "my-dark" にする」と添えた。accent の例は SPEC の例に合わせて #ff5f87 にした
- どう確かめたか:
  - そのまま読む: 既存の `TestConfigExampleReadsAsIs` が通る（書き間違いなし、テーマは default）
  - 例を外して読む: 一時的なテストで `[themes.my-dark]` の行のコメントを外し、name を my-dark にして読み込んだ。書き間違いは出ず、my-dark に base・background・foreground・cursor・accent・red の6項目が入った（このテストは commit していない）
  - `make test` が通る（Go・単体 156 件・通し 40 件）
- 本人が見るべき点: 説明の文の言い回し（日本語と英語）。例のテーマ名 my-dark は SPEC の「実物での確かめ方」と同じにした

## レビュー
Codex（2026-10-17、997c249..8f1401f）:

指摘なし
