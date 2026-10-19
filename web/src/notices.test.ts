// @vitest-environment happy-dom
import { describe, expect, it, vi } from "vitest";
import { setLang } from "./i18n";
import { setupNotices } from "./notices";

describe("palmterm notify の帯", () => {
  it("ウィンドウの番号・名前と文を出し、押すとそのウィンドウに切り替えて消える。× でも消える", () => {
    setLang("ja");
    const box = document.createElement("div");
    const select = vi.fn();
    const notices = setupNotices(box, select);
    notices.show({ text: "ビルドが終わった", window: 2, windowName: "build", windowId: "@7" });
    notices.show({ text: "" });
    const bars = [...box.querySelectorAll(".notifybar")];
    expect(bars.map((b) => b.textContent)).toEqual(["終わりました", "2:buildビルドが終わった"]);
    expect(bars[1].querySelector(".notifybar-window")?.textContent).toBe("2:build");

    (bars[1].querySelector(".notifybar-body") as HTMLButtonElement).click();
    expect(select).toHaveBeenCalledWith(2, "@7");
    expect(box.childElementCount).toBe(1);

    // ウィンドウの無い帯は押しても切り替えない
    (bars[0].querySelector(".notifybar-body") as HTMLButtonElement).click();
    expect(select).toHaveBeenCalledTimes(1);
    expect(box.childElementCount).toBe(0);

    notices.show({ text: "x" });
    (box.querySelector(".notifybar-close") as HTMLButtonElement).click();
    expect(box.childElementCount).toBe(0);
  });

  it("文を省いた知らせは設定の言語で「終わりました」。帯は新しい5つまで", () => {
    setLang("en");
    const box = document.createElement("div");
    const notices = setupNotices(box, () => {});
    for (let i = 0; i < 7; i++) notices.show({ text: i ? String(i) : "" });
    expect([...box.children].map((b) => b.textContent)).toEqual(["6", "5", "4", "3", "2"]);
    notices.show({});
    expect(box.firstElementChild?.textContent).toBe("Done");
    setLang("ja");
  });
});
