// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from "vitest";
import { setLang } from "./i18n";
import { applyTheme, resolveTheme, THEMES } from "./theme";

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
    expect(Object.keys(THEMES)).toEqual(["default", "light", "solarized-dark", "solarized-light"]);
    const { theme, errors } = resolveTheme({ name: "light" });
    expect(errors).toEqual([]);
    expect(theme.scheme).toBe("light");
    expect(theme.term.bg).toBe("#ffffff");
    expect(theme.ui["btn-bg"]).not.toBe(THEMES.default.ui["btn-bg"]);
    for (const t of Object.values(THEMES)) expect(t.term.colors).toHaveLength(16);
  });

  it("書いた色だけ上書きし、ほかはテーマの色のまま", () => {
    const { theme, errors } = resolveTheme({
      name: "light", background: "#FAFAFA", foreground: "#123", cursor: "#ff0000", accent: "#0969da",
      red: "#aa0000", bright_white: "#eeeeee",
    });
    expect(errors).toEqual([]);
    expect(theme.term.bg).toBe("#fafafa");
    expect(theme.ui.bg).toBe("#fafafa");
    expect(theme.term.fg).toBe("#112233");
    expect(theme.term.cursor).toBe("#ff0000");
    expect(theme.ui.accent).toBe("#0969da");
    expect(theme.ui["accent-dim"]).not.toBe(THEMES.light.ui["accent-dim"]);
    expect(theme.term.colors[1]).toBe("#aa0000");
    expect(theme.term.colors[15]).toBe("#eeeeee");
    const untouched = THEMES.light.term.colors.filter((_, i) => i !== 1 && i !== 15);
    expect(theme.term.colors.filter((_, i) => i !== 1 && i !== 15)).toEqual(untouched);
    expect(theme.ui["btn-bg"]).toBe(THEMES.light.ui["btn-bg"]);
    // 組み込みのテーマ自体は書き換えない
    expect(THEMES.light.term.colors[1]).toBe("#cf222e");
  });

  it("書き間違えた色は知らせ、その色だけ既定に戻す", () => {
    const { theme, errors } = resolveTheme({ name: "light", red: "#12345", background: "white", green: "#00aa00" });
    expect(errors).toEqual([
      "Config file, [theme] background: write colors as #rrggbb or #rgb: white. Using the default for this color",
      "Config file, [theme] red: write colors as #rrggbb or #rgb: #12345. Using the default for this color",
    ]);
    expect(theme.term.colors[1]).toBe(THEMES.light.term.colors[1]);
    expect(theme.term.bg).toBe(THEMES.light.term.bg);
    expect(theme.term.colors[2]).toBe("#00aa00");
  });

  it("知らないテーマの名前は知らせ、default に書いた色を重ねる", () => {
    setLang("ja");
    const { theme, errors } = resolveTheme({ name: "nope", accent: "#ff8800" });
    expect(errors).toHaveLength(1);
    expect(errors[0]).toContain("知らないテーマです: nope");
    expect(theme.term).toEqual(THEMES.default.term);
    expect(theme.ui.accent).toBe("#ff8800");
  });

  it("画面に当てると、ボタンの色と端末の色の変数が変わる", () => {
    document.head.innerHTML = '<meta name="theme-color" content="#1e1e1e" />';
    const term = document.createElement("div");
    const { theme } = resolveTheme({ name: "light", red: "#aa0000" });
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
