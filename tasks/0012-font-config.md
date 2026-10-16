---
id: 0012
status: todo        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "config.go"
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
# フォントを設定ファイルで変えられるようにする

## 目的
フォントを設定ファイルで変えられるようにする

## 完了の条件
- [ ] 確かめる手順: [font] の family に PC に入っているフォントの名前を書くと、palmterm が PC からそのフォントのファイルを探してスマホに配り、端末の文字がそのフォントになる
- [ ] 確かめる手順: PC に無い名前なら、スマホにあればそれを使い、無ければ今の JetBrains Mono に戻る
- [ ] 確かめる手順: [font] の file にフォントファイルのパスを書くと、そのファイルを配って使う。相対パスは ~/.config/palmterm/ から数える（フォルダごと写せば移る）
- [ ] 確かめる手順: Nerd Font のアイコンは、どのフォントにしても今までどおり出る
- [ ] 配るのはログインした端末にだけで、指定したフォント以外の PC のファイルは配らない
- [ ] Go のテストと画面のテストで上の1〜5を確かめ、make test が通る。config.example.toml と README に足す

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
