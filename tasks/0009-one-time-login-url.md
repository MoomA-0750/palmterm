---
id: 0009
status: done        # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "main.go"
  - "auth.go"
  - "devices.go"
  - "loginurl.go"
  - "open.go"
  - "help.go"
  - "*_test.go"
  - "README*.md"
  - "web/e2e/*.ts"
spec: SPEC.md「〇〇」
created: 2026-10-16
---
# ログイン用 URL を使い捨てにする

## 目的
ログイン用 URL を使い捨てにする

## 完了の条件
- [x] 確かめる手順: palmterm url を打つたびに、新しいトークンの入った URL と QR コードが出る
- [x] 確かめる手順: その URL を開くと、端末名の入力欄と「ログイン」ボタンのページが出る（設定の言語で）。入力欄には自動の名前（例「iPhone · Safari」）を薄く出しておく。開いただけではトークンを使わず（リンクのプレビューに使い切られない）、同じ URL をもう一度開いても同じページが出る
- [x] 確かめる手順: 名前を入れてボタンを押すとログインし、端末の画面に移る。「ログイン中の端末」の一覧にはその名前で出る（40 文字まで、前後の空白と制御文字は除く）。空のまま押せば自動の名前になる。そのあと同じ URL を開いてボタンを押しても、ログインできない
- [x] 確かめる手順: 出してから5分たった URL ではログインできない
- [x] 確かめる手順: 起動時の案内（ログ）にも使い捨ての URL を出す
- [x] 確かめる手順: 固定のトークン（token ファイルの中身、-token）ではログインできない。-token はなくなる
- [x] 確かめる手順: 更新前からログインしていた端末は、ログインし直さずにそのまま使える
- [x] 確かめる手順: palmterm を再起動すると、まだ使っていないトークンはすべて無効になる
- [x] Go のテストで上の1〜8を確かめ、make test が通る。SPEC.md の「ログインと安全」と README を、今の動きに書き換える

## 本人に訊くこと
**scope の外（web/e2e）を直さないと、完了の条件「make test が通る」を満たせません。**

- 何が起きるか: `make test` は `go test` のあとに `cd web && npm run test:e2e` を走らせる（Makefile の test）。通しのテスト `web/e2e/app.test.ts` は palmterm を `-token e2etoken` で起動し、`/auth?token=e2etoken` を開いてログインしている（起動待ちの fetch が 303 を待つ所、beforeAll の page.goto、LAN・端末一覧・ほかのブラウザのテストの goto など）。この課題で `-token` をなくし、固定のトークンではログインできなくすると、palmterm が起動しないか、全部のテストがログインできずに落ちる
- 証拠: `web/e2e/app.test.ts` の 92 行目（`"-token", TOKEN`）、95・118・394・600・645・648 行目（`/auth?token=${TOKEN}`）
- 被害規模と戻せるか: コードはまだ何も変えていない（文書もこの課題ファイルだけ）。いつでもやり直せる
- 決めてほしいこと: scope に次を足してよいか
  - `web/e2e/*.ts`
  - 直し方の案: テストの起動時に palmterm のログ（起動時の案内に使い捨ての URL を出す）か、`XDG_RUNTIME_DIR` を分けたうえで `palmterm url -no-qr` を実行して URL を受け取り、その URL を開いて「ログイン」ボタンを押す手順をまとめた関数でログインする（端末ごとに新しい URL を取る）
- ついでに決めてほしいこと（推奨案で進めてよければ答えは要らない）:
  - 「ログイン中の端末」の記録はトークンのファイルの中身で鍵をかけてあり、それで「更新前からログインしていた端末はそのまま使える」を保つ。推奨案は、token ファイルはログインには使わず「端末の記録の鍵」として残す。`-token` と同じ固定のトークンとして `PALMTERM_TOKEN`（環境変数）もなくす。`-token`・`PALMTERM_TOKEN` で動かしていた人は、更新後に一度ログインし直す
  - `palmterm url` は、サーバーの控えのファイル（login-urls）を読む代わりに、logout-all と同じ操作用のソケットで動いているサーバーに新しいトークンを出させる（控えのファイルはもう書かない）
