---
id: 0011
status: doing        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "config.go"
  - "config.example.toml"
  - "*_test.go"
  - "web/src/**"
  - "web/index.html"
  - "web/e2e/**"
  - "README*.md"
spec: SPEC.md「〇〇」
created: 2026-10-16
---
# テーマを設定ファイルで変えられるようにする

## 目的
テーマを設定ファイルで変えられるようにする

## 完了の条件
- [ ] 確かめる手順: [theme] の name で組み込みのテーマ（今の色を default として、ほかに明るいテーマを含めていくつか）を選び、読み込み直すと端末とボタンの色が変わる
- [ ] 確かめる手順: [theme] に背景・文字・カーソル・16色・強調色を書くと、そこだけ上書きされる
- [ ] 確かめる手順: 色の書き間違い（#12345 など）は画面を開いたときに知らせ、その色だけ既定に戻す
- [ ] config.example.toml と README に書き方を足す
- [ ] Go のテストと画面の単体テストか通しのテストで上の1〜3を確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
