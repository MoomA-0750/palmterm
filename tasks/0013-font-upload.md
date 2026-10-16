---
id: 0013
status: done         # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "config.go"
  - "fonts*.go"
  - "main.go"
  - "*_test.go"
  - "web/src/**"
  - "web/e2e/**"
  - "README*.md"
spec: SPEC.md「見た目とフォント」
created: 2026-10-16
---
# 画面からフォントファイルを足せるようにする

## 目的
画面からフォントファイルを足せるようにする

## 完了の条件
- [x] 確かめる手順: tmux パネルの「フォント」から .ttf・.otf・.woff・.woff2 を選ぶと、~/.config/palmterm/fonts/ に保存される
- [x] 確かめる手順: 保存したフォントは、[font] の file = "fonts/〇〇.woff2" で使える
- [x] 確かめる手順: フォントでないファイルや大きすぎるファイルは保存せず、知らせる。ファイル名に ../ などを入れても、fonts/ の外には書かない
- [x] 確かめる手順: 画面から保存したフォントの一覧を出し、削除できる（2回押し）
- [x] Go のテストと通しのテストで上の1〜4を確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
- 試してやめたことは無い。決めたことの理由は docs/decisions.md「画面からフォントファイルを足す（課題 0013）」

## 結果
### 何をしたか
- サーバー（fonts_upload.go、main.go）: `GET /api/fonts`（一覧・保存先・今の `[font]` の file）、`POST /api/fonts`（1ファイルを保存）、`POST /api/fonts/delete`（名前で削除）。どれもログインした端末だけ
  - 保存先は設定ファイルのあるフォルダの `fonts/`（既定 `~/.config/palmterm/fonts/`）
  - 拡張子（.ttf・.otf・.woff・.woff2）と中身の先頭の両方で確かめる。.ttc と 32 MB 超は断り、理由を返す
  - 名前は画像のアップロードと同じ `safeFileName` で直すので、`../` などは捨てられ fonts/ の直下にしか書かない。同じ名前は上書きせず `-1` などを付ける
- 画面（web/src/fontfiles.ts ほか）: tmux パネルの「文字」の行に「フォント」ボタン。開くと一覧（名前・大きさ・`file = "fonts/…"` の行、使用中の印）と「追加」ボタン。削除は2回押し。保存・失敗は知らせ（toast）で出す
  - index.html は scope の外なので、画面は TS で作って body に足した
- SPEC.md「見た目とフォント」に3項目（確定）、README（日英）、docs/decisions.md を更新

### どう確かめたか
- `make test` が通った（Go のテスト、画面の単体テスト 130 件、通しのテスト 32 件）
- Go: `TestUploadedFontIsSavedAndUsable`（保存・番号付け・一覧・`file = "fonts/…"` で配られる）、`TestFontUploadRejectsNonFontsAndStaysInFontsDir`（フォントでない・.ttc・大きすぎ・`../` や絶対パスの名前）、`TestFontDelete`
- 通し: 「tmux パネルの「フォント」から足したフォントは fonts/ に保存され、…」。実際に画面から選んで保存→ `[font]` に書いて読み込み直し→ブラウザが描いたフォント名が足したフォントになる→フォントでない・33 MB のものは断られる→2回押しで消える、までを chromium で確かめた
- 既存の `[font]` の通しのテストの「描いたフォントを訊く」処理を `fontProbe()` に出して使い回した（動きは同じ）

### 本人が見るべき点
- 実機（スマホ）ではまだ試していない。`make install` して palmterm を再起動してから、tmux パネル →「フォント」→「追加」でファイル選択が開き、フォントのファイルを選べるか（iOS の「ファイル」アプリから .ttf などが選べるか）
- 名前は英数字と `. _ -` だけに直す（日本語だけの名前は `file.ttf` になる）。気になれば言ってください
- 上限 32 MB でよいか（日本語フォントの .otf は収まる大きさ）
- 保存しても `[font]` は自動では書き換えない。知らせに出る行を設定ファイルに書く必要がある

## レビュー
Codex（2026-10-16、b4dc2c6..19ff09c）:

- [中] `fonts_upload.go:175` — `foo..bar.ttf` のように名前の途中へ `..` を含むフォントは、`safeFileName` ではそのまま保存されますが、削除 API は `strings.Contains(req.Name, "..")` で拒否します。そのため一覧には出るのに画面から削除できず、完了条件および `SPEC.md` の「保存したフォントを削除できる」を満たしません。アップロード時と削除時で同じ名前検証を使う必要があります。
