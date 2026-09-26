import type { WTerm } from "@wterm/dom";

/**
 * 全画面のアプリ（tmux・vim・lazygit など）の上で指を縦にスワイプしたときの処理。
 * アプリがマウスを受け取るならホイール操作を合成して wterm に渡し（wterm が信号にする）、
 * 受け取らないなら ↑↓ キーを送る。通常の画面では、ブラウザの普通のスクロールに任せる。
 */
export function setupTouchScroll(
  el: HTMLElement,
  term: WTerm,
  send: (data: string) => void,
  appCursor: () => boolean,
) {
  const rowHeight = () =>
    parseFloat(getComputedStyle(el).getPropertyValue("--term-row-height")) || 16;

  const mouseTracking = () => (term.bridge?.mouseTracking?.() ?? 0) !== 0;
  const intercept = () => (term.bridge?.usingAltScreen() ?? false) || mouseTracking();

  // wterm は描き直しのたびに行の要素を差し替えるので、指を置いた要素が外れると
  // touchmove が el まで届かなくなる。指を置いた要素そのものに、その間だけ付ける。
  el.addEventListener(
    "touchstart",
    (e) => {
      if (e.touches.length !== 1 || !intercept()) return;
      const target = e.target as EventTarget;
      let lastY = e.touches[0].clientY;
      let acc = 0;
      const onMove = (ev: Event) => {
        const te = ev as TouchEvent;
        if (te.touches.length !== 1) return;
        te.preventDefault();
        const t = te.touches[0];
        acc += lastY - t.clientY;
        lastY = t.clientY;
        // ホイール1回で数行進むアプリが多い（tmux は5行）ので、ホイールは3行ぶんごとに送る。
        const step = rowHeight() * (mouseTracking() ? 3 : 1);
        while (Math.abs(acc) >= step) {
          const dir = acc > 0 ? 1 : -1;
          acc -= dir * step;
          scrollOnce(dir, t.clientX, t.clientY);
        }
      };
      const onEnd = () => {
        target.removeEventListener("touchmove", onMove);
        target.removeEventListener("touchend", onEnd);
        target.removeEventListener("touchcancel", onEnd);
      };
      target.addEventListener("touchmove", onMove, { passive: false });
      target.addEventListener("touchend", onEnd);
      target.addEventListener("touchcancel", onEnd);
    },
    { passive: true },
  );

  // dir: 1 は下へ（指を上に動かしたとき）、-1 は上へ。
  function scrollOnce(dir: number, clientX: number, clientY: number) {
    if (mouseTracking()) {
      el.dispatchEvent(
        new WheelEvent("wheel", {
          deltaY: dir * 100,
          deltaMode: WheelEvent.DOM_DELTA_PIXEL,
          clientX,
          clientY,
          bubbles: true,
          cancelable: true,
        }),
      );
    } else {
      const final = dir > 0 ? "B" : "A";
      send(appCursor() ? `\x1bO${final}` : `\x1b[${final}`);
    }
  }
}
