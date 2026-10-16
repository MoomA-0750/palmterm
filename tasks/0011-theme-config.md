---
id: 0011
status: done         # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "config.go"
  - "config.example.toml"
  - "*_test.go"
  - "web/src/**"
  - "web/index.html"
  - "web/e2e/**"
  - "README*.md"
spec: SPEC.md「〇〇」
created: 2026-10-16
---
# テーマを設定ファイルで変えられるようにする

## 目的
テーマを設定ファイルで変えられるようにする

## 完了の条件
- [x] 確かめる手順: [theme] の name で組み込みのテーマ（今の色を default として、ほかに明るいテーマを含めていくつか）を選び、読み込み直すと端末とボタンの色が変わる
- [x] 確かめる手順: [theme] に背景・文字・カーソル・16色・強調色を書くと、そこだけ上書きされる
- [x] 確かめる手順: 色の書き間違い（#12345 など）は画面を開いたときに知らせ、その色だけ既定に戻す
- [x] config.example.toml と README に書き方を足す
- [x] Go のテストと画面の単体テストか通しのテストで上の1〜3を確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
- 16色を配列（`colors = [...]`）で書く形は、一部だけ上書きしにくいのでやめ、`black`〜`bright_white` の名前にした（docs/decisions.md）
- 色の確かめを Go 側でやると設定全体が既定に戻るので、画面側（キーバーの `[[keys]]` と同じ所）で行った

## 結果
- 何をしたか
  - 設定ファイルに `[theme]` を足した（config.go）。`name` と `background`・`foreground`・`cursor`・`accent`・16色（`black`〜`bright_white`）を書け、サーバーは `/api/config` の `theme` でそのまま画面に渡す。新しい版の設定ファイルでも `[theme]` は読む
  - 画面側に web/src/theme.ts を足した。組み込みのテーマは `default`（今までの色）・`light`・`solarized-dark`・`solarized-light`。テーマの色に書いた色を重ね、端末の CSS 変数（wterm の `--term-*`）とボタンの CSS 変数を上書きする。書き間違えた色・知らない名前は、キーの書き間違いと同じ知らせ（下の黒い帯）に出し、その所だけ既定に戻す
  - style.css で固定で書いていた色（ボタンの文字の黒、パネルの地、入力欄の地、知らせの地など）を変数にした。default での値は今までと同じ
  - config.example.toml、README.md、README.ja.md に書き方を足し、SPEC.md の「設定ファイル」に項目を足した
- どう確かめたか
  - `make test` が通る（Go、画面の単体テスト 121、通しのテスト 30）
  - Go: `TestConfigTheme`（書いた値がそのまま渡る・`[theme]` の知らない項目は書き間違い・新しい版でも読む）、`TestConfigExampleReadsAsIs`（見本に `[theme]` が入って読める）
  - 単体: web/src/theme.test.ts（テーマの選択、一部の上書き、`#12345` などの書き間違いはその色だけ戻す、知らない名前、画面への当て方）
  - 通し: 「[theme] でテーマを選ぶと端末とボタンの色が変わり、書いた色だけ上書きする。書き間違えた色は知らせて既定に戻す」（light にすると端末が白地になりキーバーの色も変わる → 背景・強調・青を上書きし、赤の `#12345` は知らせて light の赤のまま → 消すと元の色）
  - chromium で light・solarized-light・solarized-dark を開いた画面を撮って目で見た（端末・タブ・キーバー・tmux パネルの色がそろって変わる）
- 本人が見るべき点
  - 明るいテーマ（`light`、`solarized-light`）のボタン・パネルの色の好み。値は web/src/theme.ts の `THEMES` にまとめてある
  - `background` は端末だけでなく画面全体の背景も変える。`accent` を書くと薄い強調の色（tmux ボタンの地など）は自動で混ぜて作る
  - 実機で見るときは `make install` して palmterm を再起動してから読み込み直す

## レビュー
Codex（2026-10-16、320738d..2d2abd1）:

- **中** `config.go:29-51,125-129` / `web/src/theme.ts:147-150` — `red = 123` のようなTOMLとして正しい非文字列の色は、設定全体の読み込みエラーとなり、テーマ名や他の正しい色まで既定に戻ります。また `red = ""` は `omitempty` と空文字判定により通知なく無視されます。SPEC.md の「色の書き間違いは知らせ、その色だけテーマの色に戻す」と食い違います。テストも文字列形式の誤りしか扱っていません。
