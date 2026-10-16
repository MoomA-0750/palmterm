---
id: 0008
status: done         # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "main.go"
  - "auth.go"
  - "devices.go"
  - "open.go"
  - "logoutall*.go"
  - "help.go"
  - "*_test.go"
  - "README*.md"
spec: SPEC.md「ログインと安全」
created: 2026-10-16
---
# コマンドで全端末をログアウトさせる

## 目的
コマンドで全端末をログアウトさせる

## 完了の条件
- [x] 確かめる手順: palmterm が動いているときに palmterm logout-all を打つと、記録された全端末が取り消され、開いている接続もすぐ切れる。取り消した台数を出して、終了コード 0 で終わる
- [x] 確かめる手順: 切られた画面には、今の「取り消す」と同じく、読み込み直してログインし直す方法の案内が出る
- [x] 確かめる手順: palmterm が動いていないときに打つと、動いていないことを知らせて、終了コード 0 以外で終わる
- [x] サーバーとやり取りするソケットは本人しか使えない（実行時のフォルダは 0700）
- [x] Go のテストで上の1〜3を確かめ、make test が通る。SPEC.md の「ログインと安全」、README の使い方に足す

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
- 既存の `open.sock` に命令を足す案は、`-relay-browser=false` のときソケット自体が無いのでやめ、常に置く `control.sock` を別に作った

## 結果
### したこと
- `palmterm logout-all` を足した（logoutall.go）。サーバーは起動時に実行時のフォルダへ `control.sock`（0600）を置き、`logout-all` を受けると全端末の記録を消し（devices.json からも）、開いている接続を「取り消す」と同じ閉じ方（1008）で切り、`ok <台数>` を返す。コマンドは `Logged out N devices.` を出して 0 で終わる。つなげなければ `palmterm is not running (…)` を出して 1
- 実行時のフォルダを用意する所を `ensureRuntimeDir` にまとめ、前から緩い権限で在っても 0700 に直し、ほかのユーザーのフォルダ・フォルダでないものなら使わないようにした（ブラウザの受け渡しもこれを使う）
- SPEC.md「ログインと安全」に確定の項目を足し、README（英・日）のオプションの節に一文足した。docs/decisions.md に理由を書いた
### 確かめたこと
- `TestLogoutAllRevokesEveryDeviceAndClosesConnections`：2台つないだまま実行 → 終了コード 0 で台数 2、両方の WebSocket が 1008 で切れ、両方の Cookie が 401、記録を読み直しても 0 台。フォルダを 0755 で先に作っておいても 0700 に直る。`s.clients.closeAll()` を外すとこのテストが落ちることも確かめた
- `TestLogoutAllWithoutServer`：フォルダが無いときと、前の残りのソケットだけがあるときに、"not running" を出して 0 以外
- `make test` が通る（Go・画面の単体・通しのテスト）
- 実物：一時フォルダで palmterm を起動し、ログイン → `palmterm logout-all` で「Logged out 1 devices.」・終了コード 0、その Cookie は 401。止めた状態では終了コード 1
### 本人が見るべき点
- 画面の案内は「取り消す」と同じ仕組み（1008 で切れたらつなぎ直さず案内）に乗せただけで、画面のコードは変えていない。実機で案内が出るかは見ていない（`make install` して再起動してから試す）
- コマンドの出力は `palmterm url` と同じく英語だけ（設定の言語を見ない）。1台でも `devices` と複数形になる
- 打った本人の PC のブラウザも含め、全端末が締め出される（トークンは変わらないので URL を開けば入り直せる）

## レビュー
（task-loop が Codex の指摘を書き足す）
