// テーマの一覧（設定画面の「テーマ」の「一覧」で開く）。組み込みのテーマの後ろに [themes.名前] の自分のテーマを並べ、
// 各行に名前と色見本（テーマの背景の上に文字の色で書いた見本の文字と、端末の16色の四角）を出す。今のテーマの行に印を付ける。
// 行を押すと、すぐ画面に当てて設定ファイルの [theme] の name に書き戻す（書き戻せたら印が移る）。一覧は開いたままにする。
// index.html には置かず、ここで作って body に足す（大きい画面では設定画面が右の列に移す）。

import { hasMessage, type MessageKey, t } from "./i18n";
import type { Theme } from "./theme";

export interface ThemeListOptions {
  /** 並べるテーマの名前（組み込みのものの後ろに自分のテーマ）。開くたびに取り直す。 */
  names: () => string[];
  current: () => string;
  /** 色見本に使う色。 */
  resolve: (name: string) => Theme;
  /** 設定ファイルに書き戻せたら true。 */
  choose: (name: string) => Promise<boolean>;
}

/** テーマの名前の表示（自分のテーマなど、訳の無い名前はそのまま）。 */
export function themeLabel(name: string): string {
  const key = `theme.${name}` as MessageKey;
  return hasMessage(key) ? t(key) : name;
}

/** 見本の文字（端末の画面に出ていそうなもの）。 */
const SAMPLE = "~/src $ ls -la";

function swatch(theme: Theme): HTMLElement {
  const box = document.createElement("span");
  box.className = "theme-swatch";
  box.style.background = theme.term.bg;
  box.style.color = theme.term.fg;
  const sample = document.createElement("span");
  sample.className = "theme-sample";
  sample.textContent = SAMPLE;
  const colors = document.createElement("span");
  colors.className = "theme-colors";
  for (const c of theme.term.colors) {
    const sq = document.createElement("span");
    sq.className = "theme-color";
    sq.style.background = c;
    colors.append(sq);
  }
  box.append(sample, colors);
  return box;
}

export function setupThemes(opts: ThemeListOptions) {
  const el = document.createElement("div");
  el.id = "themes";
  el.hidden = true;
  const bar = document.createElement("div");
  bar.className = "copy-bar";
  const close = document.createElement("button");
  close.type = "button";
  close.id = "themes-close";
  close.textContent = t("copyClose");
  const title = document.createElement("span");
  title.id = "themes-title";
  title.textContent = t("rowTheme");
  bar.append(close, title);
  const list = document.createElement("ul");
  list.id = "theme-list";
  el.append(bar, list);
  document.body.append(el);

  function showCurrent() {
    const current = opts.current();
    for (const b of list.querySelectorAll<HTMLButtonElement>(".theme-item")) {
      const on = b.dataset.theme === current;
      b.classList.toggle("on", on);
      if (on) b.setAttribute("aria-current", "true");
      else b.removeAttribute("aria-current");
    }
  }

  function render() {
    list.replaceChildren(
      ...opts.names().map((name) => {
        const li = document.createElement("li");
        const b = document.createElement("button");
        b.type = "button";
        b.className = "theme-item";
        b.dataset.theme = name;
        b.title = t("themeTitle", { name: themeLabel(name) });
        const label = document.createElement("span");
        label.className = "theme-name";
        label.textContent = themeLabel(name);
        b.append(label, swatch(opts.resolve(name)));
        b.addEventListener("click", async () => {
          if (await opts.choose(name)) showCurrent();
        });
        li.append(b);
        return li;
      }),
    );
    showCurrent();
  }

  close.addEventListener("click", () => (el.hidden = true));

  return {
    el,
    open() {
      render();
      el.hidden = false;
    },
  };
}
