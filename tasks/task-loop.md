# 課題の扱い（task-loop）

このファイルは task-loop が管理し、`task-loop run`のたびに最新に合わせる。直すときは task-loop のリポジトリの`templates/task-loop.md`を直す。

- 本人の思いつきは`tasks/inbox.md`に入る。inbox の行は課題として読まれない。対話の中で課題に起こすときは、完了の条件・scope・名前を本人と決めてから`task-loop add . "題" --criteria "条件" --scope GLOB --name 英字の名前`で作る（番号と commit は task-loop が持つ。課題ファイルを自分で作らない）
- task-loop run が読むのは main に commit された課題だけ
- 「完了の条件」と`scope`を本人に見せてOKをもらってから`status: todo`にする
- 1件は1回の作業で終わる大きさにする。大きければ分ける
- 完了の条件は、テスト名か「こう操作するとこうなる」の手順で書く
- 既存の課題を書き換えない（状態は task-loop の`tasks`ブランチ側にある）。答えは`task-loop answer`で渡す
- 課題にホスト名・個人のパス・認証情報を書かない
