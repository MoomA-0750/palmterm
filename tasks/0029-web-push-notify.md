---
id: 0029
status: todo        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "notify*.go"
  - "push*.go"
  - "devices.go"
  - "logoutall.go"
  - "main.go"
  - "go.mod"
  - "go.sum"
  - "*_test.go"
  - "web/src/**"
  - "web/public/**"
  - "web/index.html"
  - "web/e2e/**"
  - "README*.md"
spec: SPEC.md「〇〇」
created: 2026-10-19
---
# 画面を閉じている端末にも、palmterm notify を Web Push の通知で届ける

## 目的
画面を閉じている端末にも、palmterm notify を Web Push の通知で届ける

## 完了の条件
- [ ] 確かめる手順: 設定画面に「通知」の行があり、押すと通知の許可を求め、許すとその端末で通知が有効になる。iPhone/iPad ではホーム画面から開いたときだけ有効にでき、そうでなければ行に理由を出す
- [ ] 確かめる手順: 通知を有効にした端末で palmterm を閉じる（別のアプリに切り替える・画面を消す）→ PC の tmux で palmterm notify "ビルドが終わった" → その端末に 1:build ビルドが終わった の通知が届く
- [ ] 確かめる手順: 画面がつながっている端末にはプッシュを送らず、今の帯だけを出す
- [ ] 確かめる手順: 通知を押すと palmterm が開き、打ったウィンドウに切り替わる
- [ ] 確かめる手順: 設定画面で通知を切る、端末の一覧で取り消す、palmterm logout-all のどれかをすると、その端末には届かなくなる
- [ ] 確かめる手順: 帯もプッシュもどの端末にも届かないときだけ、palmterm notify は標準エラーにそう出す（終了コードは 0 のまま）
- [ ] テスト: プッシュの送信（暗号化した中身と VAPID の署名）を Go のテストでテスト用の受け口に送って確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
