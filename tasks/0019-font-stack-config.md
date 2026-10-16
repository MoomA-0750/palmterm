---
id: 0019
status: todo        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "config.go"
  - "fonts*.go"
  - "configwrite*.go"
  - "config.example.toml"
  - "*_test.go"
  - "web/src/**"
  - "web/e2e/**"
  - "README*.md"
spec: SPEC.md「〇〇」
created: 2026-10-16
---
# 設定ファイルでフォントを優先順位付きで複数書ける

## 目的
設定ファイルでフォントを優先順位付きで複数書ける

## 完了の条件
- [ ] 確かめる手順: 設定ファイルに version = 2 と [font] の stack = ["fonts/MyMono.woff2", "Noto Sans Mono CJK JP", "JetBrains Mono"] を書いて読み込み直すと、端末の文字は並びの先頭から順に、その文字を持つフォントで描かれる（例: 英字は MyMono、MyMono に無い漢字は Noto）。Nerd のアイコンは今までどおり Nerd Symbols で描く
- [ ] 確かめる手順: 並びの項目は、フォントの拡張子（.ttf・.otf・.ttc・.woff・.woff2）で終わればファイル（相対パスは設定ファイルのフォルダから）、JetBrains Mono は組み込み、それ以外は PC のフォントの名前として探す。PC に無い名前はスマホの同じ名前のフォントを使い、無いファイルは理由を知らせて飛ばす
- [ ] 確かめる手順: 版 1 の書き方（family・file）の設定ファイルも今までどおり読める。stack と family/file を一緒に書いたら書き間違いとして知らせる
- [ ] 確かめる手順: フォントを配るのはログインした端末にだけで、配るのは並びにあるファイルだけ
- [ ] 確かめる手順: 一覧の「使う」は、stack の書き方なら並びをそのフォント1つに書き換える（版 1 の書き方なら今までどおり file を書き換える）
- [ ] Go のテストと通しのテストで上を確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
