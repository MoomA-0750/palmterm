---
id: 0029
status: done          # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "notify*.go"
  - "push*.go"
  - "devices.go"
  - "logoutall.go"
  - "main.go"
  - "go.mod"
  - "go.sum"
  - "*_test.go"
  - "web/src/**"
  - "web/public/**"
  - "web/index.html"
  - "web/e2e/**"
  - "README*.md"
spec: SPEC.md「画面を閉じている端末への通知（Web Push）」
created: 2026-10-19
after: [0028]
---
# 画面を閉じている端末にも、palmterm notify を Web Push の通知で届ける

## 目的
画面を閉じている端末にも、palmterm notify を Web Push の通知で届ける

## 完了の条件
- [ ] 確かめる手順: 設定画面に「通知」の行があり、押すと通知の許可を求め、許すとその端末で通知が有効になる。iPhone/iPad ではホーム画面から開いたときだけ有効にでき、そうでなければ行に理由を出す
- [ ] 確かめる手順: 通知を有効にした端末で palmterm を閉じる（別のアプリに切り替える・画面を消す）→ PC の tmux で palmterm notify "ビルドが終わった" → その端末に 1:build ビルドが終わった の通知が届く
- [ ] 確かめる手順: 画面がつながっている端末にはプッシュを送らず、今の帯だけを出す
- [ ] 確かめる手順: 通知を押すと palmterm が開き、打ったウィンドウに切り替わる
- [ ] 確かめる手順: 設定画面で通知を切る、端末の一覧で取り消す、palmterm logout-all のどれかをすると、その端末には届かなくなる
- [ ] 確かめる手順: 帯もプッシュもどの端末にも届かないときだけ、palmterm notify は標準エラーにそう出す（終了コードは 0 のまま）
- [ ] テスト: プッシュの送信（暗号化した中身と VAPID の署名）を Go のテストでテスト用の受け口に送って確かめ、make test が通る

## 本人に訊くこと
- task-loop の検査で止めた（2026-10-19 13:13）:
  - テスト（make test）が通らない:
```

[41m[1m FAIL [22m[49m e2e/app.test.ts[2m > [22m大きい画面の板[2m > [22m幅 700px 以上かつ高さ 600px 以上では、板が周りを暗くして中央に浮き、暗い所か Esc で閉じる。設定画面は2列で、右に「ログイン中の端末」・フォント・テーマの一覧が出る
[31m[1mAssertionError[22m: expected 77.8046875 to be greater than or equal to 160[39m
[36m [2m❯[22m expectSide e2e/app.test.ts:[2m1306:20[22m[39m
    [90m1304|[39m     expect(side.x).toBeGreaterThanOrEqual(rows.x + rows.w - 1);
    [90m1305|[39m     expect(side.x + side.w).toBeLessThanOrEqual(settings.x + settings.…
    [90m1306|[39m     expect(side.y).toBeGreaterThanOrEqual(settings.y - 1);
    [90m   |[39m                    [31m^[39m
    [90m1307|[39m     expect(side.y + side.h).toBeLessThanOrEqual(settings.y + settings.…
    [90m1308|[39m     expect(side.w).toBeGreaterThan(300);
[90m [2m❯[22m e2e/app.test.ts:[2m1328:7[22m[39m

[31m[2m⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯[1/1]⎯[22m[39m

make: *** [Makefile:28: test] Error 1
```
  - 実行役の出力: ~/.local/share/task-loop/logs/palmterm/0029-20261019-125308.log
- 本人の答え（2026-10-19 16:46）: 通しのテスト「大きい画面の板 … 設定画面は2列で…」が落ちるのは、通知の行が読み込み後に「…」からボタン・理由に入れ替わって設定画面の高さが変わり、浮いている板が測っている途中で動くためと思われる。行の高さが入れ替えで変わらないようにする（テストを緩めるのではなく）。直したら make test を2回通して確かめる

## メモ
- 「画面がつながっている端末」を WebSocket がつながっているかだけで決めると、iPhone で別のアプリに切り替えた・画面を消した直後は WebSocket が残っていて書き込めてしまい、プッシュが止まる。そこで画面から見えている・隠れたを `POST /api/push/presence` で知らせる形にした（terminal.go は scope の外なので WebSocket の制御メッセージにはしなかった）
- e2e の「通知」の行のテストは、読み込み中の行に前の文字（Turn on）が残っていて、ボタンが隠れたまま通ることがあった。見えていることも待つようにした
- 1回だけ、通しのテスト「大きい画面の板 … 設定画面は2列で…」が落ちた。単独でも、続けて2回の通しでも通ったので、時間の揺れとみている
- （2回目の実行）本人の答えでは通知の行の高さの入れ替わりが原因と見ていたが、画面の高さを毎フレーム記録すると、通知の行が「…」から入れ替わっても板は 446px のままだった。落ちた行（2回目に開いてフォントを右に出したところ）を記録すると、フォントの一覧が `/api/fonts` を読み終えて伸び、板が 446px → 614px（y 161 → 76.8）に変わっていた。落ちたときの値（一覧の y が 77.8）と合う。テストが板・左の列・右の一覧を3回に分けて測るので、その間に伸びると食い違う。確かめる中身はそのままに、3つを1回で（同じ瞬間に）測るようにした。通知の行は変えていない

