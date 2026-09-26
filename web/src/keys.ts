// 修飾キーの状態と、キーから端末に送るバイト列への変換。

export type ModName = "ctrl" | "alt" | "shift";
// once: 次の1キーだけ効く。lock: 解除するまで効き続ける。
export type ModState = "off" | "once" | "lock";

export class Modifiers {
  private state: Record<ModName, ModState> = { ctrl: "off", alt: "off", shift: "off" };
  onChange: () => void = () => {};

  get(name: ModName): ModState {
    return this.state[name];
  }

  /** タップするたびに 切 → 1回 → 固定 → 切 と進む。 */
  tap(name: ModName) {
    const next: Record<ModState, ModState> = { off: "once", once: "lock", lock: "off" };
    this.state[name] = next[this.state[name]];
    this.onChange();
  }

  active(): boolean {
    return this.state.ctrl !== "off" || this.state.alt !== "off" || this.state.shift !== "off";
  }

  has(name: ModName): boolean {
    return this.state[name] !== "off";
  }

  /** キーを1つ送ったあとに呼ぶ。「1回」のものだけ外す。 */
  consume() {
    let changed = false;
    for (const name of ["ctrl", "alt", "shift"] as const) {
      if (this.state[name] === "once") {
        this.state[name] = "off";
        changed = true;
      }
    }
    if (changed) this.onChange();
  }

  /** xterm の修飾パラメータ（1 + Shift 1 + Alt 2 + Ctrl 4）。 */
  param(): number {
    return 1 + (this.has("shift") ? 1 : 0) + (this.has("alt") ? 2 : 0) + (this.has("ctrl") ? 4 : 0);
  }
}

const ctrlSymbols: Record<string, string> = {
  "@": "\x00", " ": "\x00", "2": "\x00",
  "[": "\x1b", "3": "\x1b",
  "\\": "\x1c", "4": "\x1c",
  "]": "\x1d", "5": "\x1d",
  "^": "\x1e", "6": "\x1e",
  "_": "\x1f", "/": "\x1f", "7": "\x1f",
  "?": "\x7f", "8": "\x7f",
};

/** 1文字に修飾キーを効かせる。変換できない組み合わせは修飾なしの文字として送る。 */
export function applyToChar(ch: string, mods: Modifiers): string {
  let c = ch;
  if (mods.has("shift")) c = c.toUpperCase();
  if (mods.has("ctrl")) {
    if (/^[a-zA-Z]$/.test(c)) c = String.fromCharCode(c.toUpperCase().charCodeAt(0) & 0x1f);
    else if (c in ctrlSymbols) c = ctrlSymbols[c];
  }
  if (mods.has("alt")) c = "\x1b" + c;
  return c;
}

/** データが修飾キーを効かせる対象（エスケープで始まらない1文字）かどうか。 */
export function isSingleChar(data: string): boolean {
  return [...data].length === 1 && data >= " " && data !== "\x7f";
}

export type SpecialKey =
  | "esc" | "tab" | "enter" | "backspace"
  | "up" | "down" | "left" | "right"
  | "home" | "end" | "pageup" | "pagedown";

const cursorFinal: Partial<Record<SpecialKey, string>> = {
  up: "A", down: "B", right: "C", left: "D", home: "H", end: "F",
};

/**
 * 特殊キーのバイト列。appCursor はアプリがカーソルキーのモード（DECCKM）を
 * 有効にしているときで、修飾なしの矢印キーは ESC O A の形になる。
 */
export function specialKey(key: SpecialKey, mods: Modifiers, appCursor: boolean): string {
  const m = mods.param();
  const final = cursorFinal[key];
  if (final) {
    if (m > 1) return `\x1b[1;${m}${final}`;
    return appCursor ? `\x1bO${final}` : `\x1b[${final}`;
  }
  const alt = mods.has("alt") ? "\x1b" : "";
  switch (key) {
    case "pageup":
      return m > 1 ? `\x1b[5;${m}~` : "\x1b[5~";
    case "pagedown":
      return m > 1 ? `\x1b[6;${m}~` : "\x1b[6~";
    case "tab":
      return mods.has("shift") ? "\x1b[Z" : alt + "\t";
    case "esc":
      return alt + "\x1b";
    case "enter":
      return alt + "\r";
    case "backspace":
      return alt + (mods.has("ctrl") ? "\x08" : "\x7f");
  }
  return "";
}
