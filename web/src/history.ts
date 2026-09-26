import { WTerm } from "@wterm/dom";
import { setupNerdIcons } from "./nerd";

/**
 * スクロール用の履歴表示。tmux の中ではブラウザ側に履歴がたまらないので、tmux から色付きの
 * 履歴を取り、2つ目の wterm に流し込んで端末の上に重ねる。ブラウザの普通のスクロールなので
 * 慣性も効き、長押しで文字も選べる。タップするか一番下まで戻ると閉じる。
 */
export class HistoryView {
  private wrap: HTMLDivElement;
  private host: HTMLDivElement;
  private term: WTerm | null = null;
  private opening: Promise<boolean> | null = null;
  private generation = 0;

  constructor(parent: HTMLElement, private mainTerm: HTMLElement) {
    this.wrap = document.createElement("div");
    this.wrap.id = "history";
    this.wrap.hidden = true;
    const badge = document.createElement("div");
    badge.className = "history-badge";
    badge.textContent = "履歴 · タップで戻る";
    this.host = document.createElement("div");
    this.host.className = "history-term";
    this.wrap.append(this.host, badge);
    parent.appendChild(this.wrap);

    // 端末のクリック処理（入力欄へのフォーカス＝キーボードが出る）より先に受けて、閉じる。
    this.wrap.addEventListener(
      "click",
      (e) => {
        e.stopPropagation();
        const sel = window.getSelection();
        if (!sel || sel.isCollapsed) this.close();
      },
      true,
    );
    // 一番下まで戻して止まったら閉じる。開いた直後（一番下にいる）や、指で押さえている間は閉じない。
    this.host.addEventListener("scroll", () => {
      if (!this.atBottom()) this.armed = true;
    });
    this.host.addEventListener("scrollend", () => this.maybeClose());
    this.wrap.addEventListener("touchstart", () => this.setHolding(true), { passive: true });
    const release = (e: TouchEvent) => {
      if (e.touches.length === 0) this.setHolding(false);
    };
    this.wrap.addEventListener("touchend", release);
    this.wrap.addEventListener("touchcancel", release);
  }

  private holding = false;
  private armed = false;

  /** 指で動かしている間は true。離したときに一番下にいれば閉じる。 */
  setHolding(holding: boolean) {
    this.holding = holding;
    if (!holding) this.maybeClose();
  }

  private maybeClose() {
    if (this.isOpen() && this.armed && !this.holding && this.atBottom()) this.close();
  }

  isOpen(): boolean {
    return !this.wrap.hidden;
  }

  /** 履歴を取って表示する。表示できたら true。 */
  open(cols: number, rows: number): Promise<boolean> {
    if (this.isOpen()) return Promise.resolve(true);
    if (this.opening) return this.opening;
    const gen = ++this.generation;
    this.opening = this.load(cols, rows, gen).finally(() => (this.opening = null));
    return this.opening;
  }

  private async load(cols: number, rows: number, gen: number): Promise<boolean> {
    let text: string;
    try {
      const res = await fetch("/api/history?color=1&lines=5000", { cache: "no-store" });
      if (!res.ok) return false;
      text = await res.text();
    } catch {
      return false;
    }
    if (gen !== this.generation) return false;

    this.term?.destroy();
    this.host.replaceChildren();
    // 文字サイズと行の高さは本体の端末に合わせる。
    for (const name of ["--term-font-size", "--term-row-height"]) {
      this.host.style.setProperty(name, this.mainTerm.style.getPropertyValue(name));
    }
    // 内蔵のエンジンは履歴を約1000行までしか持てないので、履歴の量を決められる libghostty の
    // エンジンを使う（初めて開くときにだけ読み込む）。上限は行数ではなくバイト数。
    const { GhosttyCore } = await import("@wterm/ghostty");
    const core = await GhosttyCore.load({ scrollbackLimit: 32 * 1024 * 1024 });
    const term = new WTerm(this.host, { core, cols, rows, autoResize: false, cursorBlink: false, onData: () => {} });
    await term.init();
    if (gen !== this.generation) {
      term.destroy();
      return false;
    }
    this.term = term;
    setupNerdIcons(this.host);
    // カーソルは見せない。最後の行のあとで改行しないよう、末尾の改行は落とす。
    term.write("\x1b[?25l" + text.replace(/\n+$/, "").replace(/\n/g, "\r\n"));
    this.armed = false;
    this.wrap.hidden = false;
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
    this.host.scrollTop = this.host.scrollHeight;
    return true;
  }

  /** 指の動きで動かす。dy > 0 で新しい方（下）へ。 */
  scrollBy(dy: number) {
    this.host.scrollTop += dy;
  }

  atBottom(): boolean {
    return this.host.scrollHeight - this.host.clientHeight - this.host.scrollTop < 4;
  }

  close() {
    this.generation++;
    if (!this.isOpen()) return;
    this.wrap.hidden = true;
    window.getSelection()?.removeAllRanges();
    this.term?.destroy();
    this.term = null;
    this.host.replaceChildren();
  }
}
