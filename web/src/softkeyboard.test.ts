import { describe, expect, it } from "vitest";
import { SoftKeyboard } from "./softkeyboard";

describe("SoftKeyboard", () => {
  it("同じ幅で大きく縮んだらキーボードが出ている", () => {
    const k = new SoftKeyboard();
    expect(k.visible(400, 800)).toBe(false);
    expect(k.visible(400, 760)).toBe(false); // アドレスバーの出し入れ程度
    expect(k.visible(400, 450)).toBe(true);
    expect(k.visible(400, 800)).toBe(false);
  });

  it("向きを変えたら、その幅での高さで比べる", () => {
    const k = new SoftKeyboard();
    k.visible(400, 800);
    expect(k.visible(800, 400)).toBe(false); // 横向きにしただけ
    expect(k.visible(800, 200)).toBe(true);
    expect(k.visible(400, 800)).toBe(false);
  });
});
