// @vitest-environment happy-dom
import { describe, expect, it } from "vitest";
import { setLang } from "./i18n";
import { setupTmuxPanel } from "./tmuxpanel";

const noTheme = { names: ["default", "light"], current: () => "default", choose: async (_: string) => true };

function makePanel(version?: string, keybar = { shown: () => true, set: (_: boolean) => {} }, theme = noTheme) {
  const panel = document.createElement("div");
  setupTmuxPanel({
    tabs: document.createElement("div"),
    newWindow: document.createElement("button"),
    button: document.createElement("button"),
    panel,
    sendPrefix: () => {},
    fontSize: { get: () => 14, change: () => {} },
    keybar,
    toast: () => {},
    version,
    openDevices: () => {},
    openFonts: () => {},
    theme,
    bind: (btn, fire) => btn.addEventListener("click", fire),
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

describe("tmux パネルのキーバーの行", () => {
  const keybarButton = (panel: HTMLElement) => panel.querySelector<HTMLButtonElement>(".tmux-keybar");

  it("押すたびにキーバーを隠す・出すを切り替え、文字もそれに合わせる", () => {
    setLang("ja");
    let shown = true;
    const panel = makePanel(undefined, { shown: () => shown, set: (s) => (shown = s) });
    const btn = keybarButton(panel)!;
    expect(btn.parentElement?.querySelector(".tmux-label")?.textContent).toBe("キーバー");
    expect(btn.textContent).toBe("隠す");
    btn.click();
    expect(shown).toBe(false);
    expect(btn.textContent).toBe("出す");
    btn.click();
    expect(shown).toBe(true);
    expect(btn.textContent).toBe("隠す");
  });

  it("隠している間も、画面の言語で出す", () => {
    setLang("en");
    const btn = keybarButton(makePanel(undefined, { shown: () => false, set: () => {} }))!;
    expect(btn.textContent).toBe("Show");
    expect(btn.title).toBe("Show the key bar");
    setLang("ja");
    expect(keybarButton(makePanel(undefined, { shown: () => false, set: () => {} }))!.textContent).toBe("出す");
  });

  it("キーバーの行は版の行より上にある（版は一番下のまま）", () => {
    setLang("en");
    const rows = [...makePanel("v1").querySelectorAll(".tmux-row .tmux-label")].map((l) => l.textContent);
    expect(rows.slice(-2)).toEqual(["Key bar", "Version"]);
  });
});

describe("tmux パネルのテーマの行", () => {
  it("組み込みのテーマを並べ、今のものに印を付ける。押すと選び、書き戻せたら印が移る。断られたら移らない", async () => {
    setLang("ja");
    let current = "default";
    let accept = true;
    const chosen: string[] = [];
    const panel = makePanel(undefined, undefined, {
      names: ["default", "light", "solarized-dark", "solarized-light"],
      current: () => current,
      choose: async (name) => {
        chosen.push(name);
        if (accept) current = name;
        return accept;
      },
    });
    const btns = [...panel.querySelectorAll<HTMLButtonElement>(".tmux-theme")];
    expect(btns[0].parentElement?.querySelector(".tmux-label")?.textContent).toBe("テーマ");
    expect(btns.map((b) => b.textContent)).toEqual(["既定", "明るい", "Solarized 暗", "Solarized 明"]);
    const on = () => btns.filter((b) => b.classList.contains("on")).map((b) => b.dataset.theme);
    expect(on()).toEqual(["default"]);

    btns[1].click();
    await new Promise((r) => setTimeout(r));
    expect(chosen).toEqual(["light"]);
    expect(on()).toEqual(["light"]);

    accept = false;
    btns[2].click();
    await new Promise((r) => setTimeout(r));
    expect(chosen).toEqual(["light", "solarized-dark"]);
    expect(on()).toEqual(["light"]);
  });
});
