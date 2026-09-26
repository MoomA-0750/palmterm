import { afterEach, describe, expect, it, vi } from "vitest";
import { applyToChar, isSingleChar, Modifiers, type ModName, specialKey, withMods } from "./keys";

/** 決まった修飾キーだけが効いている状態。 */
function fixed(...names: ModName[]) {
  return withMods(new Modifiers(), names);
}

describe("applyToChar", () => {
  it.each([
    ["c", [], "c"],
    ["c", ["ctrl"], "\x03"],
    ["C", ["ctrl"], "\x03"],
    ["[", ["ctrl"], "\x1b"],
    [" ", ["ctrl"], "\x00"],
    ["?", ["ctrl"], "\x7f"],
    ["b", ["alt"], "\x1bb"],
    ["a", ["shift"], "A"],
    ["x", ["ctrl", "alt"], "\x1b\x18"],
    ["+", ["ctrl"], "+"], // 送れない組み合わせは修飾なしの文字
  ] as [string, ModName[], string][])("%j + %j", (ch, mods, want) => {
    expect(applyToChar(ch, fixed(...mods))).toBe(want);
  });
});

describe("specialKey", () => {
  it.each([
    ["up", [], false, "\x1b[A"],
    ["up", [], true, "\x1bOA"],
    ["left", ["alt"], true, "\x1b[1;3D"],
    ["right", ["ctrl"], false, "\x1b[1;5C"],
    ["home", [], false, "\x1b[H"],
    ["end", ["shift"], false, "\x1b[1;2F"],
    ["pageup", [], false, "\x1b[5~"],
    ["pagedown", ["ctrl"], false, "\x1b[6;5~"],
    ["insert", [], false, "\x1b[2~"],
    ["delete", ["shift"], false, "\x1b[3;2~"],
    ["f1", [], false, "\x1bOP"],
    ["f4", ["ctrl"], false, "\x1b[1;5S"],
    ["f5", [], false, "\x1b[15~"],
    ["f12", ["alt"], false, "\x1b[24;3~"],
    ["tab", [], false, "\t"],
    ["tab", ["shift"], false, "\x1b[Z"],
    ["tab", ["alt"], false, "\x1b\t"],
    ["esc", [], false, "\x1b"],
    ["esc", ["alt"], false, "\x1b\x1b"],
    ["enter", [], false, "\r"],
    ["enter", ["alt"], false, "\x1b\r"],
    ["backspace", [], false, "\x7f"],
    ["backspace", ["ctrl"], false, "\b"],
  ] as const)("%s %j appCursor=%s", (key, mods, app, want) => {
    expect(specialKey(key, fixed(...mods), app)).toBe(want);
  });
});

describe("withMods", () => {
  it("画面の修飾キーと組み合わせの修飾キーを合わせる", () => {
    const m = new Modifiers();
    m.tap("shift");
    const both = withMods(m, ["ctrl"]);
    expect(both.has("ctrl")).toBe(true);
    expect(both.has("shift")).toBe(true);
    expect(both.has("alt")).toBe(false);
    expect(both.param()).toBe(1 + 1 + 4);
  });
});

describe("Modifiers", () => {
  afterEach(() => vi.restoreAllMocks());

  function clock() {
    let now = 1000;
    vi.spyOn(performance, "now").mockImplementation(() => now);
    return (ms: number) => (now += ms);
  }

  it("1回押すと once、すばやくもう一度で lock、固定中に押すと off", () => {
    const advance = clock();
    const m = new Modifiers();
    const changes = vi.fn();
    m.onChange = changes;
    m.tap("ctrl");
    expect(m.get("ctrl")).toBe("once");
    advance(200);
    m.tap("ctrl");
    expect(m.get("ctrl")).toBe("lock");
    advance(1000);
    m.tap("ctrl");
    expect(m.get("ctrl")).toBe("off");
    expect(changes).toHaveBeenCalledTimes(3);
  });

  it("間を空けて2回目を押すと解除", () => {
    const advance = clock();
    const m = new Modifiers();
    m.tap("alt");
    advance(600);
    m.tap("alt");
    expect(m.get("alt")).toBe("off");
  });

  it("consume は once だけを外す", () => {
    const advance = clock();
    const m = new Modifiers();
    m.tap("ctrl");
    m.tap("shift");
    advance(100);
    m.tap("shift"); // lock
    expect(m.active()).toBe(true);
    expect(m.param()).toBe(1 + 1 + 4);
    const changes = vi.fn();
    m.onChange = changes;
    m.consume();
    expect(m.get("ctrl")).toBe("off");
    expect(m.get("shift")).toBe("lock");
    expect(changes).toHaveBeenCalledTimes(1);
    m.consume();
    expect(changes).toHaveBeenCalledTimes(1); // 変わらなければ知らせない
  });
});

describe("isSingleChar", () => {
  it.each([
    ["a", true],
    ["あ", true],
    ["😀", true],
    ["ab", false],
    ["\x1b[A", false],
    ["\r", false],
    ["\x7f", false],
    ["", false],
  ])("%j → %s", (s, want) => {
    expect(isSingleChar(s)).toBe(want);
  });
});
