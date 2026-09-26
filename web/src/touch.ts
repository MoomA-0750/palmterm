import type { WTerm } from "@wterm/dom";

/**
 * 端末の上での指の操作。
 * - 2本指のピンチ: 文字サイズを変える
 * - 1本指の縦のスワイプ（tmux などの全画面の上）: 始めた時点の tmux の状態で動きを決める
 *   - シェルや Claude Code など、tmux のコピーモード中: tmux のコピーモードで行単位にスクロール
 *   - 中のアプリがマウスを受け取る（vim・lazygit など）: ホイール（wterm が信号にする）
 *   - 全画面のアプリでマウスなし（less など）: ↑↓ キー
 *   どれも、指を離したあとは速度に応じて減速しながら続ける。
 * 通常の画面（tmux を使っていない）では、ブラウザの普通のスクロールに任せる。
 *
 * wterm は描き直しのたびに行の要素を差し替えるので、指を置いた要素が外れると
 * touchmove が el まで届かなくなる。指を置いた要素そのものに、その間だけ付ける。
 */

export interface TouchOptions {
  el: HTMLElement;
  term: WTerm;
  send: (data: string) => void;
  /** tmux のコピーモードでスクロールする。負なら古い方（上）へ。 */
  scrollPane: (lines: number) => void;
  appCursor: () => boolean;
  getFontSize: () => number;
  setFontSize: (size: number, save: boolean) => void;
}

interface PaneState {
  altScreen: boolean;
  mouse: boolean;
  inMode: boolean;
}

type ScrollMode = "pending" | "tmux" | "wheel" | "keys" | "none";

const MIN_FONT = 8;
const MAX_FONT = 32;

