// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { setLang, t } from "./i18n";

describe("t", () => {
  it("言語を切り替え、{name} を埋める", () => {
    setLang("ja");
    expect(t("uploading", { count: 3 })).toBe("アップロード中…（3件）");
    expect(document.documentElement.lang).toBe("ja");
    setLang("en");
    expect(t("uploading", { count: 3 })).toBe("Uploading… (3)");
    expect(t("closeWindow", { index: 1, name: "vim" })).toBe("Close window 1 (vim)");
    expect(document.documentElement.lang).toBe("en");
  });

  it("渡していない {name} はそのまま残す", () => {
    setLang("en");
    expect(t("uploading")).toBe("Uploading… ({count})");
  });
});
