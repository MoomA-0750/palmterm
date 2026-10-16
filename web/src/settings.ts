// 設定画面（tmux パネルの「設定」で開く）。文字サイズ・フォント・テーマ・言語・キーバー・ログイン中の端末・版を並べる。
// index.html には置かず、ここで作って body に足す。小さい画面では、フォントと端末の一覧はこの画面の上に開き、
// 閉じるとこの画面に戻る。板を浮かせる大きい画面では2列にし、左に設定の行、右にどちらかの一覧を出す
// （一覧の要素を右の列に移す。小さい画面に戻ると body に戻して隠す）。

import { hasMessage, type Lang, LANG_NAMES, type MessageKey, t } from "./i18n";
import { icon, type IconName } from "./icons";
import { FLOAT_QUERY } from "./sheets";

export interface SettingsOptions {
  /** 端末の文字サイズ。 */
  fontSize: { get: () => number; change: (delta: number) => void };
  /** キーバーを出しているか・出し分ける（このブラウザに覚える）。 */
  keybar: { shown: () => boolean; set: (shown: boolean) => void };
  /** 組み込みのテーマ。choose は設定ファイルに書き戻せたら true。 */
  theme: { names: string[]; current: () => string; choose: (name: string) => Promise<boolean> };
  /** 画面の言語。choose は設定ファイルに書き戻す（書き戻せたらページを読み込み直す）。 */
  language: { current: () => Lang; choose: (lang: Lang) => void };
  /** ログイン中の端末の一覧（id が devices の要素）を開く。 */
  openDevices: () => void;
  /** 画面から足したフォントの一覧（id が fonts の要素）を開く（そこでフォントを選ぶ）。 */
  openFonts: () => void;
  /** サーバーの版（`palmterm -version` と同じ）。分からなければ空で、その行を出さない。 */
  version?: string;
}

/** テーマの名前の表示（知らない名前はそのまま）。 */
function themeLabel(name: string): string {
  const key = `theme.${name}` as MessageKey;
  return hasMessage(key) ? t(key) : name;
}

/** 2列のときに右に出す一覧。値は一覧の要素の id。 */
type Side = "devices" | "fonts";
const SIDES: Side[] = ["devices", "fonts"];

type Label = string | { icon: IconName; text?: string };

function makeBtn(label: Label, title: string, fire: () => void, cls = ""): HTMLButtonElement {
  const b = document.createElement("button");
  b.type = "button";
  if (typeof label === "string") b.textContent = label;
  else {
    b.append(icon(label.icon, 18));
    if (label.text) {
      const s = document.createElement("span");
      s.textContent = label.text;
      b.append(s);
    }
  }
  b.title = title;
  b.setAttribute("aria-label", title);
  if (cls) b.className = cls;
  b.addEventListener("click", fire);
  return b;
}

function row(label: string, ...children: HTMLElement[]): HTMLElement {
  const r = document.createElement("div");
  r.className = "settings-row";
  const l = document.createElement("span");
  l.className = "settings-label";
  l.textContent = label;
  const body = document.createElement("div");
  body.className = "settings-items";
  body.append(...children);
  r.append(l, body);
  return r;
}

