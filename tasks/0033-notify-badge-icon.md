---
id: 0033
status: doing        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "web/public/**"
  - "web/src/**"
  - "web/e2e/**"
after: []           # 先に done になっていてほしい課題の id（例: [0003, 0005]）。それまで run は後回しにする
spec: SPEC.md「〇〇」
created: 2026-10-20
---
# Android の通知の小さいアイコンを palmterm の形にする

## 目的
Android の通知の小さいアイコンを palmterm の形にする

## 完了の条件
- [ ] 確かめる手順: Android の palmterm（ホーム画面から開いたもの）で通知をオンにし、画面を消して palmterm notify → ロック画面と通知の一覧の、通知の左の小さいアイコンが白い四角ではなく >_ の形で出る
- [ ] Service Worker が通知の小さいアイコンに、透明な背景に形だけを描いた白黒の画像を指していることを単体テストか通しのテストで確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
