// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from "vitest";
import { setLang } from "./i18n";
import type { PushState, PushStatus } from "./push";
import { type SettingsOptions, setupSettings } from "./settings";

function makeSettings(over: Partial<SettingsOptions> = {}) {
  const s = setupSettings({
    fontSize: { get: () => 14, change: () => {} },
    keybar: { shown: () => true, set: () => {} },
    language: { current: () => "ja", choose: () => {} },
    openDevices: () => {},
    openFonts: () => {},
    openThemes: () => {},
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

/** 通知の代わり。enable・disable で状態が変わり、呼ばれた順を覚える。 */
function notifications(initial: PushState, onEnable: PushStatus = { state: "on" }) {
  let current: PushStatus = { state: initial };
  const calls: string[] = [];
  return {
    calls,
    state: async () => current,
    enable: async () => (calls.push("enable"), (current = onEnable)),
    disable: async () => (calls.push("disable"), (current = { state: "off" })),
  };
}

const flush = () => new Promise((r) => setTimeout(r, 0));

beforeEach(() => document.body.replaceChildren());

describe("設定画面の並び", () => {
  it("文字サイズ・フォント・テーマ・言語・キーバー・通知・ログイン中の端末・版が並ぶ", () => {
    setLang("ja");
    expect(makeSettings({ version: "v1", notifications: notifications("off") }).labels()).toEqual(["文字", "フォント", "テーマ", "言語", "キーバー", "通知", "ログイン中の端末", "版"]);
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

  it("小さい画面では、フォント・テーマ・端末の一覧を開いても、設定画面は閉じない（一覧を閉じると戻る）", () => {
    setLang("en");
    setViewport(400, 760);
    const opened: string[] = [];
    const s = makeSettings({
      openFonts: () => opened.push("fonts"),
      openThemes: () => opened.push("themes"),
      openDevices: () => opened.push("devices"),
    });
    s.open();
    s.el.querySelector<HTMLButtonElement>(".settings-fonts")!.click();
    s.el.querySelector<HTMLButtonElement>(".settings-themes")!.click();
    s.el.querySelector<HTMLButtonElement>(".settings-devices")!.click();
    expect(opened).toEqual(["fonts", "themes", "devices"]);
    expect(s.el.hidden).toBe(false);
    expect(s.el.classList.contains("two-columns")).toBe(false);
  });

  it("大きい画面では2列にし、右に端末の一覧を出す。左で押した方の一覧が右に出て、閉じると一緒に隠れる", () => {
    setLang("ja");
    setViewport(1024, 768);
    const devices = list("devices");
    const fonts = list("fonts");
    const themes = list("themes");
    const s = makeSettings({
      openDevices: () => (devices.hidden = false),
      openFonts: () => (fonts.hidden = false),
      openThemes: () => (themes.hidden = false),
    });
    const side = s.el.querySelector("#settings-side")!;
    const on = () => [...s.el.querySelectorAll("#settings-body button.on:not(.settings-language)")].map((b) => b.className);
    const shown = () => [devices.hidden, fonts.hidden, themes.hidden];
    s.open();
    expect(s.el.classList.contains("two-columns")).toBe(true);
    expect([devices.parentElement, fonts.parentElement, themes.parentElement]).toEqual([side, side, side]);
    expect(shown()).toEqual([false, true, true]);
    s.el.querySelector<HTMLButtonElement>(".settings-fonts")!.click();
    expect(shown()).toEqual([true, false, true]);
    expect(on()).toEqual(["settings-fonts on"]);
    s.el.querySelector<HTMLButtonElement>(".settings-themes")!.click();
    expect(shown()).toEqual([true, true, false]);
    expect(on()).toEqual(["settings-themes on"]);
    s.close();
    expect(shown()).toEqual([true, true, true]);
    // 開き直すと右はまた端末の一覧
    s.open();
    expect(shown()).toEqual([false, true, true]);
    // 小さい画面で開くと1列に戻り、一覧は body に戻って隠れる
    s.close();
    setViewport(400, 760);
    s.open();
    expect(s.el.classList.contains("two-columns")).toBe(false);
    expect([devices.parentElement, fonts.parentElement, themes.parentElement]).toEqual([document.body, document.body, document.body]);
    expect(shown()).toEqual([true, true, true]);
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
  const keybarSwitch = (el: HTMLElement) => el.querySelector<HTMLButtonElement>(".settings-keybar");

  it("文字のボタンではなくトグルスイッチ1つ。出ていればオン、押すと隠れて切、もう一度押すと出てオン。画面は閉じない", () => {
    setLang("ja");
    let shown = true;
    const s = makeSettings({ keybar: { shown: () => shown, set: (v) => (shown = v) } });
    s.open();
    const sw = keybarSwitch(s.el)!;
    expect([...rowOf(sw)!.querySelectorAll("button")]).toEqual([sw]);
    expect(rowOf(sw)?.querySelector(".settings-label")?.textContent).toBe("キーバー");
    expect(sw.getAttribute("role")).toBe("switch");
    expect(sw.textContent).toBe("");
    expect(sw.getAttribute("aria-checked")).toBe("true");
    sw.click();
    expect(shown).toBe(false);
    expect(sw.getAttribute("aria-checked")).toBe("false");
    sw.click();
    expect(shown).toBe(true);
    expect(sw.getAttribute("aria-checked")).toBe("true");
    expect(s.el.hidden).toBe(false);
  });

  it("隠している間は切の位置。名前は画面の言語で出す", () => {
    setLang("en");
    const sw = keybarSwitch(makeSettings({ keybar: { shown: () => false, set: () => {} } }).el)!;
    expect(sw.getAttribute("aria-checked")).toBe("false");
    expect(sw.getAttribute("aria-label")).toBe("Show the key bar");
    setLang("ja");
    expect(keybarSwitch(makeSettings().el)!.getAttribute("aria-label")).toBe("キーバーを出す");
  });
});

describe("設定画面のテーマの行", () => {
  it("横並びのボタンの代わりに「一覧」があり、押すとテーマの一覧を開く", () => {
    setLang("ja");
    let opened = 0;
    const s = makeSettings({ openThemes: () => opened++ });
    const btn = s.el.querySelector<HTMLButtonElement>(".settings-themes")!;
    expect(rowOf(btn)?.querySelector(".settings-label")?.textContent).toBe("テーマ");
    expect([...rowOf(btn)!.querySelectorAll("button")].map((b) => b.textContent)).toEqual(["一覧"]);
    btn.click();
    expect(opened).toBe(1);
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

describe("設定画面の通知の行", () => {
  const notifyRow = (s: { el: HTMLElement }) => ({
    text: () => s.el.querySelector<HTMLElement>(".settings-notify-state")!,
    sw: () => s.el.querySelector<HTMLButtonElement>(".settings-notify")!,
    checked: () => s.el.querySelector(".settings-notify")!.getAttribute("aria-checked"),
  });

  it("文字のボタンではなくトグルスイッチ1つ。切なら押すと許可を求めてオンの位置に、オンなら押すと切の位置に戻る", async () => {
    setLang("ja");
    const n = notifications("off");
    const s = makeSettings({ notifications: n });
    s.open();
    await flush();
    const r = notifyRow(s);
    expect([...rowOf(r.sw())!.querySelectorAll("button")]).toEqual([r.sw()]);
    expect(r.sw().getAttribute("role")).toBe("switch");
    expect(r.sw().textContent).toBe("");
    expect(r.sw().disabled).toBe(false);
    expect(r.checked()).toBe("false");
    expect(r.text().hidden).toBe(true);
    r.sw().click();
    await flush();
    expect(n.calls).toEqual(["enable"]);
    expect(r.checked()).toBe("true");
    expect(r.text().hidden).toBe(true);
    r.sw().click();
    await flush();
    expect(n.calls).toEqual(["enable", "disable"]);
    expect(r.checked()).toBe("false");
  });

  it("許可を待つ間・登録する間は押せず、二重に呼ばない。済めば押せるようになる", async () => {
    setLang("ja");
    let finish: (v: PushStatus) => void = () => {};
    let enables = 0;
    const s = makeSettings({
      notifications: {
        state: async () => ({ state: "off" }),
        enable: () => (enables++, new Promise<PushStatus>((r) => (finish = r))),
        disable: async () => ({ state: "off" }),
      },
    });
    s.open();
    await flush();
    const r = notifyRow(s);
    r.sw().click();
    expect(r.sw().disabled).toBe(true);
    expect(r.checked()).toBe("false");
    r.sw().click();
    await flush();
    expect(enables).toBe(1);
    finish({ state: "on" });
    await flush();
    expect(r.sw().disabled).toBe(false);
    expect(r.checked()).toBe("true");
  });

  it("今の様子が分かるまでは押せない", async () => {
    setLang("ja");
    const s = makeSettings({ notifications: { state: () => new Promise<PushStatus>(() => {}), enable: async () => ({ state: "on" }), disable: async () => ({ state: "off" }) } });
    s.open();
    await flush();
    expect(notifyRow(s).sw().disabled).toBe(true);
  });

  it("使えないときはスイッチを押せない形で切の位置に出し、理由を添える（ホーム画面から開いていない・HTTPS でない・未対応・許可されていない）", async () => {
    setLang("ja");
    for (const [state, want] of [
      ["needsHomeScreen", "ホーム画面に追加した palmterm から開いたときだけ"],
      ["denied", "通知が許可されていません"],
      ["insecure", "HTTPS"],
      ["unsupported", "対応していません"],
    ] as const) {
      document.body.replaceChildren();
      const s = makeSettings({ notifications: notifications(state) });
      s.open();
      await flush();
      const r = notifyRow(s);
      expect(r.sw().hidden).toBe(false);
      expect(r.sw().disabled).toBe(true);
      expect(r.checked()).toBe("false");
      expect(r.text().hidden).toBe(false);
      expect(r.text().textContent).toContain(want);
    }
  });

  it("許可を断られたら切の位置で押せなくなり理由を出す。失敗したら切の位置に戻り、理由を出して押し直せる", async () => {
    setLang("en");
    const s = makeSettings({ notifications: notifications("off", { state: "denied" }) });
    s.open();
    await flush();
    notifyRow(s).sw().click();
    await flush();
    expect(notifyRow(s).checked()).toBe("false");
    expect(notifyRow(s).sw().disabled).toBe(true);
    expect(notifyRow(s).text().textContent).toContain("blocked");

    document.body.replaceChildren();
    const f = makeSettings({ notifications: notifications("off", { state: "error", error: "boom" }) });
    f.open();
    await flush();
    notifyRow(f).sw().click();
    await flush();
    expect(notifyRow(f).text().textContent).toBe("Could not change notifications (boom)");
    expect(notifyRow(f).checked()).toBe("false");
    expect(notifyRow(f).sw().disabled).toBe(false);
  });

  it("許可を求めても決めなかった（閉じた）ら切の位置に戻る", async () => {
    setLang("ja");
    const s = makeSettings({ notifications: notifications("off", { state: "off" }) });
    s.open();
    await flush();
    notifyRow(s).sw().click();
    await flush();
    expect(notifyRow(s).checked()).toBe("false");
    expect(notifyRow(s).sw().disabled).toBe(false);
  });
});
