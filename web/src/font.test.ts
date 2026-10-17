// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { applyFont, fontErrorKey, fontFamily, SERVED_FONT } from "./font";

describe("設定ファイルの [font]", () => {
  it("書いていなければ何も変えない（JetBrains Mono のまま）", () => {
    expect(fontFamily(undefined)).toBeNull();
    expect(fontFamily([])).toBeNull();
    expect(fontFamily([{ family: " " }])).toBeNull();
    const root = document.createElement("div");
    applyFont([], root);
    expect(root.style.getPropertyValue("--mono-font")).toBe("");
  });

  it("PC から配られたフォントを先に、次に family の名前（スマホに入っていれば）、最後に JetBrains Mono", () => {
    const stack = fontFamily([{ url: "/api/font?i=0&v=1", family: "Liberation Mono" }])!;
    const names = stack.split(", ");
    expect(names.indexOf(`"${SERVED_FONT} 1"`)).toBeLessThan(names.indexOf(`"Liberation Mono"`));
    expect(names.indexOf(`"Liberation Mono"`)).toBeLessThan(names.indexOf(`"JetBrains Mono"`));
  });

  it("PC に無ければ、family の名前でスマホのフォントを探し、無ければ JetBrains Mono", () => {
    const stack = fontFamily([{ family: "Menlo" }])!;
    expect(stack).not.toContain(SERVED_FONT);
    expect(stack.indexOf(`"Menlo"`)).toBeLessThan(stack.indexOf(`"JetBrains Mono"`));
    const root = document.createElement("div");
    applyFont([{ family: "Menlo" }], root);
    expect(root.style.getPropertyValue("--mono-font")).toBe(stack);
  });

  it("stack の並びは書いた順に置く（ブラウザは文字ごとに、先頭からその文字を持つフォントで描く）", () => {
    const stack = fontFamily([
      { url: "/api/font?i=0" },
      { url: "/api/font?i=1", family: "Noto Sans Mono CJK JP" },
      { family: "Phone Only" },
      { family: "JetBrains Mono" },
    ])!;
    expect(stack).toBe(
      `"Nerd Symbols", "${SERVED_FONT} 1", "${SERVED_FONT} 2", "Noto Sans Mono CJK JP", "Phone Only", "JetBrains Mono", ` +
        `"JetBrains Mono", "Noto Sans Mono CJK JP", "Noto Sans CJK JP", "Hiragino Sans", ui-monospace, monospace`,
    );
  });

  it("選び直して [font] が無くなったら既定（JetBrains Mono）に戻す", () => {
    const root = document.createElement("div");
    applyFont([{ family: "Menlo" }], root);
    applyFont(undefined, root);
    expect(root.style.getPropertyValue("--mono-font")).toBe("");
  });

  it("Nerd Font のアイコンは、どのフォントにしても Nerd Symbols で描く（いちばん先に置く）", () => {
    for (const stack of [fontFamily([{ url: "/api/font", family: "Some Nerd Font" }]), fontFamily([{ family: "X" }])]) {
      expect(stack!.startsWith(`"Nerd Symbols", `)).toBe(true);
    }
  });

  it("名前に \" や \\ が入っていても、ほかの指定に化けない", () => {
    expect(fontFamily([{ family: `a", serif; x: "y\\` }])).toContain(`"a\\", serif; x: \\"y\\\\"`);
  });

  it("使えないファイルの知らせは、stack なら飛ばしたこと、版 1 なら family か JetBrains Mono で描くこと", () => {
    expect(fontErrorKey({ stack: ["a.ttf"] })).toBe("fontStackError");
    expect(fontErrorKey({ file: "a.ttf" })).toBe("fontFileError");
  });
});
