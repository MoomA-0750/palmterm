---
id: 0002
status: done          # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "*.go"
spec: SPEC.md「全体」（画面の言語）
created: 2026-10-15
---
# `palmterm -help` の説明が、設定の言語で出る

## 目的
設定を日本語にしているなら、-help のオプションの説明も日本語で読みたい

## 完了の条件
- [x] 設定ファイルで `language = "ja"` のとき、`palmterm -help` の各オプションの説明が日本語で出る
- [x] 設定が無い・`en` のときは今と同じ英語のまま
- [x] 上の2つを Go のテストで確かめ、`make test` が通る

## 本人に訊くこと
- task-loop の検査で止めた（2026-10-15 17:49）:
  - scope か「完了の条件」が書き換えられている
  - 実行役の出力: ~/.local/share/task-loop/logs/palmterm/0002-20261015-174618.log
- 本人の答え（2026-10-15 17:50）: 検査の誤り（チェックボックスの [x] を書き換えと見なした）で止めた。実装と make test を確かめ直し、問題なければ done にしてよい

## メモ
- `-help` は `flag.Parse` の途中で出るので、`-config` の値をそのまま使えない。引数を自分で見て探すようにした

## 結果
- 何をしたか: `flag.Usage` を差し替え（help.go）。`-config`（無ければ既定の置き場所）の設定が `language = "ja"` なら、各オプションの説明と既定値の書き添えを日本語で出す。設定が無い・`en`・読めないときは今と同じ Go の flag の英語の出力
- どう確かめたか: `TestHelpLanguage`（help_test.go）で、設定なし・`en`・`ja`・`-config` で ja を指す（`-help` の前と後）の出力を確かめた。`make test` が通った（Go・画面の単体テスト・e2e）
- 本人が見るべき点:
  - 日本語の説明の言い回し（help.go の `flagUsageJa`）
  - 見出しは「<実行ファイル> の使い方:」、既定値は `（既定: "main"）` の形にした
  - 新しいオプションを足して訳を忘れると `TestHelpLanguage` が落ちる（数を比べている）
- 確かめ直し（2026-10-15、検査の誤りで止まった後）: 実装は commit cabc007 のまま変えていない。`go vet ./...` と `make test` が通り（Go・画面の単体テスト 110 件・e2e 27 件）、`TestHelpLanguage` の各場合（設定なし・en・ja・`-config` を前／後）が PASS。`language = "ja"` の設定で作った実行ファイルの `-help` が日本語の説明で出て終了コード 0 なのも目で見た。0002 の commit で触ったのは scope の `*.go` と SPEC.md・docs/decisions.md・この課題ファイルだけ
- 「本人に訊くこと」のログの場所に入っていた個人のパスを `~` に縮めた

## レビュー
Codex（2026-10-15、eb165bc..c4ac814）:

指摘なし
