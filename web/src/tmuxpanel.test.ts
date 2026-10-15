// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { setLang } from "./i18n";
import { setupTmuxPanel } from "./tmuxpanel";

function makePanel(version?: string) {
  const panel = document.createElement("div");
  setupTmuxPanel({
    tabs: document.createElement("div"),
    newWindow: document.createElement("button"),
    button: document.createElement("button"),
    panel,
    sendPrefix: () => {},
    fontSize: { get: () => 14, change: () => {} },
    toast: () => {},
    version,
    openDevices: () => {},
    bind: () => {},
  });
  return panel;
}

describe("tmux パネルの版の行", () => {
  it("サーバーの版を palmterm -version と同じ文字で出す", () => {
    setLang("ja");
    const row = makePanel("v1.2.3-4-gabcdef0-dirty").querySelector(".tmux-version")?.parentElement;
    expect(row?.querySelector(".tmux-label")?.textContent).toBe("版");
    expect(row?.querySelector(".tmux-version")?.textContent).toBe("palmterm v1.2.3-4-gabcdef0-dirty");
  });

  it("版が分からなければ行を出さない", () => {
    setLang("en");
    expect(makePanel(undefined).querySelector(".tmux-version")).toBeNull();
    expect(makePanel("").querySelector(".tmux-version")).toBeNull();
  });
});
