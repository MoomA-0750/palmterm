---
id: 0027
status: done         # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "notify*.go"
  - "tmux.go"
  - "*_test.go"
  - "web/src/**"
  - "web/e2e/**"
spec: SPEC.md「〇〇」
created: 2026-10-19
---
# palmterm notify の長い文を切って届け、ウィンドウの番号がずれても帯から打ったウィンドウへ移れるようにする

## 目的
palmterm notify の長い文を切って届け、ウィンドウの番号がずれても帯から打ったウィンドウへ移れるようにする

## 完了の条件
- [x] 確かめる手順: 英数字2万文字の文で palmterm notify を打つ → 1000文字に切った文の帯が出る（Go のテストで確かめる）
- [x] 確かめる手順: renumber-windows on のセッションで、ウィンドウ 2 から palmterm notify を打つ → ウィンドウ 1 を閉じる → 帯を押すと、打ったウィンドウ（今は 1 番）に移る
- [x] 確かめる手順: 打ったウィンドウが閉じられていれば、帯を押しても別のウィンドウには移らず、帯が消えるだけ
- [x] Go のテストと通しのテストで上を確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
- 受け口の1行の上限（16KB）を上げる案はとらず、送る側で切る形にした（docs/decisions.md）
- 番号で指したままでは、打ったウィンドウを閉じたあとに同じ番号のほかのウィンドウへ移ってしまうので、tmux のウィンドウ ID（@N）で指すことにした

## 結果
- 何をしたか
  - 長い文: これまでは英数字2万文字だと JSON の1行が受け口の上限 16KB を超え、読めずに捨てられて帯が出なかった。`palmterm notify` の側で文とウィンドウの名前を 1000 文字に切ってから送るようにした（notify.go）
  - 番号のずれ: 知らせに tmux のウィンドウ ID（`windowId`、@N）を添え、帯を押すと新しい操作 `select-window-id` でその ID のウィンドウに移るようにした（notify.go・tmux.go・web/src/notices.ts・tmuxpanel.ts・main.ts）。palmterm のセッションにその ID が無ければ何もせず（エラーの知らせも出さず）、帯が消えるだけ
- どう確かめたか
  - Go のテスト `TestNotifyLongText`（2万文字 → 1000 文字の帯）、`TestNotifyWindowAfterRenumber`（renumber-windows on でウィンドウ 2 から打つ → 1 を閉じる → 押すと今は 1 番の build に移る。閉じたあとは移らない）を足した
  - 通しのテスト「palmterm notify」に、renumber-windows on で同じ流れを実際のブラウザで押すテストを足した（閉じたあとは帯が消え、ウィンドウは 0 のまま、知らせ #toast も出ない）
  - `make test` が通った（Go、単体 158 件、通し 42 件）
- 本人が見るべき点
  - 帯に出す番号は打ったときのまま（例: `2:build` の帯を押すと、今は 1 番になった build に移る）。押す前に番号を今のものに書き換えはしていない
  - 打ったウィンドウが閉じられていたとき、何も知らせずに帯が消えるだけにした（完了の条件どおり）
  - 実機で試すときは `make install` して palmterm を再起動してから

## レビュー
Codex（2026-10-19、63a0472..53d14f0）:

- [中] `tmux.go:145-151,99-101` — ウィンドウの存在確認と `select-window` が別々の tmux コマンドなので、その間に対象が閉じられると 409 になり、画面にエラーの知らせが出ます。SPEC.md の「閉じられていれば帯が消えるだけ（知らせも出さない）」を競合時に満たしません。
