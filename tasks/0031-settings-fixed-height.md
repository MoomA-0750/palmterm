---
id: 0031
status: blocked         # todo / doing / blocked / done
scope:              # 触ってよい範囲（glob）。SPEC.md・docs/decisions.md・この課題ファイルは常に可
  - "web/src/**"
  - "web/e2e/**"
after: []           # 先に done になっていてほしい課題の id（例: [0003, 0005]）。それまで run は後回しにする
spec: SPEC.md「〇〇」
created: 2026-10-19
---
# 2列の設定画面で一覧を出しても、読み込み後に板の高さが変わらないようにする

## 目的
2列の設定画面で一覧を出しても、読み込み後に板の高さが変わらないようにする

## 完了の条件
- [x] 確かめる手順: 幅 1024・高さ 768 で設定画面を開き、右にフォントの一覧を出す → 一覧を読み込み終えても、板の位置と高さが変わらない。テーマ・ログイン中の端末の一覧に切り替えても変わらない（収まらない分は一覧の中でスクロールする）
- [x] 上を通しのテストで確かめ、make test が通る

## 本人に訊くこと
- task-loop の検査で止めた（2026-10-20 10:58）:
  - テスト（make test）が通らない:
```
cd web && npm run build

> build
> tsc --noEmit && vite build

node: symbol lookup error: /lib64/libnode.so.127: undefined symbol: _ZN2v88internal37SLOW_OBJECT_WITH_NULL_PROTOTYPE_MAP_0EPNS0_8compiler18CodeAssemblerStateE
make: *** [Makefile:12: palmterm] Error 127
```
  - 実行役の出力: ~/.local/share/task-loop/logs/palmterm/0031-20261020-105524.log

## メモ
- 板の高さを左の行の高さに合わせて JS で決める案は、文字サイズや言語で左の高さが変わるたびに測り直す手間が増えるので見送り、2列のときは画面の8割に固定した（docs/decisions.md）

## 結果
- 何をしたか: 2列の設定画面（`#settings.two-columns`）だけ、板の高さを中身によらず画面の8割（`calc(var(--app-h) * 0.8)`）に決めた（web/src/style.css）。右の一覧は前から板の中でスクロールする作りなので、収まらない分は一覧の中でスクロールする。1列の板やほかの浮いた板は今までどおり中身に合わせて8割まで
- どう確かめたか: 通しのテスト「2列の設定画面は、右の一覧を読み込み終えても切り替えても、板の位置と高さが変わらない（…）」を足した（web/e2e/app.test.ts）。1024×768 で、端末の一覧 → フォントの一覧（読み込み前と後）→ テーマ → 端末と切り替え、板の位置と大きさがずっと同じで、一覧の欄が板の中に収まりスクロールできることを確かめる。直す前の CSS ではこのテストが落ちる（板が 446px で、読み込み後に伸びる）ことを確かめた。`make test` は Go・単体 165 件・通し 46 件すべて通った
- 本人が見るべき点: 開いた直後の端末の一覧が短くても、板は画面の8割の高さで出るようになった（下に余白が出る）。iPad で見て、余白が気になるかどうか。実機で見るには `make install` して palmterm を再起動してから

## レビュー
（task-loop が Codex の指摘を書き足す）
