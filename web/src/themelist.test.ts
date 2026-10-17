// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from "vitest";
import { setLang } from "./i18n";
import { resolveTheme, THEMES } from "./theme";
import { setupThemes } from "./themelist";

beforeEach(() => document.body.replaceChildren());

/** happy-dom は色を書いたまま返すことがあるので、rgb() にそろえて比べる。 */
function rgb(hex: string): string {
  const n = (i: number) => parseInt(hex.slice(1 + i * 2, 3 + i * 2), 16);
  return `rgb(${n(0)}, ${n(1)}, ${n(2)})`;
}
const sameColor = (css: string, hex: string) => css === hex || css === rgb(hex);

describe("テーマの一覧", () => {
  const customs = { "my-dark": { base: "solarized-dark", accent: "#ff5f87", red: "#ff0000" } };

  function make(over: { accept?: () => boolean } = {}) {
    let current = "default";
    const chosen: string[] = [];
    const list = setupThemes({
      names: () => ["default", "light", "solarized-dark", "solarized-light", "my-dark"],
      current: () => current,
      resolve: (name) => resolveTheme(name, customs).theme,
      choose: async (name) => {
        chosen.push(name);
        const ok = over.accept?.() ?? true;
        if (ok) current = name;
        return ok;
      },
    });
    const items = () => [...list.el.querySelectorAll<HTMLButtonElement>(".theme-item")];
    const on = () => items().filter((b) => b.classList.contains("on")).map((b) => b.dataset.theme);
    return { list, chosen, items, on };
  }

  it("組み込みのテーマの後ろに自分のテーマを並べ、名前と色見本（背景の上に文字の色の見本と16色）を出し、今のものに印を付ける", () => {
    setLang("ja");
    const { list, items, on } = make();
    expect(list.el.hidden).toBe(true);
    list.open();
    expect(list.el.hidden).toBe(false);
    expect(items().map((b) => b.querySelector(".theme-name")?.textContent)).toEqual([
      "既定", "明るい", "Solarized 暗", "Solarized 明", "my-dark",
    ]);
    expect(on()).toEqual(["default"]);
    expect(items()[0].getAttribute("aria-current")).toBe("true");

    const sw = (i: number) => items()[i].querySelector<HTMLElement>(".theme-swatch")!;
    expect(sameColor(sw(1).style.background, THEMES.light.term.bg)).toBe(true);
    expect(sameColor(sw(1).style.color, THEMES.light.term.fg)).toBe(true);
    expect(sw(1).querySelector(".theme-sample")?.textContent).not.toBe("");
    const squares = (i: number) => [...sw(i).querySelectorAll<HTMLElement>(".theme-color")].map((s) => s.style.background);
    expect(squares(2)).toHaveLength(16);
    squares(2).forEach((c, i) => expect(sameColor(c, THEMES["solarized-dark"].term.colors[i])).toBe(true));
    // 自分のテーマは base の色に書いた色を重ねたもの
    expect(sameColor(sw(4).style.background, THEMES["solarized-dark"].term.bg)).toBe(true);
    expect(sameColor(squares(4)[1], "#ff0000")).toBe(true);
  });

  it("行を押すと選び、書き戻せたら印が移る。断られたら移らない。一覧は開いたまま", async () => {
    setLang("en");
    let accept = true;
    const { list, chosen, items, on } = make({ accept: () => accept });
    list.open();
    items()[1].click();
    await new Promise((r) => setTimeout(r));
    expect(chosen).toEqual(["light"]);
    expect(on()).toEqual(["light"]);

    accept = false;
    items()[2].click();
    await new Promise((r) => setTimeout(r));
    expect(chosen).toEqual(["light", "solarized-dark"]);
    expect(on()).toEqual(["light"]);

    accept = true;
    items()[4].click();
    await new Promise((r) => setTimeout(r));
    expect(on()).toEqual(["my-dark"]);
    expect(list.el.hidden).toBe(false);
  });

  it("「閉じる」で隠れる", () => {
    setLang("ja");
    const { list } = make();
    list.open();
    list.el.querySelector<HTMLButtonElement>("#themes-close")!.click();
    expect(list.el.hidden).toBe(true);
  });
});
