---
id: 0024
status: blocked      # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "configwrite*.go"
  - "*_test.go"
  - "web/src/**"
  - "README*.md"
spec: SPEC.md「〇〇」
created: 2026-10-17
---
# rose-pine と catppuccin を同梱のテーマに加える

## 目的
rose-pine と catppuccin を同梱のテーマに加える

## 完了の条件
- [ ] 確かめる手順: 設定画面のテーマの選択肢に rose-pine・rose-pine-moon・rose-pine-dawn・catppuccin-latte・catppuccin-frappe・catppuccin-macchiato・catppuccin-mocha の7つが増え、選ぶと端末の文字・背景・16色とボタンなどの色がそれぞれの公式の配色になる（dawn と latte は明るいテーマ）
- [ ] 確かめる手順: 設定ファイルの [theme] に name = "catppuccin-mocha" などと書いて読み込み直しても同じ色になる
- [ ] テスト TestThemeNamesMatchTheScreen と TestAppearanceRejectsUnknownNames が通り、make test が通る

## 本人に訊くこと
- 何が起きるか: 組み込みのテーマを7つ足すと、通しのテスト web/e2e/app.test.ts の「設定画面でテーマと fonts/ のフォントを選ぶと…」が落ちる。テーマの一覧の並びを `["default", "light", "solarized-dark", "solarized-light", "my-dark"]` と、表示名を `["Default", "Light", "Solarized dark", "Solarized light", "my-dark"]` とそのまま比べているため（1001〜1004 行目あたり）。make test は通しのテストも走らせるので、完了の条件の「make test が通る」を満たすにはこのテストの期待値を直す必要がある
- 証拠: この課題の scope は `configwrite*.go`・`*_test.go`・`web/src/**`・`README*.md` で、`*_test.go` は Go のテストだけに当たり、`web/e2e/app.test.ts` は入らない（課題 0023 は scope に `web/e2e/**` があった）
- 被害規模と戻せるか: 直すのは期待値の並びに7つの名前と表示名を足すだけで、git で戻せる。まだ何も実装していない（コードは変えていない）
- 決めてほしいこと: scope に次の glob を足してよいか
  - `web/e2e/**`
- ついでに（任意）: config.example.toml の15行目にも組み込みのテーマの名前が並んでいる。足すなら `config.example.toml` も scope に入れてほしい（入れなければ触らない）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
