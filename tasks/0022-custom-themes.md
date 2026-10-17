---
id: 0022
status: doing        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "config*.go"
  - "*_test.go"
  - "web/src/**"
  - "web/e2e/**"
  - "README*.md"
spec: SPEC.md「〇〇」
created: 2026-10-17
---
# 自分のテーマを [themes.名前] に書いて選べるようにし、[theme] には name だけを書く

## 目的
自分のテーマを [themes.名前] に書いて選べるようにし、[theme] には name だけを書く

## 完了の条件
- [ ] 確かめる手順: 設定ファイルに [themes.my-dark] を作り、base = "solarized-dark" と accent = "#ff5f87" を書く。[theme] を name = "my-dark" にして読み込み直すと、Solarized 暗の色のまま、強調の色だけが #ff5f87 になる。base を書かなければ既定のテーマをもとにする
- [ ] 確かめる手順: [theme] に name 以外の項目（accent などの色の行）を書くと、設定ファイルの書き間違いとして知らせ、既定の設定を使う
- [ ] 確かめる手順: [themes.名前] の中の書き間違いは画面を開いたときに知らせる。色の書き間違いはその色だけ base の色に戻す（今の [theme] の色と同じ扱い）。知らない項目・知らない base・組み込みと同じ名前は書き間違いとして扱う
- [ ] 確かめる手順: 設定画面のテーマの行に自分のテーマも並び、押すと読み込み直さずにその色になり、[theme] の name が書き換わる
- [ ] Go のテストと通しのテストで上を確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
