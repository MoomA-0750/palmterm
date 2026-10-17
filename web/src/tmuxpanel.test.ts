// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { setLang } from "./i18n";
import { setupTmuxPanel } from "./tmuxpanel";

function makePanel(openSettings = () => {}) {
  const panel = document.createElement("div");
  panel.hidden = true;
  const button = document.createElement("button");
  setupTmuxPanel({
    tabs: document.createElement("div"),
    newWindow: document.createElement("button"),
    button,
    panel,
    sendPrefix: () => {},
    toast: () => {},
    openSettings,
    bind: (btn, fire) => btn.addEventListener("click", fire),
  });
  return { panel, button };
}

describe("tmux パネルの行", () => {
  it("残るのは分割・ペイン・コピーモード・Ctrl+B・設定だけ", () => {
    setLang("ja");
    const { panel } = makePanel();
    const rows = [...panel.querySelectorAll(".tmux-row")].map((r) => r.querySelector(".tmux-label")?.textContent);
    expect(rows).toEqual(["分割", "ペイン", "その他"]);
    const other = panel.querySelectorAll(".tmux-row")[2];
    expect([...other.querySelectorAll("button")].map((b) => b.textContent)).toEqual(["コピーモード", "Ctrl+B", "設定"]);
    // 文字サイズ・フォント・テーマ・キーバー・ログイン中の端末・版は設定画面に移った
    expect(panel.textContent).not.toContain("テーマ");
    expect(panel.textContent).not.toContain("キーバー");
    expect(panel.textContent).not.toContain("ログイン中の端末");
    expect(panel.textContent).not.toContain("フォント");
  });

  it("「設定」を押すとパネルを閉じて設定画面を開く", () => {
    setLang("en");
    let opened = 0;
    const { panel, button } = makePanel(() => opened++);
    button.click();
    expect(panel.hidden).toBe(false);
    panel.querySelector<HTMLButtonElement>('button[title^="Settings"]')!.click();
    expect(opened).toBe(1);
    expect(panel.hidden).toBe(true);
  });
});
