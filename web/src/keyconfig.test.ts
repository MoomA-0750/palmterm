// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from "vitest";
import { setLang } from "./i18n";
import { DEFAULT_KEYS, parseKey } from "./keyconfig";

describe("既定のキーバー", () => {
  beforeEach(() => setLang("en"));

  it("並びと表示", () => {
    const keys = DEFAULT_KEYS.map(parseKey);
    expect(keys.map((k) => k.label)).toEqual([
      "Esc", "Tab", "Ctrl", "Alt", "Shift", "Left", "Down", "Up", "Right", "Enter",
      "^C", "^D", "Home", "End", "PgUp", "PgDn", "|", "~", "/", "-", "`",
    ]);
    expect(keys.filter((k) => k.icon).map((k) => k.icon)).toEqual(["left", "down", "up", "right", "enter"]);
    expect(keys.filter((k) => k.repeat).map((k) => k.label)).toEqual(["Left", "Down", "Up", "Right", "PgUp", "PgDn"]);
  });

  it("日本語では矢印の読み上げ名が日本語", () => {
    setLang("ja");
    expect(parseKey({ key: "left" }).label).toBe("左");
  });
});

describe("parseKey", () => {
  beforeEach(() => setLang("en"));

  it.each([
    [{ key: "ctrl+c" }, "^C", { type: "char", char: "c", mods: ["ctrl"] }],
    [{ key: "shift+tab" }, "S-Tab", { type: "key", key: "tab", mods: ["shift"] }],
    [{ key: "alt+left" }, "M-Left", { type: "key", key: "left", mods: ["alt"] }],
    [{ key: "ctrl+space" }, "^Space", { type: "char", char: " ", mods: ["ctrl"] }],
    [{ key: "ctrl++" }, "^+", { type: "char", char: "+", mods: ["ctrl"] }],
    [{ key: "+" }, "+", { type: "char", char: "+", mods: [] }],
    [{ key: "Control+Meta+x" }, "^M-x", { type: "char", char: "x", mods: ["ctrl", "alt"] }],
    [{ key: "F5" }, "F5", { type: "key", key: "f5", mods: [] }],
    [{ key: "delete" }, "Del", { type: "key", key: "delete", mods: [] }],
    [{ mod: "Shift" }, "Shift", { type: "mod", mod: "shift" }],
    [{ text: ":wq\r", label: ":wq" }, ":wq", { type: "text", text: ":wq\r" }],
  ])("%j", (config, label, action) => {
    const k = parseKey(config);
    expect(k.label).toBe(label);
    expect(k.action).toEqual(action);
  });

  it("label・icon・repeat を指定できる", () => {
    const k = parseKey({ key: "up", label: "上へ", icon: "cycle", repeat: false });
    expect(k).toEqual({ label: "上へ", icon: "cycle", repeat: false, action: { type: "key", key: "up", mods: [] } });
    // label を付けたら、既定のアイコンは使わない
    expect(parseKey({ key: "enter", label: "RET" }).icon).toBeUndefined();
    // 修飾つきの矢印もアイコンにしない
    expect(parseKey({ key: "ctrl+left" }).icon).toBeUndefined();
  });

  it.each([
    [{}, "exactly one"],
    [{ key: "a", text: "b" }, "exactly one"],
    [{ key: "hyper+x" }, "Unknown modifier: hyper"],
    [{ mod: "super" }, "Unknown modifier: super"],
    [{ key: "foo" }, "Unknown key: foo"],
    [{ key: "a", icon: "nope" }, "Unknown icon: nope"],
  ])("%j はエラー", (config, message) => {
    expect(() => parseKey(config)).toThrow(message);
  });
});
