---
id: 0027
status: doing        # todo / doing / blocked / done
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
- [ ] 確かめる手順: 英数字2万文字の文で palmterm notify を打つ → 1000文字に切った文の帯が出る（Go のテストで確かめる）
- [ ] 確かめる手順: renumber-windows on のセッションで、ウィンドウ 2 から palmterm notify を打つ → ウィンドウ 1 を閉じる → 帯を押すと、打ったウィンドウ（今は 1 番）に移る
- [ ] 確かめる手順: 打ったウィンドウが閉じられていれば、帯を押しても別のウィンドウには移らず、帯が消えるだけ
- [ ] Go のテストと通しのテストで上を確かめ、make test が通る

## 本人に訊くこと
（実行役が書く。本人は `task-loop answer` で答える）

## メモ
（実行役が書く。試してだめだったこと）

## 結果
（実行役が書く。何をしたか／どう確かめたか／本人が見るべき点）

## レビュー
（task-loop が Codex の指摘を書き足す）
