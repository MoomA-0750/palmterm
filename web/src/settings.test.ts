// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from "vitest";
import { setLang } from "./i18n";
import { type SettingsOptions, setupSettings } from "./settings";

const noTheme = { names: ["default", "light"], current: () => "default", choose: async (_: string) => true };

function makeSettings(over: Partial<SettingsOptions> = {}) {
  const s = setupSettings({
    fontSize: { get: () => 14, change: () => {} },
    keybar: { shown: () => true, set: () => {} },
    theme: noTheme,
    language: { current: () => "ja", choose: () => {} },
    openDevices: () => {},
    openFonts: () => {},
    ...over,
  });
  return { ...s, labels: () => [...s.el.querySelectorAll(".settings-label")].map((l) => l.textContent) };
}

/** happy-dom の画面の大きさを変える（matchMedia もこれに合わせて答える）。 */
function setViewport(width: number, height: number) {
  (window as unknown as { happyDOM: { setViewport: (v: { width: number; height: number }) => void } }).happyDOM.setViewport({ width, height });
}

/** index.html やフォントの画面が足す一覧の代わり。 */
function list(id: string): HTMLElement {
  const l = document.createElement("div");
  l.id = id;
  l.hidden = true;
  document.body.append(l);
  return l;
}

const rowOf = (el: Element | null | undefined) => el?.closest(".settings-row");

beforeEach(() => document.body.replaceChildren());

describe("設定画面の並び", () => {
  it("文字サイズ・フォント・テーマ・言語・キーバー・ログイン中の端末・版が並ぶ", () => {
    setLang("ja");
    expect(makeSettings({ version: "v1" }).labels()).toEqual(["文字", "フォント", "テーマ", "言語", "キーバー", "ログイン中の端末", "版"]);
  });

  it("開くと出て、閉じるボタンで消える", () => {
    setLang("ja");
    const s = makeSettings();
    expect(s.el.hidden).toBe(true);
    s.open();
    expect(s.el.hidden).toBe(false);
    s.el.querySelector<HTMLButtonElement>("#settings-close")!.click();
    expect(s.el.hidden).toBe(true);
  });

  it("小さい画面では、フォントと端末の一覧を開いても、設定画面は閉じない（一覧を閉じると戻る）", () => {
    setLang("en");
    setViewport(400, 760);
    const opened: string[] = [];
    const s = makeSettings({ openFonts: () => opened.push("fonts"), openDevices: () => opened.push("devices") });
    s.open();
    s.el.querySelector<HTMLButtonElement>(".settings-fonts")!.click();
    s.el.querySelector<HTMLButtonElement>(".settings-devices")!.click();
    expect(opened).toEqual(["fonts", "devices"]);
    expect(s.el.hidden).toBe(false);
    expect(s.el.classList.contains("two-columns")).toBe(false);
  });

  it("大きい画面では2列にし、右に端末の一覧を出す。左で押した方の一覧が右に出て、閉じると一緒に隠れる", () => {
    setLang("ja");
    setViewport(1024, 768);
    const devices = list("devices");
    const fonts = list("fonts");
    const s = makeSettings({ openDevices: () => (devices.hidden = false), openFonts: () => (fonts.hidden = false) });
    const side = s.el.querySelector("#settings-side")!;
    const on = () => [...s.el.querySelectorAll("#settings-body button.on")].map((b) => b.className);
    s.open();
    expect(s.el.classList.contains("two-columns")).toBe(true);
    expect([devices.parentElement, fonts.parentElement]).toEqual([side, side]);
    expect([devices.hidden, fonts.hidden]).toEqual([false, true]);
    s.el.querySelector<HTMLButtonElement>(".settings-fonts")!.click();
    expect([devices.hidden, fonts.hidden]).toEqual([true, false]);
    expect(on()).toContain("settings-fonts on");
    s.close();
    expect([devices.hidden, fonts.hidden]).toEqual([true, true]);
    // 開き直すと右はまた端末の一覧
    s.open();
    expect([devices.hidden, fonts.hidden]).toEqual([false, true]);
    // 小さい画面で開くと1列に戻り、一覧は body に戻って隠れる
    s.close();
    setViewport(400, 760);
    s.open();
    expect(s.el.classList.contains("two-columns")).toBe(false);
    expect([devices.parentElement, fonts.parentElement]).toEqual([document.body, document.body]);
    expect([devices.hidden, fonts.hidden]).toEqual([true, true]);
  });
});

