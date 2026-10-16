// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { fontConfigLine, fontSize, renderFonts } from "./fontfiles";
import { setLang } from "./i18n";

describe("画面から足したフォントの一覧", () => {
  it("名前・大きさ・設定ファイルに書く行を出し、[font] で使っているものに印を付ける", () => {
    setLang("ja");
    const list = document.createElement("ul");
    renderFonts(list, { dir: "/x/fonts", file: "fonts/B.woff2", fonts: [{ name: "A.ttf", size: 2048 }, { name: "B.woff2", size: 3 * 1024 * 1024 }] }, () => {});
    const rows = [...list.querySelectorAll(".font-file")];
    expect(rows.map((r) => r.querySelector(".font-name")?.textContent)).toEqual(["A.ttf", "B.woff2（使用中）"]);
    expect(rows[0].querySelector(".font-meta")?.textContent).toBe('2 KB · file = "fonts/A.ttf"');
    expect(rows[1].classList.contains("in-use")).toBe(true);
  });

  it("削除は2回押しで、1回目は「消す?」になる", () => {
    setLang("ja");
    const list = document.createElement("ul");
    const removed: string[] = [];
    renderFonts(list, { dir: "", file: "", fonts: [{ name: "A.ttf", size: 1 }] }, (f) => removed.push(f.name));
    const b = list.querySelector<HTMLButtonElement>(".font-delete")!;
    b.click();
    expect(removed).toEqual([]);
    expect(b.textContent).toBe("消す?");
    b.click();
    expect(removed).toEqual(["A.ttf"]);
  });

  it("無ければ案内を出す", () => {
    setLang("en");
    const list = document.createElement("ul");
    renderFonts(list, { dir: "", file: "", fonts: [] }, () => {});
    expect(list.querySelector(".fonts-empty")?.textContent).toContain(".woff2");
  });

  it("「使う」でフォントを選ぶ。使っているものには出さない。先頭の「既定のフォント」は file を消す", () => {
    setLang("ja");
    const list = document.createElement("ul");
    const used: (string | null)[] = [];
    const data = { dir: "", file: "fonts/B.ttf", fonts: [{ name: "A.ttf", size: 1 }, { name: "B.ttf", size: 1 }] };
    renderFonts(list, data, () => {}, (f) => used.push(f?.name ?? null));
    const rows = [...list.children];
    expect(rows[0].classList.contains("font-default")).toBe(true);
    expect(rows.map((r) => r.querySelector(".font-use") !== null)).toEqual([true, true, false]);
    rows[1].querySelector<HTMLButtonElement>(".font-use")!.click();
    rows[0].querySelector<HTMLButtonElement>(".font-use")!.click();
    expect(used).toEqual(["A.ttf", null]);

    // file を書いていなければ「既定のフォント」が使用中
    renderFonts(list, { ...data, file: "" }, () => {});
    expect(list.querySelector(".font-default .font-name")?.textContent).toBe("既定のフォント（使用中）");
    expect(list.querySelector(".font-default .font-use")).toBeNull();
    expect(list.querySelectorAll(".font-file .font-use").length).toBe(2);
  });

  it("設定ファイルの行と大きさの書き方", () => {
    expect(fontConfigLine("My_Font.otf")).toBe('file = "fonts/My_Font.otf"');
    expect(fontSize(500)).toBe("500 B");
    expect(fontSize(1536 * 1024)).toBe("1.5 MB");
  });
});
