// 画面の色。設定ファイルの [theme] の name で、組み込みのテーマか [themes.名前] に書いた自分のテーマを選ぶ。
// 自分のテーマは base の組み込みのテーマに、書いた色だけ重ねる。書き間違いは、その所だけ base に戻して知らせる。

import { t } from "./i18n";

/** 設定ファイルの [theme]（サーバーはそのまま渡す）。 */
export interface ThemeConfig {
  name?: string;
}

/** 設定ファイルの [themes.名前]（サーバーは中身を確かめずにそのまま渡す）。 */
export type CustomThemes = Record<string, Record<string, unknown>>;

/** 端末の16色の名前（ANSI の 0〜15 の順）。 */
export const ANSI_NAMES = [
  "black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
  "bright_black", "bright_red", "bright_green", "bright_yellow", "bright_blue", "bright_magenta", "bright_cyan", "bright_white",
] as const;

export interface Theme {
  scheme: "dark" | "light";
  /** 端末の文字・背景・カーソルと16色。 */
  term: { fg: string; bg: string; cursor: string; colors: string[] };
  /** ボタンや帯の色（style.css の :root の変数と同じ名前）。 */
  ui: {
    "bg": string;
    "bar-bg": string;
    "btn-bg": string;
    "btn-fg": string;
    "btn-active": string;
    "accent": string;
    "accent-dim": string;
    "on-accent": string;
    "panel-bg": string;
    "muted-fg": string;
    "dim-fg": string;
    "label-fg": string;
    "input-bg": string;
    "input-border": string;
    "toast-bg": string;
    "toast-fg": string;
  };
}

const SOLARIZED = [
  "#073642", "#dc322f", "#859900", "#b58900", "#268bd2", "#d33682", "#2aa198", "#eee8d5",
  "#002b36", "#cb4b16", "#586e75", "#657b83", "#839496", "#6c71c4", "#93a1a1", "#fdf6e3",
];

export const THEMES: Record<string, Theme> = {
  // 今までの色（端末は wterm の既定の色、ボタンは style.css の :root と同じ）。
  "default": {
    scheme: "dark",
    term: {
      fg: "#d4d4d4", bg: "#1e1e1e", cursor: "#aeafad",
      colors: [
        "#1e1e1e", "#f44747", "#6a9955", "#d7ba7d", "#569cd6", "#c586c0", "#4ec9b0", "#d4d4d4",
        "#808080", "#f44747", "#6a9955", "#d7ba7d", "#569cd6", "#c586c0", "#4ec9b0", "#ffffff",
      ],
    },
    ui: {
      "bg": "#1e1e1e", "bar-bg": "#2a2a2a", "btn-bg": "#3a3a3a", "btn-fg": "#e6e6e6", "btn-active": "#555555",
      "accent": "#3fb950", "accent-dim": "#1f3325", "on-accent": "#000000", "panel-bg": "rgba(30, 36, 32, 0.97)",
      "muted-fg": "#cccccc", "dim-fg": "#aaaaaa", "label-fg": "#99aa99", "input-bg": "#111111", "input-border": "#555555",
      "toast-bg": "#444444", "toast-fg": "#e6e6e6",
    },
  },
  "light": {
    scheme: "light",
    term: {
      fg: "#1f2328", bg: "#ffffff", cursor: "#1f2328",
      colors: [
        "#24292f", "#cf222e", "#116329", "#4d2d00", "#0969da", "#8250df", "#1b7c83", "#6e7781",
        "#57606a", "#a40e26", "#1a7f37", "#633c01", "#218bff", "#a475f9", "#3192aa", "#8c959f",
      ],
    },
    ui: {
      "bg": "#ffffff", "bar-bg": "#eff1f3", "btn-bg": "#dde1e5", "btn-fg": "#1f2328", "btn-active": "#c4cad0",
      "accent": "#1a7f37", "accent-dim": "#d4efdb", "on-accent": "#ffffff", "panel-bg": "rgba(246, 250, 247, 0.97)",
      "muted-fg": "#424a53", "dim-fg": "#57606a", "label-fg": "#4b6b53", "input-bg": "#ffffff", "input-border": "#afb8c1",
      "toast-bg": "#24292f", "toast-fg": "#f6f8fa",
    },
  },
  "solarized-dark": {
    scheme: "dark",
    term: { fg: "#839496", bg: "#002b36", cursor: "#93a1a1", colors: SOLARIZED },
    ui: {
      "bg": "#002b36", "bar-bg": "#073642", "btn-bg": "#0e4b5a", "btn-fg": "#eee8d5", "btn-active": "#1a6273",
      "accent": "#859900", "accent-dim": "#26400f", "on-accent": "#002b36", "panel-bg": "rgba(7, 54, 66, 0.97)",
      "muted-fg": "#93a1a1", "dim-fg": "#839496", "label-fg": "#8fa37a", "input-bg": "#001f27", "input-border": "#386b78",
      "toast-bg": "#586e75", "toast-fg": "#fdf6e3",
    },
  },
  "solarized-light": {
    scheme: "light",
    term: { fg: "#657b83", bg: "#fdf6e3", cursor: "#586e75", colors: SOLARIZED },
    ui: {
      "bg": "#fdf6e3", "bar-bg": "#eee8d5", "btn-bg": "#e0d9c3", "btn-fg": "#073642", "btn-active": "#cfc7ae",
      "accent": "#859900", "accent-dim": "#e6e9c0", "on-accent": "#fdf6e3", "panel-bg": "rgba(238, 232, 213, 0.97)",
      "muted-fg": "#586e75", "dim-fg": "#657b83", "label-fg": "#6b7a3a", "input-bg": "#fffbf0", "input-border": "#93a1a1",
      "toast-bg": "#073642", "toast-fg": "#fdf6e3",
    },
  },
};