- 本人の答え（2026-10-17 21:13）: web/e2e/*.ts を足してよい。e2e は案のとおり palmterm url で使い捨ての URL を取り、開いて「ログイン」ボタンを押す関数でログインする形に直して。ついでの2点（token ファイルは端末の記録の鍵として残し -token と PALMTERM_TOKEN はなくす／palmterm url は操作用のソケットでサーバーに新しいトークンを出させる）も推奨案で進めてよい（scope に web/e2e/*.ts を足した）

## メモ
- palmterm url の受け口は、logout-all と同じ `control.sock` に命令を足すつもりだったが、その受け口（logoutall.go）が scope の外だったので、同じ形の別のソケット `login.sock` を loginurl.go に置いた

## 結果
### したこと
- ログイン用のトークンを使い捨てにした（loginurl.go の `loginTokens`）。起動時と `palmterm url` のたびに新しく出し、1回使うか5分たつと使えない。メモリにだけ持つので再起動で全部無効
- `GET /auth?token=…` は使い切らずに、名前の欄（自動の名前を placeholder に）と「ログイン」ボタンのページを設定の言語で出す。`POST /auth` で使い切ってログインし `/` に移る。名前は一覧の「付けた名前」になる（前後の空白・制御文字を除き 40 文字まで。名前を整える処理は rename と共通の `cleanLabel` にした）
- `-token` と `PALMTERM_TOKEN` をなくした。token ファイルはログインに使わず、端末の記録の鍵として残した（更新前の端末はそのまま使える）
- `palmterm url` は、実行時のフォルダの `login.sock` でサーバーに新しいトークンを出させる。前の版の控え `login-urls` は起動時に消す
- 起動時のログにも使い捨ての URL を出す（LAN の URL も同じトークン）
- ログインしていないときの 401 の案内を「PC で palmterm url を実行し、出た URL を開く」に変えた
- e2e は `palmterm url -no-qr` で URL を取り、開いて「ログイン」ボタンを押す `login()` でログインする形にした。端末の一覧のテストに、名前の欄の placeholder と、使った URL をもう一度開くと 401 になる確認を足した
- SPEC.md「ログインと安全」、README.md / README.ja.md、docs/decisions.md を書き換えた

### 確かめたこと
- `make test` が通った（Go のテスト、画面の単体テスト 155 件、通しのテスト 40 件）
- 条件ごとの Go のテスト: 1 `TestRunURLIssuesANewTokenEachTime`、2 `TestLoginPageDoesNotUseTheToken`、3 `TestLoginWithNameUsesTheTokenOnce`、4 `TestLoginTokenExpiresAfterFiveMinutes`、5 `TestStartupLogShowsOneTimeLoginURL`、6 `TestFixedTokenCannotLogIn`・`TestHelpLanguage`（-help に -token が無い）、7 `TestDevicesFromBeforeTheUpdateStillWork`、8 `TestRestartInvalidatesUnusedLoginTokens`
- 作った palmterm を一時フォルダで動かして手で確かめた: 起動ログに使い捨ての URL が出る／`palmterm url -no-qr` で URL が出る／その URL の GET は2回とも 200／POST で 303、同じトークンの2回目の POST は 401／token ファイルの中身の URL は 401／止めたあとの `palmterm url` は終了コード 1／`-token x` を付けると終了コード 2 で起動しない

### 本人が見るべき点
- **サービスの env（`~/.config/palmterm/env` の `PALMTERM_ARGS`）に `-token` を書いていたら、更新後は palmterm が起動しない。** 消してから再起動する。`-token` や `PALMTERM_TOKEN` で動かしていた人は、端末の記録の鍵が token ファイルの中身に変わるので、一度ログインし直す
- 実機では未確認。`make install && systemctl --user restart palmterm` のあと、`palmterm url` の QR をスマホで読む → 名前の欄と「ログイン」ボタンのページが出る → 押すと端末の画面になる → 同じ QR をもう一度読むと「使えません」の案内、を見てほしい。チャットのアプリにリンクを貼ってプレビューが出たあとでもログインできるかも
- palmterm url の受け口は `control.sock` ではなく別の `login.sock`（メモのとおり。logoutall.go が scope の外のため）
- ページの見た目（色・大きさ）は最小限の CSS で、画面のテーマには合わせていない

## レビュー
Codex（2026-10-17、af3659d..0df83fe）:

- **重要** — `loginurl.go:128`, `main.go:131-133`: palmterm の二重起動を試すと、稼働中サーバーの `login.sock` を先に削除・置換してから HTTP のアドレス競合で新プロセスが終了します。元のサーバーは動き続けますが、以後 `palmterm url` は接続できず、再起動まで完了条件を満たせません。

- **重要** — `web/scripts/screenshot.mjs:43-52`: 廃止された `-token demo` で起動し、固定 URL を直接開く処理が残っています。`npm run screenshot` はサーバーが不明なオプションで終了するため動きません。

- **軽微** — `SPEC.md:35`, `SPEC.md:53`: 「使い捨てトークン導入前の端末はそのまま使える」と「この仕組みより前の端末はログインし直す」が矛盾して読めます。実装とテストが保証するのは、直前版の `devices.json` に記録された端末だけなので、その範囲を明記する必要があります。
