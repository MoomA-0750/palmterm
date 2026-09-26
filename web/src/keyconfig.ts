// キーバーのキーの定義。設定ファイルの [[keys]] と同じ形で書き、ボタンの表示と押したときの動きに直す。
//
//   key  = "esc" / "ctrl+c" / "alt+shift+left" / "ctrl+space" / "f5" …（特殊キーか1文字に、修飾キーを + でつなぐ）
//   mod  = "ctrl" / "alt" / "shift"（押すたびに 1回 → 固定 → 切。次に押したキーに効く）
//   text = "|"（そのまま送る。1文字なら画面の修飾キーも効く）
//   label・icon・repeat は省略できる（キーから決める）。

import { t } from "./i18n";
import { type IconName, isIconName } from "./icons";
import { type ModName, SPECIAL_KEYS, type SpecialKey } from "./keys";

export interface KeyConfig {
  label?: string;
  key?: string;
  mod?: string;
  text?: string;
  icon?: string;
  repeat?: boolean;
}

export type KeyAction =
  | { type: "mod"; mod: ModName }
  | { type: "key"; key: SpecialKey; mods: ModName[] }
  | { type: "char"; char: string; mods: ModName[] }
  | { type: "text"; text: string };

export interface KeyButton {
  label: string; // ボタンに出す文字（アイコンのときは読み上げ用）
  icon?: IconName;
  repeat: boolean;
  action: KeyAction;
}

export const DEFAULT_KEYS: KeyConfig[] = [
  { key: "esc" },
  { key: "tab" },
  { mod: "ctrl" },
  { mod: "alt" },
  { mod: "shift" },
  { key: "left" },
  { key: "down" },
  { key: "up" },
  { key: "right" },
  { key: "ctrl+c" },
  { key: "ctrl+d" },
  { key: "backspace" },
  { key: "enter" },
  { key: "home" },
  { key: "end" },
  { key: "pageup" },
  { key: "pagedown" },
  { text: "|" },
  { text: "~" },
  { text: "/" },
  { text: "-" },
  { text: "`" },
];

const MOD_NAMES: Record<string, ModName> = { ctrl: "ctrl", control: "ctrl", alt: "alt", meta: "alt", shift: "shift" };
const MOD_LABELS: Record<ModName, string> = { ctrl: "Ctrl", alt: "Alt", shift: "Shift" };

const KEY_LABELS: Partial<Record<SpecialKey, string>> = {
  esc: "Esc", tab: "Tab", enter: "Enter", backspace: "BS", delete: "Del", insert: "Ins",
  home: "Home", end: "End", pageup: "PgUp", pagedown: "PgDn",
  left: "Left", right: "Right", up: "Up", down: "Down",
};
// 修飾なしのとき、文字の代わりにアイコンで出すキー。
const KEY_ICONS: Partial<Record<SpecialKey, IconName>> = {
  left: "left", right: "right", up: "up", down: "down", backspace: "backspace", enter: "enter",
};
const REPEAT_KEYS = new Set<SpecialKey>(["left", "right", "up", "down", "backspace", "delete", "pageup", "pagedown"]);

function isSpecialKey(name: string): name is SpecialKey {
  return (SPECIAL_KEYS as readonly string[]).includes(name);
}

/** "ctrl+alt+x" を修飾キーと本体に分ける。"+" 自体は "ctrl++" のように書く。 */
function splitCombo(combo: string): { mods: ModName[]; base: string } {
  let rest = combo;
  let base: string;
  if (combo === "+") return { mods: [], base: "+" };
  if (combo.endsWith("++")) {
    base = "+";
    rest = combo.slice(0, -2);
  } else {
    const i = combo.lastIndexOf("+");
    base = combo.slice(i + 1);
    rest = i >= 0 ? combo.slice(0, i) : "";
  }
  const mods: ModName[] = [];
  for (const part of rest ? rest.split("+") : []) {
    const mod = MOD_NAMES[part.trim().toLowerCase()];
    if (!mod) throw new Error(t("unknownMod", { mod: part }));
    if (!mods.includes(mod)) mods.push(mod);
  }
  return { mods, base };
}

/** 修飾キーを短く書いた名前（^C、M-b、S-Tab のように）。 */
function comboLabel(mods: ModName[], base: string): string {
  const ctrlOnly = mods.length === 1 && mods[0] === "ctrl";
  if (ctrlOnly && [...base].length === 1) return "^" + base.toUpperCase();
  const prefix = mods.map((m) => ({ ctrl: "^", alt: "M-", shift: "S-" })[m]).join("");
  return prefix + base;
}

/** 設定の1キーをボタンに直す。書き方がおかしければ、理由を付けて投げる。 */
export function parseKey(k: KeyConfig): KeyButton {
  const kinds = [k.key, k.mod, k.text].filter((v) => v !== undefined && v !== "");
  if (kinds.length !== 1) throw new Error(t("keyNeedsOne"));
  if (k.icon !== undefined && !isIconName(k.icon)) throw new Error(t("unknownIcon", { icon: k.icon }));
  const icon = k.icon as IconName | undefined;

  if (k.mod) {
    const mod = MOD_NAMES[k.mod.toLowerCase()];
    if (!mod) throw new Error(t("unknownMod", { mod: k.mod }));
    return { label: k.label ?? MOD_LABELS[mod], icon, repeat: false, action: { type: "mod", mod } };
  }
  if (k.text !== undefined) {
    return { label: k.label ?? k.text, icon, repeat: k.repeat ?? false, action: { type: "text", text: k.text } };
  }

  const { mods, base } = splitCombo(k.key!);
  const name = base.toLowerCase();
  if (isSpecialKey(name)) {
    const plainIcon = mods.length === 0 && k.label === undefined ? KEY_ICONS[name] : undefined;
    const keyName = KEY_LABELS[name] ?? name.toUpperCase();
    // アイコンで出すキーの読み上げ用の名前。
    const spoken: string | undefined = {
      left: t("key.left"), right: t("key.right"), up: t("key.up"), down: t("key.down"),
      backspace: "Backspace", enter: "Enter",
    }[name as string];
    return {
      label: k.label ?? (mods.length ? comboLabel(mods, keyName) : (spoken ?? keyName)),
      icon: icon ?? plainIcon,
      repeat: k.repeat ?? REPEAT_KEYS.has(name),
      action: { type: "key", key: name, mods },
    };
  }
  const char = name === "space" ? " " : base;
  if ([...char].length !== 1) throw new Error(t("unknownKey", { key: k.key! }));
  return {
    label: k.label ?? comboLabel(mods, name === "space" ? "Space" : char),
    icon,
    repeat: k.repeat ?? false,
    action: { type: "char", char, mods },
  };
}
