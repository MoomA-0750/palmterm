---
id: 0024
status: done       # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "configwrite*.go"
  - "*_test.go"
  - "web/src/**"
  - "README*.md"
  - "web/e2e/**"
  - "config.example.toml"
spec: SPEC.md「〇〇」
created: 2026-10-17
---
# rose-pine と catppuccin を同梱のテーマに加える

## 目的
rose-pine と catppuccin を同梱のテーマに加える

## 完了の条件
- [x] 確かめる手順: 設定画面のテーマの選択肢に rose-pine・rose-pine-moon・rose-pine-dawn・catppuccin-latte・catppuccin-frappe・catppuccin-macchiato・catppuccin-mocha の7つが増え、選ぶと端末の文字・背景・16色とボタンなどの色がそれぞれの公式の配色になる（dawn と latte は明るいテーマ）
- [x] 確かめる手順: 設定ファイルの [theme] に name = "catppuccin-mocha" などと書いて読み込み直しても同じ色になる
- [x] テスト TestThemeNamesMatchTheScreen と TestAppearanceRejectsUnknownNames が通り、make test が通る

## 本人に訊くこと
- 何が起きるか: 組み込みのテーマを7つ足すと、通しのテスト web/e2e/app.test.ts の「設定画面でテーマと fonts/ のフォントを選ぶと…」が落ちる。テーマの一覧の並びを `["default", "light", "solarized-dark", "solarized-light", "my-dark"]` と、表示名を `["Default", "Light", "Solarized dark", "Solarized light", "my-dark"]` とそのまま比べているため（1001〜1004 行目あたり）。make test は通しのテストも走らせるので、完了の条件の「make test が通る」を満たすにはこのテストの期待値を直す必要がある
- 証拠: この課題の scope は `configwrite*.go`・`*_test.go`・`web/src/**`・`README*.md` で、`*_test.go` は Go のテストだけに当たり、`web/e2e/app.test.ts` は入らない（課題 0023 は scope に `web/e2e/**` があった）
- 被害規模と戻せるか: 直すのは期待値の並びに7つの名前と表示名を足すだけで、git で戻せる。まだ何も実装していない（コードは変えていない）
- 決めてほしいこと: scope に次の glob を足してよいか
  - `web/e2e/**`
- ついでに（任意）: config.example.toml の15行目にも組み込みのテーマの名前が並んでいる。足すなら `config.example.toml` も scope に入れてほしい（入れなければ触らない）
- 本人の答え（2026-10-17 21:11）: web/e2e/** と config.example.toml を足してよい。example のテーマ名の並びも7つ足して（scope に web/e2e/**, config.example.toml を足した）

## メモ
- 配色は公式の Alacritty のテーマ（rose-pine/alacritty、catppuccin/alacritty）から取った。配色表から自分で端末の16色を割り当てるのはやめた（公式の端末のテーマと割り当てがずれるため。docs/decisions.md）

## 結果
- したこと
  - web/src/theme.ts の `THEMES` に7つのテーマを足した（端末の文字・背景・カーソル・16色は公式の Alacritty のテーマと同じ。ボタン・帯・パネルなどの色は公式の配色の色から選んだ）。dawn と latte は `scheme: "light"`
  - configwrite.go の `themeNames` に同じ7つを足した（画面から選んで書き戻せる）
  - web/src/i18n.ts に表示名（Rosé Pine・Rosé Pine Moon・Rosé Pine Dawn・Catppuccin Latte・Catppuccin Frappé・Catppuccin Macchiato・Catppuccin Mocha。日英とも同じ）
  - README（日英）に組み込みのテーマの表、config.example.toml のテーマ名の並びに7つ、SPEC.md の該当の項目を直した
- 確かめたこと
  - `make test` が通った（Go のテスト、単体テスト 156 件、通しのテスト 40 件）。TestThemeNamesMatchTheScreen・TestAppearanceRejectsUnknownNames も通る
  - 足したテスト: 単体「Rosé Pine と Catppuccin は公式の配色…」（7つの背景・文字・16色の前の8色・明暗）、`TestRosePineAndCatppuccinCanBeChosen`（7つとも画面から選ぶと `[theme]` の name に書き戻る）、通しのテストで設定ファイルに `name = "catppuccin-mocha"` などを書いて読み込み直すと端末の背景・文字・赤とボタンの色が変わり知らせが出ないこと、テーマの一覧に7つが名前つきで並ぶこと
- 本人が見るべき点
  - 実機での見た目（目では見ていない）。とくにボタン・帯・強調の色は公式が決めていないので私の選び（Rosé Pine は強調 foam、dawn は pine、Catppuccin は mauve）。好みでなければ web/src/theme.ts の各テーマの `ui` を直す
  - Rosé Pine のカーソルは公式どおり背景に近い色（#524f67 など）で見えにくいかもしれない
  - 実機で試すときは `make install` して palmterm を再起動してから

## レビュー
（task-loop が Codex の指摘を書き足す）
