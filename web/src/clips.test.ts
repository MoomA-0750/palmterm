// @vitest-environment happy-dom
import { describe, expect, it, vi } from "vitest";
import { type ClipItem, clipMeta, clipPreview, renderClips } from "./clips";
import { setLang } from "./i18n";

const at = (y: number, mo: number, d: number, h: number, mi: number) => new Date(y, mo - 1, d, h, mi).getTime() / 1000;

describe("clipPreview", () => {
  it("最初の空でない行を、長ければ切って返す", () => {
    expect(clipPreview("\n  \n  hello world  \nnext")).toBe("hello world");
    expect(clipPreview("あいうえお", 3)).toBe("あいう…");
    expect(clipPreview("")).toBe("");
  });
});

describe("clipMeta", () => {
  it("今日なら時:分、前の日なら月/日も付け、文字数を添える", () => {
    setLang("ja");
    const now = new Date(2026, 9, 2, 15, 0);
    expect(clipMeta({ names: ["b"], created: at(2026, 10, 2, 9, 5), text: "日本語" }, now)).toBe("09:05 · 3");
    expect(clipMeta({ names: ["b"], created: at(2026, 9, 30, 23, 59), text: "ab", truncated: true }, now)).toBe(
      "9/30 23:59 · 2 （長いので途中まで）",
    );
  });
});

describe("renderClips", () => {
  const items: ClipItem[] = [
    { names: ["buffer1"], created: at(2026, 10, 2, 9, 0), text: "new" },
    { names: ["buffer0", "buffer2"], created: at(2026, 10, 2, 8, 0), text: "old\ntext" },
  ];

  it("項目ごとにコピー・入力欄へ・削除（2回押し）を出す", () => {
    setLang("ja");
    const list = document.createElement("ul");
    const actions = { copy: vi.fn(), toLine: vi.fn(), remove: vi.fn() };
    renderClips(list, items, actions);
    const lis = list.querySelectorAll("li");
    expect(lis).toHaveLength(2);
    expect(lis[1].querySelector(".clip-text")!.textContent).toBe("old\ntext");

    lis[0].querySelector<HTMLButtonElement>(".clip-copy")!.click();
    expect(actions.copy).toHaveBeenCalledWith(items[0]);
    lis[1].querySelector<HTMLButtonElement>(".clip-to-line")!.click();
    expect(actions.toLine).toHaveBeenCalledWith(items[1]);

    const del = lis[1].querySelector<HTMLButtonElement>(".clip-delete")!;
    del.click();
    expect(actions.remove).not.toHaveBeenCalled();
    expect(del.textContent).toBe("消す?");
    del.click();
    expect(actions.remove).toHaveBeenCalledWith(items[1]);
  });

  it("空なら説明を出す", () => {
    setLang("en");
    const list = document.createElement("ul");
    renderClips(list, [], { copy: vi.fn(), toLine: vi.fn(), remove: vi.fn() });
    expect(list.textContent).toMatch(/Nothing yet/);
  });
});
