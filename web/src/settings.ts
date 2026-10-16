// 設定画面（tmux パネルの「設定」で開く）。文字サイズ・フォント・テーマ・キーバー・ログイン中の端末・版を並べる。
// index.html には置かず、ここで作って body に足す。フォントと端末の一覧はこの画面の上に開き、閉じるとこの画面に戻る。

import { hasMessage, type MessageKey, t } from "./i18n";
import { icon, type IconName } from "./icons";

export interface SettingsOptions {
  /** 端末の文字サイズ。 */
  fontSize: { get: () => number; change: (delta: number) => void };
  /** キーバーを出しているか・出し分ける（このブラウザに覚える）。 */
  keybar: { shown: () => boolean; set: (shown: boolean) => void };
  /** 組み込みのテーマ。choose は設定ファイルに書き戻せたら true。 */
  theme: { names: string[]; current: () => string; choose: (name: string) => Promise<boolean> };
  /** ログイン中の端末の一覧を開く。 */
  openDevices: () => void;
  /** 画面から足したフォントの一覧を開く（そこでフォントを選ぶ）。 */
  openFonts: () => void;
  /** サーバーの版（`palmterm -version` と同じ）。分からなければ空で、その行を出さない。 */
  version?: string;
}

/** テーマの名前の表示（知らない名前はそのまま）。 */
function themeLabel(name: string): string {
  const key = `theme.${name}` as MessageKey;
  return hasMessage(key) ? t(key) : name;
}

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
  el.append(bar, body);

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

  body.append(
    row(
      t("rowText"),
      sizeBtn({ icon: "textSmaller" }, t("textSmaller"), -1),
      sizeLabel,
      sizeBtn({ icon: "textLarger" }, t("textLarger"), 1),
    ),
    row(t("fonts"), makeBtn(t("fontsOpen"), t("fontsTitle"), () => opts.openFonts(), "settings-fonts")),
    row(t("rowTheme"), ...themeBtns),
    row(t("rowKeybar"), keybarBtn),
    row(t("devices"), makeBtn(t("devicesOpen"), t("devicesTitle"), () => opts.openDevices(), "settings-devices")),
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
    showKeybarState();
  }
  refresh();

  function open() {
    refresh();
    el.hidden = false;
  }
  function close() {
    el.hidden = true;
  }
  closeBtn.addEventListener("click", close);

  return { el, open, close, isOpen: () => !el.hidden };
}
