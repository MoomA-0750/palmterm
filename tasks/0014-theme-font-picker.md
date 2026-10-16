---
id: 0014
status: doing        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "config.go"
  - "configwrite*.go"
  - "fonts*.go"
  - "main.go"
  - "config.example.toml"
  - "*_test.go"
  - "web/src/**"
  - "web/e2e/**"
  - "README*.md"
spec: SPEC.md「〇〇」
created: 2026-10-16
---
# 画面でテーマとフォントを選べるようにする

## 目的
画面でテーマとフォントを選べるようにする

## 完了の条件
- [ ] 確かめる手順: tmux パネルからテーマ（組み込み）とフォント（fonts/ のもの、名前で指定）を選ぶと、すぐ画面に反映され、config.toml の [theme] の name、[font] が書き換わる
- [ ] 確かめる手順: 書き戻しても、本人が書いたコメント、ほかの項目、並び順は変わらない。ファイルが無ければ作る
- [ ] 確かめる手順: 書き換えたあと、別の端末でページを読み込み直すと同じテーマ・フォントになる
- [ ] 確かめる手順: 設定ファイルに書き間違いがあって読めないときは書き戻さず、知らせる
- [ ] Go のテストと通しのテストで上の1〜4を確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
