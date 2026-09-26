// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { setupNerdIcons } from "./nerd";

/** wterm と同じく、1文字ずつ span にした行を作る。 */
function row(cells: string[]): HTMLElement {
  const r = document.createElement("div");
  r.className = "term-row";
  for (const c of cells) {
    const s = document.createElement("span");
    s.textContent = c;
    r.append(s);
  }
  return r;
}

const wide = (r: Element) => [...r.querySelectorAll("span")].map((s) => s.classList.contains("nf-wide"));
const flush = () => new Promise((r) => setTimeout(r, 0));

describe("setupNerdIcons", () => {
  it("次のマスが空白か行末のアイコンだけを nf-wide にする", () => {
    const term = document.createElement("div");
    // U+F07B（フォルダ）、U+E0B0（Powerline の区切り：常に1マス）
    const r = row(["", " ", "", "x", "a", "", " ", ""]);
    term.append(r);
    setupNerdIcons(term);
    expect(wide(r)).toEqual([true, false, false, false, false, false, false, true]);
  });

  it("描き直された行と、追加された行を直す", async () => {
    const term = document.createElement("div");
    const r = row(["", " "]);
    term.append(r);
    setupNerdIcons(term);
    expect(wide(r)).toEqual([true, false]);

    // 隣が文字になったら外す
    r.children[1].textContent = "x";
    await flush();
    expect(wide(r)).toEqual([false, false]);

    // アイコンが普通の文字に変わったら外す
    r.children[1].textContent = " ";
    await flush();
    expect(wide(r)).toEqual([true, false]);
    r.children[0].textContent = "a";
    await flush();
    expect(wide(r)).toEqual([false, false]);

    const added = row([""]);
    term.append(added);
    await flush();
    expect(wide(added)).toEqual([true]);
  });
});
