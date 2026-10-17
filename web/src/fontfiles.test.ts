// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { filterFonts, fontConfigLine, fontSize, renderFonts } from "./fontfiles";
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

  it("組み込みの JetBrains Mono・足したフォント・PC のフォントを出どころの印を付けて並べ、PC と組み込みの「使う」は名前を渡す", () => {
    setLang("ja");
    const list = document.createElement("ul");
    const named: string[] = [];
    const data = { dir: "", file: "Liberation Mono", fonts: [{ name: "A.ttf", size: 1 }], system: ["DejaVu Sans Mono", "Liberation Mono"], builtin: "JetBrains Mono" };
    renderFonts(list, data, () => {}, () => {}, (name) => named.push(name));
    const rows = [...list.children] as HTMLElement[];
    expect(rows.map((r) => r.className)).toEqual(["font-default", "font-builtin", "font-file", "font-system", "font-system in-use"]);
    expect(rows.slice(1).map((r) => r.querySelector(".font-source")?.textContent)).toEqual(["組み込み", "追加", "PC", "PC"]);
    expect(rows.slice(1).map((r) => r.querySelector(".font-name")?.textContent)).toEqual(["JetBrains Mono", "A.ttf", "DejaVu Sans Mono", "Liberation Mono（使用中）"]);
    expect(rows[3].querySelector(".font-meta")?.textContent).toBe('stack = ["DejaVu Sans Mono"]');
    // 使っている PC のフォントには「使う」を出さず、PC のフォントには「削除」も無い
    expect(rows[4].querySelector(".font-use")).toBeNull();
    expect(list.querySelectorAll(".font-system .font-delete").length).toBe(0);
    rows[3].querySelector<HTMLButtonElement>(".font-use")!.click();
    rows[1].querySelector<HTMLButtonElement>(".font-use")!.click();
    expect(named).toEqual(["DejaVu Sans Mono", "JetBrains Mono"]);

    // 並びの先頭の名前は大文字小文字を問わず「使用中」。足したフォントが無くても PC と組み込みの行は出す
    renderFonts(list, { ...data, file: "jetbrains mono", fonts: [] }, () => {});
    expect(list.querySelector(".font-builtin")?.classList.contains("in-use")).toBe(true);
    expect([...list.children].map((r) => r.className)).toEqual(["font-default", "font-builtin in-use", "fonts-empty", "font-system", "font-system"]);
  });

  it("名前で絞り込む（大文字小文字は問わない）。既定のフォントの行と案内はいつも出す", () => {
    setLang("en");
    const list = document.createElement("ul");
    renderFonts(list, { dir: "", file: "x", fonts: [{ name: "MyMono.ttf", size: 1 }], system: ["DejaVu Sans Mono", "Noto Sans CJK JP"], builtin: "JetBrains Mono" }, () => {});
    const shown = () => [...list.children].filter((r) => !(r as HTMLElement).hidden).map((r) => r.querySelector(".font-name")?.textContent);
    filterFonts(list, " MONO ");
    expect(shown()).toEqual(["Default font", "JetBrains Mono", "MyMono.ttf", "DejaVu Sans Mono"]);
    filterFonts(list, "noto");
    expect(shown()).toEqual(["Default font", "Noto Sans CJK JP"]);
    filterFonts(list, "");
    expect(shown().length).toBe(5);
  });

  it("設定ファイルの行と大きさの書き方", () => {
    expect(fontConfigLine("My_Font.otf")).toBe('file = "fonts/My_Font.otf"');
    expect(fontSize(500)).toBe("500 B");
    expect(fontSize(1536 * 1024)).toBe("1.5 MB");
  });
});
