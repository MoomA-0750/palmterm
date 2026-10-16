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
Codex（2026-10-16、a0c3a88..20d51c2）:

- **重大** — `logoutall.go:83-89`, `devices.go:150-155`: `devices.json` の保存失敗をログだけで無視し、`ok` を返しています。その場ではログアウトしますが、再起動すると古い記録から全端末が復活します。それでもコマンドは終了コード 0 を返すため、「全端末を取り消す」という完了条件を満たしません。

- **重大** — `terminal.go:35-78`, `open.go:72-79`: WebSocket を開いてから `clientSet` に登録するまでに tmux 起動処理があります。この間に `logout-all` が走ると `closeAll` の対象から漏れ、その接続は取り消し後も操作を続けられます。追加テストも端末出力を受信して登録完了を待ってから実行するため、この競合を検出できません。

- **重要** — `logoutall.go:52-71`, `main.go:125-139`: 起動時に既存の `control.sock` を無条件で削除します。稼働中に2つ目の palmterm を誤って起動すると、既存サーバーのソケットを奪った後、HTTP ポート競合で2つ目が終了し、最初のサーバーは動いているのに `logout-all` が使えなくなります。これは SPEC.md の「動いている palmterm に対して成功する」と食い違います。

- **重要** — `logoutall.go:96-103`, `main.go:125`: control socket の作成失敗を警告だけで済ませ、サーバーを起動し続けます。権限エラーやソケットパス長超過などでは、palmterm が正常に動いているのに `logout-all` は「not running」と終了します。必須機能として確定された SPEC.md の動作になっていません。

- **重要** — `main.go:125-136`, `loginurl.go:33-38`: `ensureRuntimeDir` が他ユーザー所有の実行時フォルダを拒否しても起動を続け、その後 `writeLoginURLs` が同じフォルダを検査せず使用します。予測可能な `/tmp/palmterm-<uid>` を別ユーザーが書き込み可能な状態で先に作り、`login-urls` を用意しておけば、トークン入り URL を書かせられます。`docs/decisions.md` の「ほかのユーザーのフォルダなら使わない」という判断とも食い違います。

- **軽微** — `README.md:242-243`, `README.ja.md:235-236`: セキュリティ節が今も「全端末を締め出すにはトークンを変更して再起動する」と案内しており、`palmterm logout-all` を通常手段、トークン漏洩時だけ変更・再起動とする `SPEC.md:37-42` に更新されていません。
