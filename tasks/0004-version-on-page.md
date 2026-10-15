---
id: 0004
status: done        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - web/src/**
  - "config.go"
  - "*_test.go"
spec: SPEC.md「全体」
created: 2026-10-15
---
# 画面からも palmterm の版が分かる

## 目的
スマホで開いている画面から、つないでいるサーバーの版を確かめたい

## 完了の条件
- [x] 画面の設定・情報を出すところに、サーバーの版（`palmterm -version` と同じ文字）が出る
- [x] 画面の単体テストで確かめ、`make test` が通る

## 本人に訊くこと
- 何が起きるか: scope（`web/src/**`）の中だけでは、画面がサーバーの版を知る手段がない。版は Go の `main.version`（`-ldflags` で埋め込み）にしかなく、画面に渡す API（`/api/config` など）にも入っていない
- 証拠: `main.go` の `var version` は `-version` の表示にしか使われていない。`config.go` の `handleConfig` の応答は設定・`path`・`error` だけ
- 被害規模と戻せるか: どの案も小さく、commit を戻せば元に戻る
- 決めてほしいこと: scope を広げてよいか。推奨は案A
  - 案A（推奨）: `config.go` の `/api/config` の応答に `version` を足し（Go のテストも1つ足す）、画面は設定パネルに出す。scope に `config.go`・`auth_config_test.go`（か新しいテストファイル）を足す。`palmterm -version` と同じ値を確実に出せる
  - 案B: Makefile から Vite に `VITE_VERSION` を渡して画面に焼き込む。scope に `Makefile`・`web/vite.config.ts` が要る。画面だけ作り直したとき（`npm run dev` など）サーバーの版とずれうるので勧めない
- 本人の答え（2026-10-15 18:22）: 案Aでよい。/api/config の応答に version を足し、画面の設定パネルに出す（scope に config.go, *_test.go を足した）

## メモ
- 試してやめたことはない。案B（Vite に焼き込む）は本人の答えどおり採らなかった

## 結果
- したこと
  - `config.go` の `/api/config` の応答に `version`（`main.version`、`palmterm -version` と同じ値）を足した
  - 画面は tmux ボタンのパネルの一番下に「版」（英語では Version）の行を足し、`palmterm <版>` と出す。応答に版が無ければ（古いサーバー・取得失敗）行を出さない。文字は選んでコピーできる
  - SPEC.md「全体」に項目を足し（仮）、docs/decisions.md に理由を書いた
- 確かめたこと
  - Go のテスト `TestConfigIncludesVersion`（auth_config_test.go）、画面の単体テスト「tmux パネルの版の行」（web/src/tmuxpanel.test.ts）を足した
  - `make test` が通った（Go、画面の単体テスト 112 件、通しのテスト 27 件）
- 本人が見るべき点
  - 置き場所を tmux パネルの一番下の行にした。ほかの所（「その他」の行の中など）がよければ言ってほしい
  - 実機で見るには `make` のあと install して palmterm を再起動する（`make` だけでは動いているサーバーは変わらない）

## レビュー
Codex（2026-10-15、4bc84e9..a0a718f）:

- [中] `tasks/0004-version-on-page.md:3` — 既存課題の状態・結果を直接書き換えています。`AGENTS.md` の「既存の課題を書き換えない」「答えは `task-loop answer` で渡す」に反するため、このファイルの変更は差分から除く必要があります。
