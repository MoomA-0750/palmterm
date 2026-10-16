// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { applyFont, fontFamily, SERVED_FONT } from "./font";

describe("設定ファイルの [font]", () => {
  it("書いていなければ何も変えない（JetBrains Mono のまま）", () => {
    expect(fontFamily(undefined, undefined)).toBeNull();
    expect(fontFamily({ family: " " }, undefined)).toBeNull();
    const root = document.createElement("div");
    applyFont({}, undefined, root);
    expect(root.style.getPropertyValue("--mono-font")).toBe("");
  });

  it("PC から配られたフォントを先に、次に family の名前（スマホに入っていれば）、最後に JetBrains Mono", () => {
    const stack = fontFamily({ family: "Liberation Mono" }, "/api/font?v=1")!;
    const names = stack.split(", ");
    expect(names.indexOf(`"${SERVED_FONT}"`)).toBeLessThan(names.indexOf(`"Liberation Mono"`));
    expect(names.indexOf(`"Liberation Mono"`)).toBeLessThan(names.indexOf(`"JetBrains Mono"`));
  });

  it("PC に無ければ、family の名前でスマホのフォントを探し、無ければ JetBrains Mono", () => {
    const stack = fontFamily({ family: "Menlo" }, undefined)!;
    expect(stack).not.toContain(SERVED_FONT);
    expect(stack.indexOf(`"Menlo"`)).toBeLessThan(stack.indexOf(`"JetBrains Mono"`));
    const root = document.createElement("div");
    applyFont({ family: "Menlo" }, undefined, root);
    expect(root.style.getPropertyValue("--mono-font")).toBe(stack);
  });

  it("Nerd Font のアイコンは、どのフォントにしても Nerd Symbols で描く（いちばん先に置く）", () => {
    for (const stack of [fontFamily({ family: "Some Nerd Font" }, "/api/font"), fontFamily({ family: "X" }, undefined)]) {
      expect(stack!.startsWith(`"Nerd Symbols", `)).toBe(true);
    }
  });

  it("名前に \" や \\ が入っていても、ほかの指定に化けない", () => {
    expect(fontFamily({ family: `a", serif; x: "y\\` }, undefined)).toContain(`"a\\", serif; x: \\"y\\\\"`);
  });
});
