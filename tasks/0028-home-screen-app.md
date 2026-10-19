---
id: 0028
status: todo        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "web/index.html"
  - "web/public/**"
  - "web/src/**"
  - "web/e2e/**"
  - "main.go"
  - "auth.go"
  - "*_test.go"
  - "README*.md"
spec: SPEC.md「〇〇」
created: 2026-10-19
---
# palmterm をホーム画面に追加して、ブラウザの枠なしで開けるようにする

## 目的
palmterm をホーム画面に追加して、ブラウザの枠なしで開けるようにする

## 完了の条件
- [ ] 確かめる手順: iPhone/iPad の Safari で palmterm を開き「ホーム画面に追加」→ palmterm のアイコンと名前でホーム画面に置かれ、押すとアドレスバーなしの全画面で開く。Android の Chrome でも同じ
- [ ] 確かめる手順: ホーム画面から開いた palmterm でログインして、端末を操作できる。Safari とログインの記録が分かれる場合は、ホーム画面の palmterm の中でログインする手立てを用意する
- [ ] テスト: manifest がアイコンと display: standalone を返し、画面から読まれていることを Go か通しのテストで確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
