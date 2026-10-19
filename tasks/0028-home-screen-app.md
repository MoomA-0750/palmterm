---
id: 0028
status: done         # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "web/index.html"
  - "web/public/**"
  - "web/src/**"
  - "web/e2e/**"
  - "main.go"
  - "auth.go"
  - "*_test.go"
  - "README*.md"
spec: SPEC.md「ホーム画面に追加する」
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
- Makefile は scope の外なので `web/public` を palmterm の依存に足していない。`web/public` だけを変えたときは `touch web/index.html` してから make する（docs/decisions.md にも記録）
- Safari のログインをホーム画面のアプリへ渡す仕組み（短いコードなど）は新しい受け口が要るのでやめ、`palmterm url` の URL を貼る形にした

## 結果
- 何をしたか
  - `web/public/manifest.webmanifest`（名前 palmterm、`display: standalone`、`start_url: /`、192/512px のアイコン）と、アイコン（暗い地に `>_`。`web/public/icons/` に元の SVG と 180/192/512px の PNG）を足した
  - `web/index.html` に manifest・`apple-touch-icon`・`apple-mobile-web-app-capable` などを足した
  - manifest と `/icons/` はログインなしで返すようにした（main.go）。ほかはこれまでどおりログインが要る
  - ログインしていないときの `/` を、401 と「ログイン用の URL」を貼る欄のページにした（auth.go）。貼った URL（またはトークンだけ）からトークンを取り出し、今までのログインのページ（名前の欄と「ログイン」）に進む。ホーム画面のアプリが Safari と Cookie が分かれていても、アプリの中でログインできる
- どう確かめたか
  - `make test` が通る（Go・画面の単体テスト 158・通しのテスト 43）
  - Go: `TestHomeScreenManifest`（manifest が standalone とアイコンを返し、アイコンが PNG で取れ、画面と未ログインのページが manifest を読む、ほかはログインが要る）、`TestPasteLoginURL`（URL 丸ごと・空白付き・トークンだけで進め、違う・使用済みは断る）
  - 通しのテスト「ホーム画面のアプリ」: 画面が manifest を指し、Cookie の無い別の記録で `/` を開くと 401 の貼り付けページ → URL を貼って「次へ」→「ログイン」→ 端末に文字を送れる
- 本人が見るべき点
  - 実機の確認はしていない。iPhone/iPad の Safari と Android の Chrome で「ホーム画面に追加」→ アイコン・名前・アドレスバーなしで開くこと、ホーム画面の palmterm で URL を貼ってログインできることを確かめてほしい（install して restart してから）
  - Android の Chrome で枠なしで開くのは正しい証明書の HTTPS（Tailscale）のときだけ。`-lan` の自分で署名した証明書ではタブで開くことがある
  - アイコンの見た目（`web/public/icons/icon-512.png`）が気に入るか
  - SPEC の新しい2項目は実機で確かめていないので「仮」にした

## レビュー
（task-loop が Codex の指摘を書き足す）
