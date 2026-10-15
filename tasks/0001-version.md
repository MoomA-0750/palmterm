---
id: 0001
status: todo         # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "*.go"
  - Makefile
spec: SPEC.md「全体」
created: 2026-10-15
---
# `palmterm -version` で版が分かる

## 目的
いま動いている palmterm がどの版か、コマンドで確かめたい

## 完了の条件
- [x] `palmterm -version` が `palmterm <版>` を1行出し、終了コード 0 で終わる。サーバーは起動しない
- [x] 版はビルド時に `-ldflags "-X main.version=..."` で埋め込める。埋め込まなければ `dev`
- [x] `make` で作る実行ファイルには `git describe --tags --always --dirty` の値が入る
- [x] 上の動きを Go のテストで確かめ、`make test` が通る

## 本人に訊くこと
- task-loop の検査で止めた（2026-10-15 17:46）:
  - scope か「完了の条件」が書き換えられている
  - 実行役の出力: ~/.local/share/task-loop/logs/palmterm/0001-20261015-174417.log
- 本人の答え（2026-10-15 17:50）: 検査の誤り（チェックボックスの [x] を書き換えと見なした）で止めた。実装と make test を確かめ直し、問題なければ done にしてよい

## メモ
- 試してだめだったことはない
- `make` は Go と画面のファイルが変わったときだけ作り直すので、commit しただけでは実行ファイルの版は変わらない（`-dirty` が残ることがある）。作り直すには `make -B`。直すなら版を書いた控えのファイルを依存に足すやり方があるが、`.gitignore` が scope の外なので見送った

## 結果
- 何をしたか
  - main.go: `var version = "dev"` と `-version` フラグを足した。フラグの読み込み直後に `palmterm <版>` を標準出力に出して戻るので、設定・トークン・待ち受けには触れない
  - Makefile: `VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)` を置き、`go build -ldflags "-X main.version=$(VERSION)"` で埋め込む。`make VERSION=v1.0.0` で上書きもできる
  - version_test.go: `TestVersionFlag`（埋め込みあり・なしで実行ファイルを作って `-version` を実行し、出力・終了コード 0・10 秒以内に終わることを確かめる）と `TestMakefileEmbedsGitDescribe`（`make -B -n palmterm` の `go build` に `git describe` の値が入ることを確かめる）
  - SPEC.md「全体」に項目を足し、docs/decisions.md に記録した
- どう確かめたか
  - `make test` が通った（Go のテスト、画面の単体テスト 110 件、通しのテスト 27 件）
  - `make` で作った `./palmterm -version` が `palmterm 27c5236-dirty` を出し、終了コード 0
- 本人が見るべき点
  - SPEC の項目は「仮」で置いた。よければ「確定」に
  - タグがまだ無いので、版は commit のハッシュになる。`v0.1.0` などのタグを打てば `v0.1.0-3-gxxxx` の形になる
  - 上のメモのとおり、commit しただけでは `make` が作り直さないので版が古いまま残ることがある（`make -B` で作り直す）

## レビュー
（task-loop が Codex の指摘を書き足す）