export function setupSettings(opts: SettingsOptions) {
  const el = document.createElement("div");
  el.id = "settings";
  el.hidden = true;
  const bar = document.createElement("div");
  bar.className = "copy-bar";
  const closeBtn = document.createElement("button");
  closeBtn.type = "button";
  closeBtn.id = "settings-close";
  closeBtn.textContent = t("copyClose");
  const title = document.createElement("span");
  title.id = "settings-title";
  title.textContent = t("settings");
  bar.append(closeBtn, title);
  const body = document.createElement("div");
  body.id = "settings-body";
  const main = document.createElement("div");
  main.id = "settings-main";
  main.append(bar, body);
  const sideEl = document.createElement("div");
  sideEl.id = "settings-side";
  el.append(main, sideEl);

  // 文字サイズ。続けて押して合わせる。
  const sizeLabel = document.createElement("span");
  sizeLabel.className = "settings-size";
  const showSize = () => (sizeLabel.textContent = `${opts.fontSize.get()}px`);
  const sizeBtn = (label: Label, title: string, delta: number) =>
    makeBtn(label, title, () => {
      opts.fontSize.change(delta);
      showSize();
    });

  // テーマ。押すとすぐ画面に当てて設定ファイルに書き戻す。
  const themeBtns = opts.theme.names.map((name) => {
    const b = makeBtn(themeLabel(name), t("themeTitle", { name }), async () => {
      if (await opts.theme.choose(name)) showTheme();
    }, "settings-theme");
    b.dataset.theme = name;
    return b;
  });
  const showTheme = () => {
    const current = opts.theme.current();
    for (const b of themeBtns) b.classList.toggle("on", b.dataset.theme === current);
  };

  // 言語。名前はそれぞれの言語で出す。選ぶと設定ファイルに書き戻して読み込み直す。
  const langBtns = (Object.keys(LANG_NAMES) as Lang[]).map((lang) => {
    const b = makeBtn(LANG_NAMES[lang], t("languageTitle", { name: LANG_NAMES[lang] }), () => {
      if (lang !== opts.language.current()) opts.language.choose(lang);
    }, "settings-language");
    b.lang = lang;
    b.dataset.lang = lang;
    return b;
  });
  const showLanguage = () => {
    const current = opts.language.current();
    for (const b of langBtns) b.classList.toggle("on", b.dataset.lang === current);
  };

  // キーバーを隠す・出す。
  const keybarBtn = makeBtn("", "", () => {
    opts.keybar.set(!opts.keybar.shown());
    showKeybarState();
  }, "settings-keybar");
  const showKeybarState = () => {
    const shown = opts.keybar.shown();
    keybarBtn.textContent = t(shown ? "keybarHide" : "keybarShow");
    keybarBtn.title = t(shown ? "keybarHideTitle" : "keybarShowTitle");
    keybarBtn.setAttribute("aria-label", keybarBtn.title);
  };

  // フォントと端末の一覧。小さい画面ではこの画面の上に開き、2列のときは右の列に出す。
  const openList = { devices: () => opts.openDevices(), fonts: () => opts.openFonts() };
  const sideBtns = {
    devices: makeBtn(t("devicesOpen"), t("devicesTitle"), () => showList("devices"), "settings-devices"),
    fonts: makeBtn(t("fontsOpen"), t("fontsTitle"), () => showList("fonts"), "settings-fonts"),
  };
  const twoColumns = () => window.matchMedia?.(FLOAT_QUERY).matches ?? false;
  let side: Side = "devices";

  function showList(s: Side) {
    if (twoColumns()) {
      side = s;
      for (const other of SIDES) {
        const l = document.getElementById(other);
        if (other !== s && l) l.hidden = true;
        sideBtns[other].classList.toggle("on", other === s);
      }
    }
    openList[s]();
  }

  /** 画面の大きさに合わせて、一覧を右の列に移すか body に戻す（戻した一覧は隠す）。 */
  function layout() {
    const two = twoColumns();
    el.classList.toggle("two-columns", two);
    for (const s of SIDES) {
      const l = document.getElementById(s);
      if (two && l && l.parentElement !== sideEl) sideEl.append(l);
      if (!two && l && l.parentElement === sideEl) {
        l.hidden = true;
        document.body.append(l);
      }
      if (!two) sideBtns[s].classList.remove("on");
    }
  }

  body.append(
    row(
      t("rowText"),
      sizeBtn({ icon: "textSmaller" }, t("textSmaller"), -1),
      sizeLabel,
      sizeBtn({ icon: "textLarger" }, t("textLarger"), 1),
    ),
    row(t("fonts"), sideBtns.fonts),
    row(t("rowTheme"), ...themeBtns),
    row(t("rowLanguage"), ...langBtns),
    row(t("rowKeybar"), keybarBtn),
    row(t("devices"), sideBtns.devices),
  );
  if (opts.version) {
    const v = document.createElement("span");
    v.className = "settings-version";
    v.textContent = `palmterm ${opts.version}`;
    body.append(row(t("rowVersion"), v));
  }
  document.body.append(el);

  function refresh() {
    showSize(); // ピンチで変わっていることがある
    showTheme();
    showLanguage();
    showKeybarState();
  }
  refresh();

  function open() {
    refresh();
    el.hidden = false;
    layout();
    if (twoColumns()) showList("devices"); // 開いた直後の右は端末の一覧
  }
  function close() {
    el.hidden = true;
    for (const l of sideEl.children) (l as HTMLElement).hidden = true;
  }
  closeBtn.addEventListener("click", close);

  // 開いている間に画面の大きさが変わったら、その場で1列と2列を切り替える。
  window.matchMedia?.(FLOAT_QUERY).addEventListener?.("change", () => {
    if (el.hidden) return;
    const wasTwo = el.classList.contains("two-columns");
    layout();
    if (twoColumns() && !wasTwo) showList(SIDES.find((s) => !document.getElementById(s)?.hidden) ?? side);
  });

  return { el, open, close, isOpen: () => !el.hidden };
}
