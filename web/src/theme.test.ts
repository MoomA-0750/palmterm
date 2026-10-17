// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from "vitest";
import { setLang } from "./i18n";
import { applyTheme, resolveTheme, themeNames, themeProblems, THEMES } from "./theme";

describe("設定ファイルの [theme]", () => {
  beforeEach(() => setLang("en"));

  it("書いていなければ default（今までの色）", () => {
    const { theme, errors } = resolveTheme(undefined);
    expect(errors).toEqual([]);
    expect(theme).toEqual(THEMES.default);
    expect(theme.term.bg).toBe("#1e1e1e");
    expect(theme.ui.accent).toBe("#3fb950");
  });

  it("name で組み込みのテーマを選ぶ。明るいテーマもある", () => {
    expect(Object.keys(THEMES)).toEqual([
      "default", "light", "solarized-dark", "solarized-light",
      "rose-pine", "rose-pine-moon", "rose-pine-dawn", "catppuccin-latte", "catppuccin-frappe", "catppuccin-macchiato", "catppuccin-mocha",
    ]);
    const { theme, errors } = resolveTheme("light");
    expect(errors).toEqual([]);
    expect(theme.scheme).toBe("light");
    expect(theme.term.bg).toBe("#ffffff");
    expect(theme.ui["btn-bg"]).not.toBe(THEMES.default.ui["btn-bg"]);
    for (const t of Object.values(THEMES)) expect(t.term.colors).toHaveLength(16);
  });

  it("Rosé Pine と Catppuccin は公式の配色（端末の色は公式の Alacritty のテーマと同じ）。Dawn と Latte は明るいテーマ", () => {
    const want: Record<string, { scheme: string; bg: string; fg: string; colors: string[] }> = {
      "rose-pine": { scheme: "dark", bg: "#191724", fg: "#e0def4", colors: ["#26233a", "#eb6f92", "#31748f", "#f6c177", "#9ccfd8", "#c4a7e7", "#ebbcba", "#e0def4"] },
      "rose-pine-moon": { scheme: "dark", bg: "#232136", fg: "#e0def4", colors: ["#393552", "#eb6f92", "#3e8fb0", "#f6c177", "#9ccfd8", "#c4a7e7", "#ea9a97", "#e0def4"] },
      "rose-pine-dawn": { scheme: "light", bg: "#faf4ed", fg: "#575279", colors: ["#f2e9e1", "#b4637a", "#286983", "#ea9d34", "#56949f", "#907aa9", "#d7827e", "#575279"] },
      "catppuccin-latte": { scheme: "light", bg: "#eff1f5", fg: "#4c4f69", colors: ["#bcc0cc", "#d20f39", "#40a02b", "#df8e1d", "#1e66f5", "#ea76cb", "#179299", "#5c5f77"] },
      "catppuccin-frappe": { scheme: "dark", bg: "#303446", fg: "#c6d0f5", colors: ["#51576d", "#e78284", "#a6d189", "#e5c890", "#8caaee", "#f4b8e4", "#81c8be", "#b5bfe2"] },
      "catppuccin-macchiato": { scheme: "dark", bg: "#24273a", fg: "#cad3f5", colors: ["#494d64", "#ed8796", "#a6da95", "#eed49f", "#8aadf4", "#f5bde6", "#8bd5ca", "#b8c0e0"] },
      "catppuccin-mocha": { scheme: "dark", bg: "#1e1e2e", fg: "#cdd6f4", colors: ["#45475a", "#f38ba8", "#a6e3a1", "#f9e2af", "#89b4fa", "#f5c2e7", "#94e2d5", "#bac2de"] },
    };
    for (const [name, w] of Object.entries(want)) {
      const { theme, errors } = resolveTheme(name);
      expect(errors).toEqual([]);
      expect([name, theme.scheme, theme.term.bg, theme.term.fg, theme.ui.bg]).toEqual([name, w.scheme, w.bg, w.fg, w.bg]);
      expect(theme.term.colors.slice(0, 8)).toEqual(w.colors);
      expect(theme.ui["btn-bg"]).not.toBe(THEMES.default.ui["btn-bg"]);
    }
  });

  it("知らないテーマの名前は知らせ、default を使う", () => {
    setLang("ja");
    const { theme, errors } = resolveTheme("nope", { mine: {} });
    expect(errors).toHaveLength(1);
    expect(errors[0]).toContain("知らないテーマです: nope");
    expect(errors[0]).toContain("catppuccin-mocha, mine");
    expect(theme).toEqual(THEMES.default);
  });
});