## 結果
### したこと
- サーバー（push.go）: Web Push の暗号化（RFC 8291 aes128gcm）と VAPID 署名（RFC 8292 ES256）を標準ライブラリだけで実装。鍵は `~/.config/palmterm/vapid.pem`（0600、無ければ作る）。API は `GET /api/push`（公開鍵と有効か）・`POST /api/push/subscribe`・`POST /api/push/unsubscribe`・`POST /api/push/presence`。宛先は devices.json のその端末のところに持つので、取り消し・logout-all で一緒に消える。受け口が 404/410 を返した宛先は消す
- palmterm notify（notify.go）: 帯は今までどおり全画面に送り、加えて「帯が届いて画面も見えている」端末以外の、通知を有効にした端末にプッシュする。帯もプッシュもどこにも届かないときだけ標準エラーに `no palmterm page is open and no device has notifications turned on, …` を出す（終了コード 0）
- 画面: 設定画面に「通知」の行（キーバーとログイン中の端末の間）。「オンにする」で許可を求めて宛先を作る／「オン」「切る」／使えないときは理由（iPhone・iPad でホーム画面から開いていない、HTTPS でない、未対応、許可されていない）。Service Worker（web/public/sw.js、`/sw.js` はログインなしで返す）が通知を出し、押すと開いている palmterm を前に出してウィンドウを切り替えさせるか、`/?window=1&windowId=@3` で開く。画面は見えている・隠れたをサーバーに知らせる（見えている間は1分ごと、サーバーは3分で忘れる）
- 通知の題は `1:build`（ウィンドウが無ければ `palmterm`）、本文は文（空なら設定の言語の「終わりました」、500 文字で切る）
- SPEC.md に「画面を閉じている端末への通知（Web Push）」の節、README（英・日）に使い方、docs/decisions.md に判断を書いた

- （2回目の実行）検査で落ちた通しのテストを直した。原因は通知の行ではなく、フォントの一覧が読み込み後に伸びて板が高くなるのと、テストが位置を別々の時点で測っていたことの競合（「メモ」参照）。`expectSide` が板・左の列・右の一覧を同じ瞬間に測るようにした（確かめる条件は変えていない）

### 確かめたこと
- （2回目の実行）`make test` を続けて2回通した（どちらも Go・単体 165 件・通し 45 件が通る）。直したテストだけを続けて5回走らせても通る
- `make test` が通る（Go・画面の単体テスト 165 件・通しのテスト 45 件）
- Go: `TestPushEncryptsAndSigns`（テスト用の TLS の受け口が端末の秘密鍵で復号し、VAPID の署名をサーバーの公開鍵で確かめる）、`TestNotifyPushesOnlyToDevicesNotShowing`（見えている端末には帯だけ、隠れた・つながっていない・古い「見えている」ならプッシュ）、`TestNotifyPushStopsWhenTurnedOffOrLoggedOut`（切る・取り消す・logout-all で届かなくなり、どこにも届かなければ標準エラーに出る）、`TestPushDropsGoneSubscriptions`、`TestPushSubscribeValidates`、`TestVAPIDKeyIsKept`
- 画面: 「設定画面の通知の行」「通知を使えるか」「通知を押して開いたときのウィンドウ」、通しのテストで PC の Chromium では「Turn on」が出ること、iPhone の UA では理由が出ること、`?window=…&windowId=…` で開くとそのウィンドウに移って URL から消えること

### 本人が見るべき点
- 本人の答えの見立て（通知の行の高さが変わる）とは原因が違った。行の高さは変わっていなかったので、行は直さず、テストの測り方を直した。テストを緩めたのではなく、同じ条件を同じ瞬間の位置で確かめるようにしたもの。なお、2列の設定画面でフォントの一覧を出すと、読み込み後に板が一段高くなる（前からの動き）。気になるなら別の課題で
- **実機では確かめていない**（プッシュの受け口が要るので、テストの環境では本物の Apple・Google の受け口に送れない）。`make install` して palmterm を再起動し、HTTPS（tailscale serve など）でホーム画面に追加した palmterm から、SPEC.md の各項目の「実機での確かめ方」を試してほしい。特に iPhone/iPad で (1) 許可を求められるか (2) 画面を消したときに通知が届くか (3) 通知を押してウィンドウが切り替わるか
- 「画面がつながっている」の決め方を「帯が届いて、かつ画面が見えていると知らせてきている」にした（WebSocket だけだと iPhone で画面を消した直後もつながって見えるため）。隠れた画面にも帯は送るので、戻ると帯と通知センターの通知が両方残る
- VAPID の `sub` はリポジトリの URL（`https://github.com/MoomA-0750/palmterm`）にした。Apple に断られる（通知が来ず、journal に「受け口の応答 403」などが出る）ようなら見直す
- `web/public` を変えたので、make する前に `touch web/index.html` が要る（Makefile は scope の外で、web/public を依存に入れていない。課題 0028 と同じ事情）

## レビュー
Codex（2026-10-19、faf24b1..caaf248）:

指摘なし
