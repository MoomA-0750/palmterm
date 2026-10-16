---
id: 0012
status: done         # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "config.go"
  - "fonts*.go"
  - "main.go"
  - "config.example.toml"
  - "*_test.go"
  - "web/src/**"
  - "web/e2e/**"
  - "README*.md"
spec: SPEC.md「〇〇」
created: 2026-10-16
---
# フォントを設定ファイルで変えられるようにする

## 目的
フォントを設定ファイルで変えられるようにする

## 完了の条件
- [x] 確かめる手順: [font] の family に PC に入っているフォントの名前を書くと、palmterm が PC からそのフォントのファイルを探してスマホに配り、端末の文字がそのフォントになる
- [x] 確かめる手順: PC に無い名前なら、スマホにあればそれを使い、無ければ今の JetBrains Mono に戻る
- [x] 確かめる手順: [font] の file にフォントファイルのパスを書くと、そのファイルを配って使う。相対パスは ~/.config/palmterm/ から数える（フォルダごと写せば移る）
- [x] 確かめる手順: Nerd Font のアイコンは、どのフォントにしても今までどおり出る
- [x] 配るのはログインした端末にだけで、指定したフォント以外の PC のファイルは配らない
- [x] Go のテストと画面のテストで上の1〜5を確かめ、make test が通る。config.example.toml と README に足す

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
- PC のフォント探しは fc-list だけに頼らず、palmterm が自分で置き場所を歩いて name テーブルを読む（macOS で使えるように、テストが PC のフォントに左右されないように）。go.mod は scope の外なので、ライブラリは足さずに name テーブルを読む所だけ書いた
- 試してやめたことは特になし

## 結果
### したこと
- 設定ファイルに `[font]`（`family`・`file`）を足した（config.go。新しい版の設定ファイルでも読む）
- fonts.go：`family` の名前で PC のフォントを探す（`$XDG_DATA_HOME/fonts`・`~/.fonts`・`$XDG_DATA_DIRS/fonts`・macOS の置き場所 → 無ければ `fc-list`）。普通の太さ・斜体でないもの、.ttc より単独のファイルを選ぶ。`file` は設定ファイルのあるフォルダからの相対パス（`~/` も可）で、中身の先頭でフォントか確かめる。選んだファイルだけを `GET /api/font`（ログインが要る）で配る。`/api/config` に `fontURL` と `fontError` を足した
- 画面（web/src/font.ts）：配られたフォントを FontFace で読み、font-family を「Nerd Symbols、配ったフォント、family の名前、JetBrains Mono、…」にする。`file` を使えないときは下に知らせる
- config.example.toml・README（英日）・SPEC.md・docs/decisions.md を直した

### 確かめたこと
- `make test` が通る（Go、画面の単体テスト 126 件、通しのテスト 31 件）
- 通しのテストでは、テスト用のフォント（JetBrains Mono の名前を付け替えたもの）を「PC だけにある」「ブラウザ（スマホ）だけにある」「設定フォルダの file」の3つ用意し、Chromium が実際に描いたフォントの名前（CDP の CSS.getPlatformFontsForNode）で、配ったフォント・スマホのフォント・JetBrains Mono への戻り、アイコンのフォントが変わらないことを確かめた。フォントを当てる所を外すとこのテストが落ちることも確かめた
- この PC で `Liberation Mono`・`Noto Sans Mono`・`Adwaita Mono` が名前で見つかることを確かめた

### 本人が見るべき点
- 実機（iPad など）ではまだ試していない。`make install` と再起動のあと、`[font]` に `family = "Liberation Mono"` などを書いて読み込み直し、文字が変わるか見てほしい
- 配るのは普通の太さの1ファイルだけなので、太字・斜体はブラウザが合成する。大きなフォント（CJK など数十 MB）は最初の読み込みに時間がかかる（読み込み中は最大3秒ほど文字が出ない）
- `file` の相対パスは「設定ファイルのあるフォルダ」から数える（既定では `~/.config/palmterm/`）。`-config` で別の場所を指したときは、その設定ファイルのフォルダになる
- PC に入れたばかりのフォントは、30秒たつか設定ファイルを保存し直すと見つかる

## レビュー
Codex（2026-10-16、c48cb86..3a0ee4a）:

- 中: `fonts.go:193-198,303-304` — SFNT の Macintosh 名を Mac Roman として復号せず UTF-8 扱いし、スタイル判定も英語の `regular`・`bold`・`italic` 等だけです。非 ASCII の family 名を持つ macOS フォントは見つからず、ローカライズされたスタイル名では太字・斜体を通常書体として選ぶ可能性があります。「macOS のフォントを名前で探し、普通の太さで斜体でないものを選ぶ」という `SPEC.md` の確定仕様を満たしません。テストも ASCII・platform 3 のフォントしか扱っておらず、このケースを検出できません。
- 低: `README.ja.md:243`, `README.md:251`, `config.example.toml:35` — 利用可能な `file` 形式から `.ttc` が抜けています。`SPEC.md:156` と実装は `.ttc` 対応を明記しており、利用者向け説明と食い違います。