describe("設定ファイルの [themes.名前]（自分のテーマ）", () => {
  beforeEach(() => setLang("en"));

  it("base のテーマに書いた色だけ重ねる。ほかは base の色のまま", () => {
    const { theme, errors } = resolveTheme("my-dark", { "my-dark": { base: "solarized-dark", accent: "#ff5f87" } });
    expect(errors).toEqual([]);
    expect(theme.ui.accent).toBe("#ff5f87");
    expect(theme.ui["accent-dim"]).not.toBe(THEMES["solarized-dark"].ui["accent-dim"]);
    expect(theme.term).toEqual(THEMES["solarized-dark"].term);
    expect({ ...theme.ui, accent: "", "accent-dim": "" }).toEqual({ ...THEMES["solarized-dark"].ui, accent: "", "accent-dim": "" });
    expect(theme.scheme).toBe("dark");
    // 組み込みのテーマ自体は書き換えない
    expect(THEMES["solarized-dark"].ui.accent).toBe("#859900");
  });

  it("どの色も書ける。base を書かなければ default をもとにする", () => {
    const { theme, errors } = resolveTheme("mine", {
      mine: { background: "#FAFAFA", foreground: "#123", cursor: "#ff0000", accent: "#0969da", red: "#aa0000", bright_white: "#eeeeee" },
    });
    expect(errors).toEqual([]);
    expect(theme.term.bg).toBe("#fafafa");
    expect(theme.ui.bg).toBe("#fafafa");
    expect(theme.term.fg).toBe("#112233");
    expect(theme.term.cursor).toBe("#ff0000");
    expect(theme.ui.accent).toBe("#0969da");
    expect(theme.term.colors[1]).toBe("#aa0000");
    expect(theme.term.colors[15]).toBe("#eeeeee");
    const untouched = THEMES.default.term.colors.filter((_, i) => i !== 1 && i !== 15);
    expect(theme.term.colors.filter((_, i) => i !== 1 && i !== 15)).toEqual(untouched);
    expect(theme.ui["btn-bg"]).toBe(THEMES.default.ui["btn-bg"]);
  });

  it("書き間違えた色は知らせ、その色だけ base の色に戻す", () => {
    const { theme, errors } = resolveTheme("mine", { mine: { base: "light", red: "#12345", background: "white", green: "#00aa00", blue: 5 } });
    expect(errors).toEqual([
      "Config file, [themes.mine] background: write colors as #rrggbb or #rgb: white. Using the base theme's color",
      "Config file, [themes.mine] red: write colors as #rrggbb or #rgb: #12345. Using the base theme's color",
      "Config file, [themes.mine] blue: write colors as #rrggbb or #rgb: 5. Using the base theme's color",
    ]);
    expect(theme.term.colors[1]).toBe(THEMES.light.term.colors[1]);
    expect(theme.term.colors[4]).toBe(THEMES.light.term.colors[4]);
    expect(theme.term.bg).toBe(THEMES.light.term.bg);
    expect(theme.term.colors[2]).toBe("#00aa00");
  });

  it("知らない項目と知らない base は知らせる。知らない base なら default をもとにする", () => {
    const { theme, errors } = resolveTheme("mine", { mine: { base: "dracula", backgroud: "#000000", accent: "#ff8800" } });
    expect(errors).toEqual([
      "Config file, [themes.mine]: unknown key: backgroud (ignored)",
      "Config file, [themes.mine] base: unknown theme: dracula (use one of default, light, solarized-dark, solarized-light, rose-pine, rose-pine-moon, rose-pine-dawn, catppuccin-latte, catppuccin-frappe, catppuccin-macchiato, catppuccin-mocha). Based on default",
    ]);
    expect(theme.term).toEqual(THEMES.default.term);
    expect(theme.ui.accent).toBe("#ff8800");
  });

  it("組み込みと同じ名前は知らせ、組み込みのテーマを使う。選べる名前にも出さない", () => {
    const customs = { light: { accent: "#ff0000" }, mine: {} };
    expect(resolveTheme("light", customs)).toEqual({ theme: THEMES.light, errors: [] });
    expect(themeProblems(customs)).toEqual(["Config file, [themes.light]: a built-in theme has this name. Using the built-in light"]);
    expect(themeNames(customs)).toEqual([
      "default", "light", "solarized-dark", "solarized-light",
      "rose-pine", "rose-pine-moon", "rose-pine-dawn", "catppuccin-latte", "catppuccin-frappe", "catppuccin-macchiato", "catppuccin-mocha", "mine",
    ]);
  });

  it("画面を開いたときは、選んでいない自分のテーマの書き間違いも知らせる（選んだものは resolveTheme の方で）", () => {
    const customs = { a: { red: "x" }, b: { glow: true }, c: { base: "light" } };
    expect(themeProblems(customs, "a")).toEqual(["Config file, [themes.b]: unknown key: glow (ignored)"]);
    expect(themeProblems(customs)).toHaveLength(2);
    expect(themeProblems(undefined)).toEqual([]);
  });

  it("画面に当てると、ボタンの色と端末の色の変数が変わる", () => {
    document.head.innerHTML = '<meta name="theme-color" content="#1e1e1e" />';
    const term = document.createElement("div");
    const { theme } = resolveTheme("mine", { mine: { base: "light", red: "#aa0000" } });
    applyTheme(theme, document.documentElement, term);
    const root = document.documentElement.style;
    expect(root.getPropertyValue("--bg")).toBe("#ffffff");
    expect(root.getPropertyValue("--btn-bg")).toBe(THEMES.light.ui["btn-bg"]);
    expect(root.getPropertyValue("--accent")).toBe(THEMES.light.ui.accent);
    expect(term.style.getPropertyValue("--term-bg")).toBe("#ffffff");
    expect(term.style.getPropertyValue("--term-fg")).toBe(THEMES.light.term.fg);
    expect(term.style.getPropertyValue("--term-color-1")).toBe("#aa0000");
    expect(document.querySelector('meta[name="theme-color"]')?.getAttribute("content")).toBe("#ffffff");
  });
});
