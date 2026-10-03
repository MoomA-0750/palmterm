import type { WTerm } from "@wterm/dom";

/**
 * 入力欄にフォーカスがあるときに、端末の文字をなぞって選べるようにする。
 * iPad の Safari は、入力欄にフォーカスがある（ソフトキーボードが出ている・物理キーボードで端末に
 * 入力している）間は、ほかの所の文字を選ばせない。触れた瞬間に決めてしまうので、あとからフォーカスを
 * 外しても間に合わない。そこで、そのときだけ選択の範囲をこちらで作る。
 * - マウス: 押したまま動かしたら、押した所から今の所まで選ぶ
 * - 指: 長押ししたら選び始め、そのまま動かすと広げる（スクロールはしない）
 * 選び始めるときに入力欄のフォーカスを外す（選んだ文字をコピーできるように）。
 * 入力欄にフォーカスがなければブラウザの選択がそのまま使えるので、何もしない。
 */

const LONG_PRESS_MS = 400;
const TOUCH_MOVE_PX = 10;
const MOUSE_MOVE_PX = 4;

export interface DragSelect {
  /** 今、指でなぞって選んでいる途中か（その間は指でスクロールしない）。 */
  selecting(): boolean;
}

export function setupDragSelect(el: HTMLElement, term: WTerm): DragSelect {
  let drag: { id: number; x: number; y: number; touch: boolean; timer?: number } | null = null;
  let active = false; // 選び始めた

  const inputFocused = () => document.activeElement?.matches("input, textarea") ?? false;

  const end = () => {
    if (drag?.timer !== undefined) window.clearTimeout(drag.timer);
    drag = null;
    active = false;
  };

  /** 選び始める。押した所の1文字を選んでおく（指を動かさなくても選んだことが見えるように）。 */
  const begin = () => {
    if (!drag) return;
    active = true;
    (document.activeElement as HTMLElement | null)?.blur();
    extendTo(drag.x, drag.y, true);
  };

  /** 押した所から (x, y) までを選ぶ。押した所も毎回測り直す（描き直しで行の要素が変わるため）。 */
  const extendTo = (x: number, y: number, oneChar = false) => {
    if (!drag) return;
    const anchor = caretAt(el, drag.x, drag.y);
    const focus = caretAt(el, x, y);
    if (!anchor || !focus) return;
    if (oneChar && anchor[0] === focus[0] && anchor[1] === focus[1]) {
      const len = anchor[0].textContent?.length ?? 0;
      if (anchor[1] < len) focus[1] = anchor[1] + 1;
    }
    document.getSelection()?.setBaseAndExtent(anchor[0], anchor[1], focus[0], focus[1]);
  };

  el.addEventListener("pointerdown", (e) => {
    end();
    if (!e.isPrimary || e.button !== 0 || !inputFocused()) return;
    const touch = e.pointerType !== "mouse";
    // 中のアプリがマウスを受け取っているときは、アプリが選ぶ（wterm がマウスの操作を送る）
    if (!touch && !e.shiftKey && (term.bridge?.mouseTracking?.() ?? 0) !== 0) return;
    drag = { id: e.pointerId, x: e.clientX, y: e.clientY, touch };
    // 描き直しで指の下の行の要素が差し替わっても、動きを受け取り続ける
    try {
      el.setPointerCapture(e.pointerId);
    } catch {
      // 取れなくても、要素が差し替わらない間は動く
    }
    if (touch) drag.timer = window.setTimeout(begin, LONG_PRESS_MS);
    else e.preventDefault(); // ブラウザの選択を始めない（こちらで選ぶ）
  });

  el.addEventListener("pointermove", (e) => {
    if (!drag || e.pointerId !== drag.id) return;
    const moved = Math.hypot(e.clientX - drag.x, e.clientY - drag.y);
    if (!active) {
      if (drag.touch) {
        if (moved > TOUCH_MOVE_PX) end(); // 長押しの前に動かした（スクロール）
        return;
      }
      if (moved <= MOUSE_MOVE_PX) return;
      begin();
    }
    extendTo(e.clientX, e.clientY);
  });

  el.addEventListener("pointerup", end);
  el.addEventListener("pointercancel", end);
  // 2本目の指が来たらピンチなので取りやめる。
  el.addEventListener("touchstart", (e) => e.touches.length > 1 && !active && end(), { passive: true });

  return { selecting: () => active && drag?.touch === true };
}

/** 画面の位置にある端末の文字の位置（テキストのノードと何文字目か）。端末の行の外なら null。 */
function caretAt(el: HTMLElement, x: number, y: number): [Node, number] | null {
  const doc = document as Document & {
    caretRangeFromPoint?: (x: number, y: number) => Range | null;
    caretPositionFromPoint?: (x: number, y: number) => { offsetNode: Node; offset: number } | null;
  };
  let point: [Node, number] | null = null;
  if (doc.caretPositionFromPoint) {
    const p = doc.caretPositionFromPoint(x, y);
    if (p) point = [p.offsetNode, p.offset];
  } else if (doc.caretRangeFromPoint) {
    const r = doc.caretRangeFromPoint(x, y);
    if (r) point = [r.startContainer, r.startOffset];
  }
  if (!point) return null;
  const node = point[0];
  const row = (node instanceof Element ? node : node.parentElement)?.closest(".term-row");
  return row && el.contains(row) ? point : null;
}
