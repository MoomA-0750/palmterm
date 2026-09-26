// ソフトキーボードが出ているかの見積もり。ブラウザは直接は教えてくれないので、画面の見えている高さが、
// 同じ幅（＝同じ向き）で今までに見た一番高いときより大きく縮んでいれば、出ているとみなす。

// これより縮んでいたらキーボード（アドレスバーの出し入れ程度の変化は含めない）。
const KEYBOARD_MIN_PX = 120;

export class SoftKeyboard {
  private fullHeight = new Map<number, number>();

  /** 今の見えている範囲の幅と高さを渡す。キーボードが出ていそうなら true。 */
  visible(width: number, height: number): boolean {
    const w = Math.round(width);
    const full = Math.max(this.fullHeight.get(w) ?? 0, height);
    this.fullHeight.set(w, full);
    return height < full - KEYBOARD_MIN_PX;
  }
}
