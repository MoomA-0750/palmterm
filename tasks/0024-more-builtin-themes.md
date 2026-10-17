---
id: 0024
status: todo        # todo / doing / blocked / done
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
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
