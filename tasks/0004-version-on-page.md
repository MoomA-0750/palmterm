---
id: 0004
status: todo       # todo / doing / blocked / done
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
- [ ] 画面の設定・情報を出すところに、サーバーの版（`palmterm -version` と同じ文字）が出る
- [ ] 画面の単体テストで確かめ、`make test` が通る

## 本人に訊くこと
- 何が起きるか: scope（`web/src/**`）の中だけでは、画面がサーバーの版を知る手段がない。版は Go の `main.version`（`-ldflags` で埋め込み）にしかなく、画面に渡す API（`/api/config` など）にも入っていない
- 証拠: `main.go` の `var version` は `-version` の表示にしか使われていない。`config.go` の `handleConfig` の応答は設定・`path`・`error` だけ
- 被害規模と戻せるか: どの案も小さく、commit を戻せば元に戻る
- 決めてほしいこと: scope を広げてよいか。推奨は案A
  - 案A（推奨）: `config.go` の `/api/config` の応答に `version` を足し（Go のテストも1つ足す）、画面は設定パネルに出す。scope に `config.go`・`auth_config_test.go`（か新しいテストファイル）を足す。`palmterm -version` と同じ値を確実に出せる
  - 案B: Makefile から Vite に `VITE_VERSION` を渡して画面に焼き込む。scope に `Makefile`・`web/vite.config.ts` が要る。画面だけ作り直したとき（`npm run dev` など）サーバーの版とずれうるので勧めない
- 本人の答え（2026-10-15 18:22）: 案Aでよい。/api/config の応答に version を足し、画面の設定パネルに出す（scope に config.go, *_test.go を足した）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
- 実装はしていない（scope の外の変更が要るため blocked）。「本人に訊くこと」を見てほしい

## レビュー
（task-loop が Codex の指摘を書き足す）