export function setupTouch(opts: TouchOptions) {
  const { el, term } = opts;
  let endCurrent: (() => void) | null = null;
  let glideGeneration = 0;

  const rowHeight = () => parseFloat(getComputedStyle(el).getPropertyValue("--term-row-height")) || 16;
  const mouseTracking = () => (term.bridge?.mouseTracking?.() ?? 0) !== 0;
  const inFullScreen = () => (term.bridge?.usingAltScreen() ?? false) || mouseTracking();

  /** 今触れている指それぞれの要素に、ジェスチャーの間だけ処理を付ける。 */
  function track(e: TouchEvent, onMove: (ev: TouchEvent) => void, onEnd: () => void) {
    endCurrent?.();
    const targets = new Set<EventTarget>([...e.touches].map((t) => t.target));
    const move = (ev: Event) => onMove(ev as TouchEvent);
    const end = (ev: Event) => {
      if (ev.type === "touchend" && (ev as TouchEvent).touches.length > 0) return;
      finish();
    };
    const finish = () => {
      for (const t of targets) {
        t.removeEventListener("touchmove", move);
        t.removeEventListener("touchend", end);
        t.removeEventListener("touchcancel", end);
      }
      if (endCurrent === finish) endCurrent = null;
      onEnd();
    };
    for (const t of targets) {
      t.addEventListener("touchmove", move, { passive: false });
      t.addEventListener("touchend", end);
      t.addEventListener("touchcancel", end);
    }
    endCurrent = finish;
  }

  // ブラウザは指を置いた時点で、その場所に止められる touchmove の処理があるかを見て、
  // なければ自分のスクロールを始めてしまう（あとから付けた処理では止められない）。
  // 何もしない処理を常に付けておき、上の track で止められるようにする。
  el.addEventListener("touchmove", () => {}, { passive: false });

  el.addEventListener(
    "touchstart",
    (e) => {
      glideGeneration++; // 慣性で動いている途中なら止める
      if (e.touches.length === 2) startPinch(e);
      else if (e.touches.length === 1 && inFullScreen()) startScroll(e);
    },
    { passive: true },
  );

  // ---- ピンチで文字サイズ ----

  function startPinch(e: TouchEvent) {
    const dist = (ev: TouchEvent) =>
      Math.hypot(ev.touches[0].clientX - ev.touches[1].clientX, ev.touches[0].clientY - ev.touches[1].clientY);
    const d0 = dist(e);
    const size0 = opts.getFontSize();
    let size = size0;
    track(
      e,
      (ev) => {
        if (ev.touches.length !== 2) return;
        ev.preventDefault();
        const next = Math.min(MAX_FONT, Math.max(MIN_FONT, Math.round(size0 * (dist(ev) / d0))));
        if (next !== size) {
          size = next;
          opts.setFontSize(size, false);
        }
      },
      () => opts.setFontSize(size, true),
    );
  }

  // ---- 1本指のスクロール ----

  function startScroll(e: TouchEvent) {
    let mode: ScrollMode = "pending";
    let lastY = e.touches[0].clientY;
    let lastX = e.touches[0].clientX;
    let acc = 0; // まだ行に換算していない指の移動（新しい方へが正）
    let pendingDy = 0; // 状態を問い合わせている間の移動
    let velocity = 0; // 慣性用（px/ms、新しい方へが正）
    let lastTime = performance.now();
    let tmuxLines = 0; // 次のフレームで tmux に送る行数
    let flushScheduled = false;

    fetchPaneState().then((st) => {
      if (mode !== "pending") return;
      mode = decide(st);
      apply(pendingDy);
      pendingDy = 0;
    });

    track(
      e,
      (ev) => {
        if (ev.touches.length !== 1) return;
        ev.preventDefault();
        const t = ev.touches[0];
        const dy = lastY - t.clientY; // 指を上に動かすと正（新しい方へ）
        const now = performance.now();
        velocity = 0.8 * (dy / Math.max(1, now - lastTime)) + 0.2 * velocity;
        lastTime = now;
        lastY = t.clientY;
        lastX = t.clientX;
        if (mode === "pending") pendingDy += dy;
        else apply(dy);
      },
      () => {
        if (mode === "pending") mode = "none";
        // 動かしながら離したときだけ慣性を付ける（止めてから離したら付けない）。
        if (mode !== "none" && performance.now() - lastTime < 80) glide(velocity, apply);
      },
    );

    function decide(st: PaneState | null): ScrollMode {
      if (!st) return mouseTracking() ? "wheel" : "keys";
      if (st.inMode) return "tmux";
      if (st.mouse) return mouseTracking() ? "wheel" : "keys";
      if (st.altScreen) return "keys";
      return "tmux";
    }

    function apply(dy: number) {
      if (dy === 0 || mode === "pending" || mode === "none") return;
      acc += dy;
      const step = rowHeight() * (mode === "wheel" ? 3 : 1);
      while (Math.abs(acc) >= step) {
        const dir = acc > 0 ? 1 : -1;
        acc -= dir * step;
        scrollOnce(dir);
      }
    }

    // dir: 1 は下へ（新しい方）、-1 は上へ。
    function scrollOnce(dir: number) {
      if (mode === "tmux") {
        // 1行ごとに tmux を呼ばず、1フレーム分まとめて送る。
        tmuxLines += dir;
        if (!flushScheduled) {
          flushScheduled = true;
          requestAnimationFrame(() => {
            flushScheduled = false;
            opts.scrollPane(tmuxLines);
            tmuxLines = 0;
          });
        }
      } else if (mode === "wheel") {
        el.dispatchEvent(
          new WheelEvent("wheel", {
            deltaY: dir * 100,
            deltaMode: WheelEvent.DOM_DELTA_PIXEL,
            clientX: lastX,
            clientY: lastY,
            bubbles: true,
            cancelable: true,
          }),
        );
      } else {
        const final = dir > 0 ? "B" : "A";
        opts.send(opts.appCursor() ? `\x1bO${final}` : `\x1b[${final}`);
      }
    }
  }

  /** 指を離したあとの慣性。速度を少しずつ落としながら move を呼び続ける。次に触れたら止まる。 */
  function glide(v: number, move: (dy: number) => void) {
    const gen = glideGeneration;
    let last = performance.now();
    const frame = (now: number) => {
      if (gen !== glideGeneration || Math.abs(v) < 0.05) return;
      const dt = now - last;
      last = now;
      move(v * dt);
      v *= Math.pow(0.995, dt);
      requestAnimationFrame(frame);
    };
    requestAnimationFrame(frame);
  }
}

async function fetchPaneState(): Promise<PaneState | null> {
  try {
    const res = await fetch("/api/pane", { cache: "no-store" });
    return res.ok ? await res.json() : null;
  } catch {
    return null;
  }
}
