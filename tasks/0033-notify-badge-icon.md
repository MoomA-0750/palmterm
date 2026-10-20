---
id: 0033
status: done         # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "web/public/**"
  - "web/src/**"
  - "web/e2e/**"
after: []           # 先に done になっていてほしい課題の id（例: [0003, 0005]）。それまで run は後回しにする
spec: SPEC.md「画面を閉じている端末への通知（Web Push）」
created: 2026-10-20
---
# Android の通知の小さいアイコンを palmterm の形にする

## 目的
Android の通知の小さいアイコンを palmterm の形にする

## 完了の条件
- [ ] 確かめる手順: Android の palmterm（ホーム画面から開いたもの）で通知をオンにし、画面を消して palmterm notify → ロック画面と通知の一覧の、通知の左の小さいアイコンが白い四角ではなく >_ の形で出る
- [x] Service Worker が通知の小さいアイコンに、透明な背景に形だけを描いた白黒の画像を指していることを単体テストか通しのテストで確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
- 試してだめだったことは無い。PNG は ImageMagick で badge.svg から作った（手順は docs/decisions.md）

## 結果
- 何をしたか: 透明な背景に >_ を白で描いた `web/public/icons/badge.svg` と、それを 96px にした `badge-96.png` を足し、Service Worker（`web/public/sw.js`）の通知の `badge` をそれに替えた（大きいアイコン `icon` は icon-192.png のまま）。SPEC.md の「画面を閉じている端末への通知（Web Push）」に項目を足した
- どう確かめたか: 通しのテスト「通知の小さいアイコン（badge）は、透明な背景に形だけを白で描いた画像で、ログインなしでも取れる」を足した（sw.js の badge が `/icons/badge-96.png` を指し、ログインなしで image/png が返り、画素を読むと 96×96・隅が透明・不透明な所はすべて白・形は全面の5〜50%）。`make test` は Go・単体（168件）・通し（47件）とも通った。拡大した画像を目で見て >_ の形になっていることも確かめた
- 本人が見るべき点: 1つめの完了の条件（Android の実機でロック画面・通知の一覧の小さいアイコンが >_ になるか）は実機が無いので確かめていない。`make install` してサービスを入れ替え、Android で palmterm を開き直して（Service Worker が新しい sw.js に替わるまで1度開く必要がある）から試してほしい

## レビュー
Codex（2026-10-20、65a66b8..c40e538）:

- [重要] `tasks/0033-notify-badge-icon.md`: Android 実機で「ロック画面と通知一覧の小さいアイコンが `>_` になる」ことを未確認で、1つ目の完了条件を満たしていません。追加テストも白い非矩形画像であることまでしか検証せず、Androidでの実際の表示は保証できません。