describe("設定画面の文字サイズ", () => {
  it("押すと変わり、今の大きさを出す。開くたびに今の大きさを取り直す", () => {
    setLang("ja");
    let size = 14;
    const s = makeSettings({ fontSize: { get: () => size, change: (d) => (size += d) } });
    s.open();
    const label = s.el.querySelector(".settings-size")!;
    expect(label.textContent).toBe("14px");
    s.el.querySelector<HTMLButtonElement>('button[title="文字を大きく"]')?.click();
    expect(size).toBe(15);
    expect(label.textContent).toBe("15px");
    size = 20; // ピンチで変わった
    s.open();
    expect(label.textContent).toBe("20px");
  });
});

describe("設定画面の版の行", () => {
  it("サーバーの版を palmterm -version と同じ文字で出す", () => {
    setLang("ja");
    const row = rowOf(makeSettings({ version: "v1.2.3-4-gabcdef0-dirty" }).el.querySelector(".settings-version"));
    expect(row?.querySelector(".settings-label")?.textContent).toBe("版");
    expect(row?.querySelector(".settings-version")?.textContent).toBe("palmterm v1.2.3-4-gabcdef0-dirty");
  });

  it("版が分からなければ行を出さない", () => {
    setLang("en");
    expect(makeSettings().el.querySelector(".settings-version")).toBeNull();
    expect(makeSettings({ version: "" }).el.querySelector(".settings-version")).toBeNull();
  });
});

describe("設定画面のキーバーの行", () => {
  const keybarButton = (el: HTMLElement) => el.querySelector<HTMLButtonElement>(".settings-keybar");

  it("押すたびにキーバーを隠す・出すを切り替え、文字もそれに合わせる。画面は閉じない", () => {
    setLang("ja");
    let shown = true;
    const s = makeSettings({ keybar: { shown: () => shown, set: (v) => (shown = v) } });
    s.open();
    const btn = keybarButton(s.el)!;
    expect(rowOf(btn)?.querySelector(".settings-label")?.textContent).toBe("キーバー");
    expect(btn.textContent).toBe("隠す");
    btn.click();
    expect(shown).toBe(false);
    expect(btn.textContent).toBe("出す");
    btn.click();
    expect(shown).toBe(true);
    expect(btn.textContent).toBe("隠す");
    expect(s.el.hidden).toBe(false);
  });

  it("隠している間も、画面の言語で出す", () => {
    setLang("en");
    const btn = keybarButton(makeSettings({ keybar: { shown: () => false, set: () => {} } }).el)!;
    expect(btn.textContent).toBe("Show");
    expect(btn.title).toBe("Show the key bar");
    setLang("ja");
    expect(keybarButton(makeSettings({ keybar: { shown: () => false, set: () => {} } }).el)!.textContent).toBe("出す");
  });
});

describe("設定画面のテーマの行", () => {
  it("組み込みのテーマを並べ、今のものに印を付ける。押すと選び、書き戻せたら印が移る。断られたら移らない", async () => {
    setLang("ja");
    let current = "default";
    let accept = true;
    const chosen: string[] = [];
    const s = makeSettings({
      theme: {
        names: ["default", "light", "solarized-dark", "solarized-light"],
        current: () => current,
        choose: async (name) => {
          chosen.push(name);
          if (accept) current = name;
          return accept;
        },
      },
    });
    const btns = [...s.el.querySelectorAll<HTMLButtonElement>(".settings-theme")];
    expect(rowOf(btns[0])?.querySelector(".settings-label")?.textContent).toBe("テーマ");
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

describe("設定画面の言語の行", () => {
  it("English と日本語をそれぞれの言語の名前で並べ、今のものに印を付ける。今と違うものを押すと選ぶ", () => {
    setLang("ja");
    const chosen: string[] = [];
    const s = makeSettings({ language: { current: () => "ja", choose: (lang) => chosen.push(lang) } });
    const btns = [...s.el.querySelectorAll<HTMLButtonElement>(".settings-language")];
    expect(rowOf(btns[0])?.querySelector(".settings-label")?.textContent).toBe("言語");
    expect(btns.map((b) => b.textContent)).toEqual(["English", "日本語"]);
    expect(btns.filter((b) => b.classList.contains("on")).map((b) => b.dataset.lang)).toEqual(["ja"]);

    btns[1].click(); // 今の言語は選び直さない
    btns[0].click();
    expect(chosen).toEqual(["en"]);
  });
});
