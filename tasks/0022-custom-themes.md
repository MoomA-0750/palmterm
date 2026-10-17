---
id: 0022
status: done         # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "config*.go"
  - "*_test.go"
  - "web/src/**"
  - "web/e2e/**"
  - "README*.md"
spec: SPEC.md「設定ファイル」「画面でテーマとフォントを選ぶ」
created: 2026-10-17
---
# 自分のテーマを [themes.名前] に書いて選べるようにし、[theme] には name だけを書く

## 目的
自分のテーマを [themes.名前] に書いて選べるようにし、[theme] には name だけを書く

## 完了の条件
- [x] 確かめる手順: 設定ファイルに [themes.my-dark] を作り、base = "solarized-dark" と accent = "#ff5f87" を書く。[theme] を name = "my-dark" にして読み込み直すと、Solarized 暗の色のまま、強調の色だけが #ff5f87 になる。base を書かなければ既定のテーマをもとにする
- [x] 確かめる手順: [theme] に name 以外の項目（accent などの色の行）を書くと、設定ファイルの書き間違いとして知らせ、既定の設定を使う
- [x] 確かめる手順: [themes.名前] の中の書き間違いは画面を開いたときに知らせる。色の書き間違いはその色だけ base の色に戻す（今の [theme] の色と同じ扱い）。知らない項目・知らない base・組み込みと同じ名前は書き間違いとして扱う
- [x] 確かめる手順: 設定画面のテーマの行に自分のテーマも並び、押すと読み込み直さずにその色になり、[theme] の name が書き換わる
- [x] Go のテストと通しのテストで上を確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
- 自分のテーマを設定画面に「設定ファイルに書いた順」で並べるつもりだったが、サーバーは map で持ち encoding/json がキーを名前の順に並べるので、名前の順になる。SPEC も名前の順と書いた（docs/decisions.md）

## 結果
何をしたか
- 設定ファイル（config.go）: `[theme]` は `name` だけにし、`[themes.名前]` を足した。中身は型を決めずに `/api/config` の `themes` でそのまま画面に渡す。`[theme]` に `name` 以外（色の行など）を書くと「[theme] に書けるのは name だけです。色は [themes.名前] に書き…」と項目の名前を出して書き間違いにし、既定の設定を使う。新しい版の設定ファイルでも `[theme]` の `name` と `[themes]` は読む
- 画面（web/src/theme.ts）: `resolveTheme(name, themes)` で、組み込み → `[themes.名前]` の順に探す。自分のテーマは `base`（書かなければ default）の色に書いた色だけ重ねる。色の書き間違いはその色だけ base の色に戻し、知らない項目・知らない base・組み込みと同じ名前は知らせる（設定全体は既定にしない。同じ名前なら組み込みを使う）。`themeProblems` で、選んでいないテーマの書き間違いも画面を開いたときに知らせる
- 設定画面のテーマの行に、組み込みの後ろに自分のテーマを書いた名前のまま並べた（名前の順）。押すと読み込み直さずに色が変わり、`[theme]` の `name` を書き換える。サーバー（configwrite.go）は今の設定ファイルの `[themes]` にある名前も受け付ける
- SPEC.md・README（日英）・docs/decisions.md を直した

どう確かめたか
- `make test` が通った（Go、画面の単体テスト 152、通しのテスト 40）
- 条件1〜3: 通しのテスト「[theme] の name でテーマを選ぶと…。[theme] に色を書くと書き間違い」で、実際のブラウザで my-dark（base = solarized-dark、accent = #ff5f87）が Solarized 暗の色と同じで tmux ボタンの色だけ rgb(255, 95, 135) になること、base を書かなければ既定の色に accent だけ重なること、書き間違えた色が base の色に戻り、知らない項目・知らない base・組み込みと同じ名前が知らされること、`[theme]` に accent を書くと書き間違いで既定（言語も英語）になることを確かめた。単体テスト（theme.test.ts）と `TestConfigTheme` も
- 条件4: 通しのテスト「設定画面でテーマと fonts/ のフォントを選ぶと…」で、自分のテーマが組み込みの後ろに並び、押すと読み込み直さずに（window の印が残る）色が変わり、設定ファイルの `name` だけが `"my-dark"` になることを確かめた。`TestAppearanceIsWrittenBackKeepingTheRest`・`TestAppearanceRejectsUnknownNames`（無い自分のテーマは断る）も

本人が見るべき点
- 設定ファイルの版は 2 のまま。今 `[theme]` に色を書いている設定ファイルは、読み込み直すと書き間違いの知らせが出て既定の設定になる（完了の条件どおり。方針の「古い書き方も読めるようにして版を上げる」とは食い違う）。手元の config.toml に色の行があれば `[themes.名前]` に移す
- config.example.toml は scope の外なので直していない。コメントに「[theme] に色を書く」古い書き方が残っている（コメントだけなので読めはする）。直すなら別の課題で
- 知らせの文言（i18n.ts の themeUnknownKey・themeUnknownBase・themeSameAsBuiltin、themeBadColor は [themes.名前] を出す形に変えた）
- 実機で見るには `make install` して palmterm を再起動してから

## レビュー
（task-loop が Codex の指摘を書き足す）
