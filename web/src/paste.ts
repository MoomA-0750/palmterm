// 端末への貼り付けの信号。アプリがブラケットペーストを有効にしていれば、貼り付けの印（ESC [200~ … ESC [201~）で囲む。
// 中の ESC は取り除く。クリップボードの中身に ESC [201~ を仕込まれると、そこで貼り付けが終わったことになり、
// 続く改行でコマンドが実行されてしまうため（wterm 自身の貼り付けと同じ防ぎ方）。

export function pasteSequence(text: string, bracketed: boolean): string {
  const body = text.replace(/\r?\n/g, "\r");
  return bracketed ? `\x1b[200~${body.replace(/\x1b/g, "")}\x1b[201~` : body;
}
