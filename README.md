# palmterm

スマホのブラウザから tmux を操作するための Web ターミナル。
描画は [wterm](https://github.com/vercel-labs/wterm)（DOM に描くので OS の文字選択が効く）、サーバーは Go。

## できること

- **入力欄モード（行）**：下の入力欄で編集してから送る。OS のカーソル移動・IME・予測変換・音声入力がそのまま使える。
  - Enter は改行。送信ボタン、Ctrl+Enter（⌘+Enter）、またはキーバーの Ctrl を押してから Enter で送る。複数行はブラケットペーストで送るので、Claude Code には複数行のまま入る。
  - 空のときの Backspace は端末側の文字を消す。
  - キーバーの Tab は、書きかけの文字を先に送ってから Tab を送る（補完用）。
- **直接入力モード（直、既定）**：1文字ずつすぐ送る（vim や TUI 向け）。端末をタップするとキーボードが出る。
- **キーバー**：Esc / Tab / 矢印 / ^C / ^D / tmux prefix（^B）など。矢印・⌫・PgUp/PgDn は押し続けると繰り返す。
  - Ctrl / Alt / Shift はタップするたびに 切 → 1回（青） → 固定（黄） → 切。
  - 矢印はアプリのカーソルキーのモード（DECCKM）に合わせて送る。
- **コピーモード**：tmux の履歴（3000行）を普通のテキストとして出す。長押しで選んで「選択をコピー」または「入力欄へ」。
- **スワイプ**：全画面のアプリの上で縦にスワイプすると、マウスを受け取るアプリ（tmux の `mouse on`、lazygit など）にはホイール、受け取らないアプリ（less など）には ↑↓ を送る。
- 切断されても tmux のセッションは残り、つなぎ直すと続きから表示される。
- **フォント**：JetBrains Mono ＋ Symbols Nerd Font Mono（Nerd Fonts v3.5.1、MIT）をサーバーから配る。日本語は端末のフォントで表示する。
  - アイコンは WezTerm・kitty・Ghostty と同じく、次のマスが空白なら2マス分の大きさで描き、そうでなければ1マスに収める。Powerline の区切りは1マスいっぱいに引き伸ばす。フォントは `tools/fit-nerd-symbols.py` で作り直したもの。

## 使い方

```sh
make            # web をビルドして ./palmterm を作る（Go と Node が必要）
./palmterm      # 127.0.0.1:7681 で待ち受け。表示される /auth?token=… の URL を開く
```

オプション：`-listen`（待ち受けるアドレス）、`-session`（tmux のセッション名、既定 `main`）、`-token`（省略時は `~/.config/palmterm/token` を使い、なければ作る）。

スマホからは、Tailscale の中だけで公開するのがおすすめ。
`tailscale serve --bg 7681` にすると HTTPS になり、貼付ボタン（clipboard API）も使える。
HTTP のままだと貼付ボタンは使えないので、入力欄を長押しして貼り付ける。

tmux で `set -g mouse on` にしておくと、タップでペインを選べ、スワイプで履歴をさかのぼれる。

## 開発

```sh
make dev   # Go のサーバー（7681）と Vite（5173）を起動する。
           # 最初に http://localhost:5173/auth?token=… を開く
```
