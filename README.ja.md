# palmterm

[English](README.md) | **日本語**

**スマホから tmux を操作する**ための Web ターミナル。タッチ画面の xterm.js では難しい、文字の選択、
Ctrl や Alt の入力、カーソル移動、スクロールを使いやすくし、vim・lazygit・Claude Code などの TUI も
スマホから扱えるようにする。

<p align="center"><img src="docs/screenshot.png" width="360" alt="スマホで開いた palmterm：tmux のウィンドウのタブ、キーバー、テキストボックス"></p>

- 描画は [wterm](https://github.com/vercel-labs/wterm)。端末を DOM に描くので、OS の文字選択が効く。
- サーバーは画面を埋め込んだ Go の実行ファイル1つ。ブラウザごとに tmux のクライアントを1つ起動して
  同じセッションにつなぐので、PC で作業していた続きをスマホからそのまま触れる。

## できること

### 入力

- **テキストボックス**（下に常に出ている）：OS のキーボードで編集してから送る。カーソル移動・IME・
  予測変換・音声入力がそのまま使える。
  - Enter は改行。送信ボタン、Ctrl+Enter（Mac・iPad では ⌘+Return）、またはキーバーの Ctrl を押してから Enter で送る。
    複数行はブラケットペーストで送るので、Claude Code には複数行のまま入る。
  - 空のときの Backspace は端末側の文字を消す。
- **直接入力**：端末をタップして打つと、1文字ずつすぐ端末に送る（vim や TUI 向け）。
- Tab と画像は、最後に入力していた方に効く。

### キーバー

- 既定は Esc、Tab、Ctrl/Alt/Shift、矢印、Enter、^C、^D、Home/End、PgUp/PgDn といくつかの記号。
  キーと並びは設定ファイルで変えられる（[設定ファイル](#設定ファイル)）。
- **修飾キー**：1回押すと次のキーに1回だけ効く（緑の枠）。すばやく2回押すと固定（緑の塗りつぶし）。
  もう一度押すと解除。ソフトキーボードで打った文字にも効く。
- 矢印と PgUp/PgDn は押し続けると繰り返す。矢印はアプリのカーソルキーのモード（DECCKM）に合わせて送る。
- キーバーを押してもソフトキーボードは出さない。出ているときは閉じない。

### tmux のウィンドウとペイン

- **ウィンドウのタブ**（キーバーの上に常に出す）：タップで切り替え、**＋** で今のディレクトリに新しい
  ウィンドウ、**×** で閉じる（2回押し）。今いるタブをもう一度タップすると名前を変えられる（空にすると
  tmux の自動の名前に戻す）。
- **緑の tmux ボタン**（キーバーの右端に固定）：ペインの分割、拡大、閉じる（2回押し）、ペインの移動、
  tmux のコピーモード、プレフィックス（Ctrl+B）の送信と、「設定」があるパネルを開く。
- **設定画面**（パネルの「設定」）：文字サイズ、フォント、テーマ、言語、キーバーを隠す・出す、ログイン中の端末、
  palmterm の版が並ぶ。「閉じる」で端末に戻る。
  「言語」で English か日本語を選ぶと、設定ファイルの `language` の行を書き換えて読み込み直す（テーマと同じく、
  ほかの行はそのまま。どの端末で開いても同じ言語になる）。
  「ログイン中の端末」の「一覧」で、ログインしている端末の一覧を出し、この端末のログアウトと、
  ほかの端末の取り消し（どちらも2回押し）ができる。なくしたスマホなどは、手元の端末から取り消す。
  端末の名前を押すと、見分けやすい名前（「仕事用の iPad」など）を付けられる。空にすると自動の名前に戻る。
- **ペインをタップ**すると、そのペインに移る。
- これらはサーバーが tmux のコマンドで直接行うので、プレフィックスやキーの割り当てを変えていても、
  tmux の `mouse` が off でも動く。

### スクロールとジェスチャー

- **縦にスワイプ**してスクロールする。スワイプを始めたときのペインの状態で動きを変える。
  - シェルや Claude Code など（マウスも全画面も使っていない）、tmux のコピーモード中：tmux の履歴を
    さかのぼる。一番下まで戻すとコピーモードを抜ける。
  - マウスを使うアプリ（vim・lazygit など）：ホイールを送る。
  - 全画面でマウスを使わないアプリ（less など）：↑↓ を送る。
  - 指を離したあとも、速さに応じて少し続く。
- **マウスのホイールやトラックパッド**でも、スワイプと同じ決め方でスクロールする。
- **ピンチ**で文字サイズを変える（8〜32px）。

### コピーと貼り付け

- **クリップボードのボタン**：palmterm のクリップボード（下記）と tmux の履歴を出す画面を開く。タブで切り替え、
  開き直すと最後に開いていたタブで開く（最初はクリップボード）。
  - 「履歴」のタブ：tmux の履歴（3000 行）を普通のテキストとして出す。長押しで選び、「選択をコピー」か「入力欄へ」。
- 端末の文字は、テキストボックスにフォーカスがある間も、マウスのドラッグや長押しからのドラッグで選べる
  （iPad の Safari 向け）。長押しから選んでいる間は、指でスクロールしない。
- スマホのクリップボードからの貼り付けは、テキストボックスを長押しして行う。
- **palmterm のクリップボード**：tmux の中でコピーした文字（tmux のコピーモード、Claude Code、herdr など
  OSC 52 でコピーするプログラム）は、PC のクリップボードだけでなく、tmux のバッファとしてサーバーに残る。
  - コピーされると、上に「スマホにコピー」の知らせを出す（15 秒）。ブラウザは押されたときしか
    クリップボードへの書き込みを許さないので、押してもらう。
  - コピーの画面の「クリップボード」のタブ（クリップボードのボタンで開く）に、新しい順に並べる。項目ごとに「コピー」「入力欄へ」
    「削除」（2回押し）。
  - tmux の中のプログラムのコピーも受け取れるよう、tmux の `set-clipboard` を `on` にする（tmux の既定の
    `external` では捨てられる）。使わないときは `-capture-clipboard=false`。

### 画像（Claude Code 向け）

- 画像のボタンで写真を選ぶか、テキストボックスや端末に画像を貼り付けると、サーバーに保存する
  （既定は `~/.cache/palmterm/uploads`）。
- Claude Code が読めない形式（HEIC・AVIF など）は、ffmpeg で JPEG に変換する。
- テキストボックスに入力しているときは、その上にサムネイルが並ぶ（× で外せる）。送信すると、
  アップロードが終わるのを待ってから画像のパスを1枚ずつ貼り付け、文章、Enter の順に送る。Claude Code
  では `[Image #1]` のように添付される。アップロードに失敗した画像があれば、何も送らずに知らせる。
- 端末に直接入力しているときは、アップロードが終わりしだいパスを貼り付ける。

### フォント

- JetBrains Mono と Symbols Nerd Font Mono を palmterm が配るので、スマホでも Nerd Font のアイコンが出る。
- WezTerm・kitty・Ghostty と同じく、次のマスが空白のアイコンは2マス分の大きさで描き、そうでなければ
  1マスに収める。Powerline の区切りは1マスいっぱいに引き伸ばす。
- 設定ファイルの `[font]` で端末のフォントを変えられる（[下](#フォント-1)）。アイコンはどのフォントでも今までどおり出る。

### リンクをスマホで開く

- tmux の中のプログラムがブラウザを開こうとすると（たとえば `gh auth login`）、PC でブラウザが開く代わりに、
  palmterm の画面の上にそのリンクと「開く」ボタンを出す。
- palmterm は tmux の環境の `BROWSER` を、付属の `palmterm-open` にする（以後に作るウィンドウ・ペインで効く）。
  リンクは最後に使っていた palmterm の画面に届き、画面がつながっていなければ今までどおり PC のブラウザで開く。
  渡すのは http と https のリンクだけ。使わないときは `-relay-browser=false`。

### 終わったことをスマホに知らせる

- tmux の中で `palmterm notify "ビルドが終わった"` と打つと、つながっているすべての palmterm の画面の上に、その文と
  打ったウィンドウの番号・名前の帯が出る。帯は × か押すまで残り、押すとそのウィンドウに切り替わる。
  文を省くと「終わりました」（英語の設定なら Done）。
- 画面を閉じていても知らせを受けたいときは、その端末で tmux パネルの「設定」→「通知」の「オンにする」を押し、
  通知を許可する。以後、その端末で palmterm を閉じている（別のアプリに切り替えた・画面を消した）ときは、
  OS の通知（Web Push）で届く。通知を押すと palmterm が開いて、打ったウィンドウに切り替わる。
  画面が見えている端末には通知を送らず、帯だけを出す。
  - HTTPS（`tailscale serve` など）か localhost で開いたときだけ使える。iPhone・iPad では、ホーム画面に追加した
    palmterm から開いたときだけ使える（iOS 16.4 以降）。使えないときは「通知」の行に理由が出る
  - 「切る」を押す、「ログイン中の端末」で取り消す、`palmterm logout-all` のどれかで、その端末には届かなくなる
  - 送るときの署名の鍵は `~/.config/palmterm/vapid.pem` に置く（消すと作り直し、各端末で通知をオンにし直す）
- palmterm が動いていない・帯もプッシュもどの端末にも届かないときは、標準エラーにそう出すが、終了コードは 0。
  `make; palmterm notify` や Claude Code のフックに書いても止めない。Claude Code が手を止めたときに知らせるには、
  `~/.claude/settings.json` の `hooks` に `"Stop": [{"hooks": [{"type": "command", "command": "palmterm notify Claude Code が止まりました"}]}]`
  のように書く。

### その他

- 切断されても tmux のセッションは残り、つなぎ直すと続きから表示される。
- 画面の言語は英語と日本語。

## 必要なもの

- Linux（Fedora で確認）など、PTY のある Unix
- tmux 3.x
- ビルドに Go 1.26 以降と Node.js 22 以降
- ffmpeg（任意。HEIC などを Claude Code 向けに変換する）
- Chromium（通しのテストにだけ使う）

## 使い方

```sh
git clone https://github.com/MoomA-0750/palmterm.git
cd palmterm
make          # 画面をビルドして ./palmterm を作る
./palmterm    # 127.0.0.1:7681 で待ち受け、ログイン用の URL を表示する
```

表示された `http://127.0.0.1:7681/auth?token=…` を開くと、端末の名前の欄と「ログイン」ボタンのページが出る。
名前（空なら「iPhone · Safari」のような自動の名前）を入れてボタンを押せば、以後そのブラウザはログインしたままになる。
ログイン用の URL は使い捨てで、1回ログインすると使えなくなり、出してから5分で切れる。
`palmterm url` を打つたびに、動いている palmterm の新しいログイン用の URL（`tailscale serve` が中継していれば
その URL も）と、スマホのカメラで読める QR コードを出せる。
つなぐ tmux のセッションは `main`（なければ作る）。

### スマホから

**Tailscale（おすすめ）**：palmterm は 127.0.0.1 のままにして、Tailscale に tailnet の中だけ HTTPS で
公開させる。

```sh
tailscale serve --bg 7681
```

**LAN**：LAN 向けに HTTPS でも待ち受ける。

```sh
./palmterm -lan :7682
```

この PC の LAN のプライベートな IPv4 アドレス（と Tailscale のアドレス）ごとに待ち受け、`https://<アドレス>:7682/auth?token=…`
を表示する。アドレスを書けば（`-lan 192.168.1.5:7682`）そのアドレスだけで待ち受ける。証明書は自分で署名した
ものを `~/.config/palmterm/` に作る（新しい IP アドレスが出てきたら、前のアドレスも残して作り直す）ので、
ブラウザは最初に警告を出す。「詳細設定」から先へ進む。LAN 側を HTTPS にしているのは、トークンや Cookie を平文で流さないためと、スマホの
クリップボードへのコピーにブラウザのクリップボードの機能を使えるようにするため。

**ホーム画面に追加**：Safari の共有 →「ホーム画面に追加」（Android の Chrome ではメニューの「ホーム画面に追加」）で、
palmterm のアイコンからアドレスバーなしの全画面で開ける。iPhone/iPad のホーム画面の palmterm は Safari とログインの記録が
分かれていることがあり、そのときは「ログイン用の URL」の欄が出る。PC で `palmterm url` を実行し、出た URL をコピーして
欄に貼り、「次へ」→「ログイン」を押す。Android で枠なしで開くには、正しい証明書の HTTPS（Tailscale）で開く必要がある。

### サービスとして動かす

systemd のユーザーサービスのファイル（[contrib/systemd/palmterm.service](contrib/systemd/palmterm.service)）がある。

```sh
make install                                   # ~/.local/bin/palmterm と ~/.config/systemd/user/palmterm.service を置く
echo 'PALMTERM_ARGS="-lan :7682"' > ~/.config/palmterm/env   # 任意：オプションを足す
systemctl --user enable --now palmterm
palmterm url                                   # ログイン用の URL と、スマホで読む QR コード
```

- ログインしなくても PC の起動時に動かすには、一度 `loginctl enable-linger $USER` を実行する。
- tmux のサーバーがまだ動いていなければ、palmterm はそれをサービスの外（`systemd-run --user --scope`）で起動する
  ので、サービスを止めても再起動しても tmux のセッションは残る。
- 更新したら `make install && systemctl --user restart palmterm`。`make uninstall` で2つとも取り除く。

### オプション

`palmterm url [-no-qr]` は新しい使い捨てのログイン用の URL と QR コードを出す。`palmterm logout-all` は動いている palmterm の
全端末のログインを取り消し、開いている接続も切る（端末をなくしたときなど）。`palmterm open URL` は URL を palmterm の画面に渡す
（[リンクをスマホで開く](#リンクをスマホで開く)）。`palmterm notify [文]` は palmterm の画面に知らせの帯を出す
（[終わったことをスマホに知らせる](#終わったことをスマホに知らせる)）。

| オプション | 既定 | |
|---|---|---|
| `-listen` | `127.0.0.1:7681` | 待ち受けるアドレス（HTTP） |
| `-lan` | なし | LAN 向けに HTTPS でも待ち受けるアドレス（例 `:7682`） |
| `-session` | `main` | つなぐ tmux のセッション |
| `-config` | `~/.config/palmterm/config.toml` | 設定ファイル |
| `-upload-dir` | `~/.cache/palmterm/uploads` | アップロードした画像の保存先 |
| `-relay-browser` | `true` | tmux の中のプログラムが開こうとしたリンクを palmterm の画面に出す（上記） |
| `-capture-clipboard` | `true` | tmux の `set-clipboard` を `on` にし、tmux の中のプログラムのコピーも palmterm のクリップボードに入れる（上記） |
| `-allow-origin` | | WebSocket を許す別の Origin（開発用の Vite など） |
| `-version` | | 版（`palmterm <版>`）を出して終わる |

## 設定ファイル

`~/.config/palmterm/config.toml` で、画面の言語とキーバー、色（テーマ）、フォントを決める。書き換えたらページを読み込み直すだけで
反映する（再起動は要らない）。[config.example.toml](config.example.toml) に既定のキーバーがそのまま
書いてあるので、コピーして編集するとよい。

```toml
version = 2            # 設定ファイルの書き方の版（書かなければ 1。版 1 の書き方もそのまま読める）
language = "ja"        # "en"（既定）か "ja"

[[keys]]
key = "esc"

[[keys]]
mod = "ctrl"           # 修飾キー

[[keys]]
key = "shift+tab"      # 組み合わせ
label = "S-Tab"

[[keys]]
text = ":wq\r"         # そのまま送る文字
label = ":wq"
```

`version` は書き方の版。この palmterm より新しい版の設定ファイルは、読める項目だけ使い、画面を開いたときに
知らせる。好みの設定は `~/.config/palmterm/` にまとめて置くので、このフォルダを写せば別の PC でも同じになる。

### 色（テーマ）

`[theme]` の `name` でテーマを選ぶ。組み込みのテーマか、`[themes.名前]` に書いた自分のテーマを選べる。
`[theme]` に書けるのは `name` だけで、色を書くと設定ファイルの書き間違いとして知らせる。

```toml
[theme]
name = "my-dark"       # 組み込みのテーマ（下の表）か自分のテーマ

[themes.my-dark]
base = "solarized-dark" # もとにする組み込みのテーマ（書かなければ "default"）
accent = "#ff5f87"      # ボタンの強調の色
# background = "#002b36" # 端末と画面の背景
# foreground = "#839496" # 端末の文字
# cursor = "#93a1a1"
# red = "#dc322f"        # 端末の16色の1つ
```

組み込みのテーマ:

| name | 色 |
| --- | --- |
| `default` | 既定の暗い色（書かなければこれ） |
| `light` | 明るい色 |
| `solarized-dark`・`solarized-light` | Solarized の暗・明 |
| `rose-pine`・`rose-pine-moon`・`rose-pine-dawn` | Rosé Pine（dawn は明るい） |
| `catppuccin-latte`・`catppuccin-frappe`・`catppuccin-macchiato`・`catppuccin-mocha` | Catppuccin（latte は明るい） |

自分のテーマは `base` の色に、書いた色だけを重ねる。16色は `black` `red` `green` `yellow` `blue`
`magenta` `cyan` `white` と、`bright_` を付けた `bright_black`〜`bright_white`。色は `#rrggbb` か
`#rgb` で書く。`[themes.名前]` の書き間違いは画面を開いたときに知らせる。書き間違えた色（`#12345` など）は
その色だけ `base` の色に戻し、知らない項目・知らない `base`・組み込みと同じ名前も知らせる（同じ名前なら
組み込みのテーマを使う）。`[theme]` の知らないテーマの名前は知らせて `default` を使う。

テーマは画面からも選べる。設定画面の「テーマ」の「一覧」で、テーマごとに名前と色見本（背景の上の文字の色と、端末の16色）が並ぶ
（自分のテーマは組み込みのものの後ろに並ぶ）。行を押すと、すぐ色が変わり、設定ファイルの
`[theme]` の `name` の行を書き換える。一覧は開いたままなので、続けて押して見比べられる（ほかの行・コメント・並び順はそのまま。ファイルが無ければ作る）。
設定ファイルに書き間違いがあって読めないときは書き換えず、理由を知らせる。

### フォント

`[font]` で端末のフォントを変える。`stack` に使うフォントを優先する順に並べると、文字ごとに、並びの先頭から順に
その文字を持つフォントで描く（例: 英字は自分のフォント、それに無い漢字は日本語のフォント）。

```toml
[font]
stack = ["fonts/MyMono.woff2", "Noto Sans Mono CJK JP", "JetBrains Mono"]
```

- .ttf・.otf・.ttc・.woff・.woff2 で終わる項目はフォントのファイルで、palmterm が配る。相対パスは
  `~/.config/palmterm/` から数えるので、フォルダごと写すだけで別の PC でも同じになる。使えないファイルは飛ばし、
  画面を開いたときに理由を知らせる。
- `"JetBrains Mono"` は palmterm の組み込みのフォント。
- それ以外は PC に入っているフォントの名前（fc-list で出る名前）で、palmterm がファイルを探して配る。PC に無ければ、
  スマホに入っている同じ名前のフォントを使う。
- 並びのどれも持たない文字は JetBrains Mono で描く。
- 版 1 の書き方もそのまま使える：`family = "Liberation Mono"`（PC のフォント。無ければスマホのもの）と
  `file = "fonts/MyFont.ttf"`（`family` で探す代わりに配るファイル）。`stack` と `family`・`file` を一緒に書くと
  書き間違いとして知らせる。
- フォントのファイルはスマホからも足せる。設定画面の「フォント」の「一覧」→「追加」で .ttf・.otf・.woff・.woff2
  （32 MB まで）を選ぶと `~/.config/palmterm/fonts/` に保存される。一覧の「使う」を押すと、すぐ端末の文字が
  そのフォントになる。`stack` の書き方なら並びをそのフォント1つ（`stack = ["fonts/名前"]`）に、版 1 の書き方なら
  `file` の行（`file = "fonts/名前"`）を書き換える（テーマと同じく、ほかの行はそのまま）。先頭の「既定のフォント」の
  「使う」は `stack`（か `file`）の行を消す。同じ画面で、保存したフォントを2回押しで消せる。
- 一覧には、足したフォント（印「追加」）のほか、PC に入っているフォント（印「PC」。フォントの名前ごとに1行）と
  組み込みの JetBrains Mono（印「組み込み」）も並び、上の欄に打つと名前で絞り込める。PC と組み込みのフォントの
  「使う」は、`[font]` を `stack = ["その名前"]` にする（版 1 の `family`・`file` の行は消す）。PC のフォントの一覧は
  ログインした端末にだけ出し、名前だけでファイルのパスは出さない。
- 一覧の上の「使う順」の欄で、複数のフォントを並べられる。一覧の各行の「足す」で欄の最後に入り、欄の中は「↑」「↓」で
  順を変え、「外す」で抜ける。変えるたびにすぐ端末の文字に当て、`[font]` の `stack` を欄の順に書き換える
  （版 1 の `family`・`file` の行は消す）。欄を空にすると `stack` の行を消し、JetBrains Mono に戻る。
- 配るのはログインした端末にだけで、配るのは並びにあるフォントのファイルだけ。
- 配るのはフォントごとに普通の太さの1ファイルなので、太字はブラウザが太らせて描く。Nerd Font のアイコンは、どのフォントにしても
  palmterm が配るアイコン用フォントで描く。

### キーバー

`[[keys]]` の1つ1つには、次のどれか1つを書く。

| | |
|---|---|
| `key` | 特殊キーか1文字に、修飾キーを `+` でつないだもの。特殊キー：`esc` `tab` `enter` `backspace` `delete` `insert` `up` `down` `left` `right` `home` `end` `pageup` `pagedown` `f1`〜`f12` `space`。修飾キー：`ctrl`、`alt`（`meta` でも可）、`shift`。例：`ctrl+c`、`alt+left`、`ctrl+space`、`ctrl++` |
| `mod` | `ctrl`・`alt`・`shift`。修飾キーのボタンになる |
| `text` | そのまま送る文字（`\r` や `\u001b` のような TOML のエスケープが使える） |

省略できるもの：`label`、`icon`（`left` `right` `up` `down` `backspace` `enter` `close` `plus`
`splitH` `splitV` `maximize` `cycle` `panes` `textSmaller` `textLarger`）、`repeat`。
キーは書いた順に左から並び、`[[keys]]` を1つでも書くと既定の並びを丸ごと置き換える。書き間違い
（知らない項目・キー・アイコン）は画面を開いたときに知らせ、それ以外はそのまま使える。

## セキュリティ

palmterm にログインした人は、**この PC のシェルを使える**。外に漏らさないこと。

- ログイン用のトークンは使い捨て。起動時と `palmterm url` のたびに新しく出し、1回ログインに使うか、出してから
  5分たつと使えない。サーバーのメモリにだけ持つので、再起動すると、まだ使っていないものもすべて無効になる。
  起動時の案内に URL を出すので journal などのログにも残るが、5分で切れる。
- ログイン用の URL を開いただけではトークンを使わない（チャットのアプリなどのリンクのプレビューで使い切られない）。
  ページの「ログイン」ボタンを押したときに使う。
- ログインすると、その端末（ブラウザ）を記録し、1年間有効な Cookie（HttpOnly、SameSite=Lax、
  HTTPS なら Secure）を入れる。Cookie の中身はトークンではなく、端末ごとのランダムな値。サーバーは
  その値のハッシュだけを `~/.config/palmterm/devices.json`（権限 0600）に持つ。画面・API・WebSocket の
  すべてでこれを確かめ、WebSocket はさらに Origin も確かめる。
- ブラウザは Cookie を同じホスト名のすべてのポートに送るので、同じホストで開いたほかのサービス
  （`127.0.0.1` の別のポートの開発用サーバーなど）にも Cookie が届く。漏れた Cookie では新しい端末で
  ログインできず、その端末の分だけ取り消せる（取り消すと、その端末の開いている接続もすぐ切る）。
- 1台ずつではなく全端末を締め出すには `palmterm logout-all`。
- `~/.config/palmterm/token`（権限 0600。最初の起動で作り、緩ければ起動時に直す）はログインには使わず、
  ログインした端末の記録の鍵にする。このファイルを消して再起動しても、全端末を締め出せる。
- 既定では 127.0.0.1 でしか待ち受けない。`-lan :7682` も、すべてのネットワークではなく、この PC の
  プライベート（RFC 1918）・リンクローカル・Tailscale（100.64.0.0/10）の IPv4 アドレスだけで待ち受ける。
  それ以外で待ち受けるにはアドレスを書く。インターネットには公開しないこと。
- アップロードしたファイルは `X-Content-Type-Options: nosniff` と `Content-Security-Policy: sandbox` を付けて返す。

## 開発

```sh
make dev    # Go のサーバー（7681）と Vite（5173）を起動。表示された URL の 127.0.0.1:7681 を localhost:5173 にして開く
make test   # Go のテスト、画面の単体テスト（vitest）、Chromium での通しのテスト（playwright-core）
```

- tmux を使うテストは、テスト専用の tmux サーバー（`TMUX_TMPDIR` を分け、シェルは `/bin/sh`）で動かすので、
  ふだんのセッションには触れない。通しのテストは `/usr/bin/chromium` を使う（`CHROMIUM=…` で変えられる）。
- `cd web && npm run screenshot` で、同じく切り離した環境で `docs/screenshot.png` を撮り直せる。
- `tools/fit-nerd-symbols.py` は、Symbols Nerd Font Mono から `web/src/fonts/` のアイコン用フォントを作り直す。

## ライセンス

[MIT](LICENSE)

使っているもの：[wterm](https://github.com/vercel-labs/wterm)（Apache-2.0）、
[JetBrains Mono](https://github.com/JetBrains/JetBrainsMono)（OFL-1.1）、
[Nerd Fonts](https://github.com/ryanoasis/nerd-fonts) のシンボル（MIT。`web/src/fonts/SymbolsNerdFont-LICENSE`）、
[coder/websocket](https://github.com/coder/websocket)（ISC）、[creack/pty](https://github.com/creack/pty)（MIT）、
[BurntSushi/toml](https://github.com/BurntSushi/toml)（MIT）。