const COLOR = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i;

/** "#rgb" を "#rrggbb" に直す（小文字にそろえる）。 */
function normalize(color: string): string {
  const hex = color.slice(1).toLowerCase();
  return "#" + (hex.length === 3 ? [...hex].map((c) => c + c).join("") : hex);
}

/** 2つの色を混ぜる（a を ratio の割合で）。 */
function mix(a: string, b: string, ratio: number): string {
  const ch = (c: string, i: number) => parseInt(c.slice(1 + i * 2, 3 + i * 2), 16);
  return "#" + [0, 1, 2].map((i) => Math.round(ch(a, i) * ratio + ch(b, i) * (1 - ratio)).toString(16).padStart(2, "0")).join("");
}

/** 自分のテーマに書ける色の名前。 */
const COLOR_KEYS = ["background", "foreground", "cursor", "accent", ...ANSI_NAMES];

function copyTheme(base: Theme): Theme {
  return { scheme: base.scheme, term: { ...base.term, colors: [...base.term.colors] }, ui: { ...base.ui } };
}

/** 選べるテーマの名前（組み込みのものの後ろに、自分のテーマを書いた順に。組み込みと同じ名前のものは除く）。 */
export function themeNames(customs: CustomThemes = {}): string[] {
  return [...Object.keys(THEMES), ...Object.keys(customs).filter((name) => !Object.hasOwn(THEMES, name))];
}

/** [themes.名前] から色を決める。書き間違いは、その所だけ base の色に戻して理由を返す。 */
function customTheme(name: string, def: Record<string, unknown>): { theme: Theme; errors: string[] } {
  const errors: string[] = [];
  for (const key of Object.keys(def)) {
    if (key !== "base" && !COLOR_KEYS.includes(key)) errors.push(t("themeUnknownKey", { name, key }));
  }
  let base = THEMES.default;
  if (def.base !== undefined) {
    if (typeof def.base === "string" && Object.hasOwn(THEMES, def.base)) base = THEMES[def.base];
    else errors.push(t("themeUnknownBase", { name, base: String(def.base), names: Object.keys(THEMES).join(", ") }));
  }
  const theme = copyTheme(base);

  const color = (key: string): string | undefined => {
    const v = def[key];
    if (v === undefined || v === "") return undefined;
    if (typeof v === "string" && COLOR.test(v)) return normalize(v);
    errors.push(t("themeBadColor", { name, key, value: String(v) }));
    return undefined;
  };
  const bg = color("background");
  if (bg) theme.term.bg = theme.ui.bg = bg;
  const fg = color("foreground");
  if (fg) theme.term.fg = fg;
  const cursor = color("cursor");
  if (cursor) theme.term.cursor = cursor;
  const accent = color("accent");
  if (accent) {
    theme.ui.accent = accent;
    theme.ui["accent-dim"] = mix(accent, theme.ui["bar-bg"], 0.25);
  }
  ANSI_NAMES.forEach((key, i) => {
    const c = color(key);
    if (c) theme.term.colors[i] = c;
  });
  return { theme, errors };
}

/**
 * [theme] の name で使う色を決める。組み込みのテーマが先で、無ければ [themes.名前]。
 * どちらにも無い名前は知らせて default にする。errors には選んだ自分のテーマの書き間違いも入る。
 */
export function resolveTheme(name: string | undefined, customs: CustomThemes = {}): { theme: Theme; errors: string[] } {
  if (name === undefined || name === "") return { theme: copyTheme(THEMES.default), errors: [] };
  if (Object.hasOwn(THEMES, name)) return { theme: copyTheme(THEMES[name]), errors: [] };
  const def = Object.hasOwn(customs, name) ? customs[name] : undefined;
  if (def && typeof def === "object") return customTheme(name, def);
  return { theme: copyTheme(THEMES.default), errors: [t("themeUnknown", { name, names: themeNames(customs).join(", ") })] };
}

/** [themes] の書き間違い（画面を開いたときに知らせる）。skip の名前のもの（resolveTheme で知らせたもの）は除く。 */
export function themeProblems(customs: CustomThemes = {}, skip?: string): string[] {
  const errors: string[] = [];
  for (const [name, def] of Object.entries(customs)) {
    if (Object.hasOwn(THEMES, name)) errors.push(t("themeSameAsBuiltin", { name }));
    else if (name !== skip) errors.push(...customTheme(name, def ?? {}).errors);
  }
  return errors;
}

/** 色を画面に当てる。ボタンの色は root（:root）に、端末の色は端末の要素に（wterm の .wterm の既定より強く）。 */
export function applyTheme(theme: Theme, root: HTMLElement, termEl: HTMLElement) {
  for (const [name, value] of Object.entries(theme.ui)) root.style.setProperty(`--${name}`, value);
  root.style.colorScheme = theme.scheme;
  termEl.style.setProperty("--term-fg", theme.term.fg);
  termEl.style.setProperty("--term-bg", theme.term.bg);
  termEl.style.setProperty("--term-cursor", theme.term.cursor);
  theme.term.colors.forEach((c, i) => termEl.style.setProperty(`--term-color-${i}`, c));
  document.querySelector('meta[name="theme-color"]')?.setAttribute("content", theme.ui.bg);
}
