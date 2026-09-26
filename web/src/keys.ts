// 修飾キーの状態と、キーから端末に送るバイト列への変換。

export type ModName = "ctrl" | "alt" | "shift";
// once: 次の1キーだけ効く。lock: 解除するまで効き続ける。
export type ModState = "off" | "once" | "lock";

/** 効いている修飾キー。画面の Ctrl/Alt/Shift の状態のほか、設定の組み合わせ（ctrl+c など）にも使う。 */
export interface ModSet {
  has(name: ModName): boolean;
  param(): number;
}

/** xterm の修飾パラメータ（1 + Shift 1 + Alt 2 + Ctrl 4）。 */
function modParam(m: Pick<ModSet, "has">): number {
  return 1 + (m.has("shift") ? 1 : 0) + (m.has("alt") ? 2 : 0) + (m.has("ctrl") ? 4 : 0);
}

/** base（画面の修飾キーの状態）に、extra の修飾キーを足したもの。 */
export function withMods(base: ModSet, extra: readonly ModName[]): ModSet {
  const has = (name: ModName) => extra.includes(name) || base.has(name);
  return { has, param: () => modParam({ has }) };
}

// 修飾キーをこの間隔（ms）以内に2回押したら「固定」にする。
const DOUBLE_TAP_MS = 350;

export class Modifiers implements ModSet {
  private state: Record<ModName, ModState> = { ctrl: "off", alt: "off", shift: "off" };
  onChange: () => void = () => {};

  get(name: ModName): ModState {
    return this.state[name];
  }

  private lastTap: Record<ModName, number> = { ctrl: 0, alt: 0, shift: 0 };

  /**
   * 1回押すと「1回」。すばやく続けてもう一度押すと「固定」、間を空けて押すと解除。
   * 「固定」のときは押すと解除。
   */
  tap(name: ModName) {
    const now = performance.now();
    const quick = now - this.lastTap[name] <= DOUBLE_TAP_MS;
    this.lastTap[name] = now;
    const state = this.state[name];
    this.state[name] = state === "off" ? "once" : state === "once" && quick ? "lock" : "off";
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

  param(): number {
    return modParam(this);
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
export function applyToChar(ch: string, mods: ModSet): string {
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

export const SPECIAL_KEYS = [
  "esc", "tab", "enter", "backspace", "delete", "insert",
  "up", "down", "left", "right",
  "home", "end", "pageup", "pagedown",
  "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12",
] as const;
export type SpecialKey = (typeof SPECIAL_KEYS)[number];

// F5〜F12 と Insert・Delete・PageUp・PageDown は ESC [ 番号 ~ の形。
const tildeCode: Partial<Record<SpecialKey, number>> = {
  insert: 2, delete: 3, pageup: 5, pagedown: 6,
  f5: 15, f6: 17, f7: 18, f8: 19, f9: 20, f10: 21, f11: 23, f12: 24,
};
// F1〜F4 は ESC O P〜S（修飾つきは ESC [ 1 ; 修飾 P〜S）。
const ss3Final: Partial<Record<SpecialKey, string>> = { f1: "P", f2: "Q", f3: "R", f4: "S" };

const cursorFinal: Partial<Record<SpecialKey, string>> = {
  up: "A", down: "B", right: "C", left: "D", home: "H", end: "F",
};

/**
 * 特殊キーのバイト列。appCursor はアプリがカーソルキーのモード（DECCKM）を
 * 有効にしているときで、修飾なしの矢印キーは ESC O A の形になる。
 */
export function specialKey(key: SpecialKey, mods: ModSet, appCursor: boolean): string {
  const m = mods.param();
  const final = cursorFinal[key];
  if (final) {
    if (m > 1) return `\x1b[1;${m}${final}`;
    return appCursor ? `\x1bO${final}` : `\x1b[${final}`;
  }
  const tilde = tildeCode[key];
  if (tilde) return m > 1 ? `\x1b[${tilde};${m}~` : `\x1b[${tilde}~`;
  const ss3 = ss3Final[key];
  if (ss3) return m > 1 ? `\x1b[1;${m}${ss3}` : `\x1bO${ss3}`;
  const alt = mods.has("alt") ? "\x1b" : "";
  switch (key) {
    case "tab":
      // Shift+Tab は ESC [ Z。Alt はほかのキーと同じく前に ESC を付ける（Ctrl+Tab には広く決まった信号がない）。
      return alt + (mods.has("shift") ? "\x1b[Z" : "\t");
    case "esc":
      return alt + "\x1b";
    case "enter":
      return alt + "\r";
    case "backspace":
      return alt + (mods.has("ctrl") ? "\x08" : "\x7f");
  }
  return "";
}

// 端末が送るキーの信号から、特殊キーの名前を引く（修飾なしの形だけ）。
const KEY_SEQUENCES: Record<string, SpecialKey> = {
  "\x1b": "esc", "\t": "tab", "\r": "enter", "\x7f": "backspace",
  "\x1b[A": "up", "\x1b[B": "down", "\x1b[C": "right", "\x1b[D": "left", "\x1b[H": "home", "\x1b[F": "end",
  "\x1bOA": "up", "\x1bOB": "down", "\x1bOC": "right", "\x1bOD": "left", "\x1bOH": "home", "\x1bOF": "end",
  "\x1bOP": "f1", "\x1bOQ": "f2", "\x1bOR": "f3", "\x1bOS": "f4",
};
for (const [key, code] of Object.entries(tildeCode)) KEY_SEQUENCES[`\x1b[${code}~`] = key as SpecialKey;

/**
 * 物理キーボードの特殊キー（矢印など）の信号に、画面の修飾キーを効かせた信号。キーの信号でなければ null
 * （端末が自分で返す答えやマウスの信号には効かせない）。
 */
export function keySequenceWithMods(data: string, mods: ModSet, appCursor: boolean): string | null {
  const key = KEY_SEQUENCES[data];
  return key ? specialKey(key, mods, appCursor) : null;
}
