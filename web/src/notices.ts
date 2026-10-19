// palmterm notify の知らせの帯。端末の上に新しい順に積み、× か帯を押すまで残す（コピーの帯のように消えない）。
// tmux の中で打った知らせには打ったウィンドウの番号と名前を添え、帯を押すとそのウィンドウに切り替える。

import { t } from "./i18n";
import { icon } from "./icons";

export interface NotifyMessage {
  text?: string;
  window?: number;
  windowName?: string;
}

/** 一度に出しておく帯の数（これより古いものは消す）。 */
const MAX_NOTICES = 5;

export function setupNotices(container: HTMLElement, selectWindow: (index: number) => void) {
  function show(msg: NotifyMessage) {
    const bar = document.createElement("div");
    bar.className = "notifybar";
    bar.setAttribute("role", "alert");
    const body = document.createElement("button");
    body.type = "button";
    body.className = "notifybar-body";
    const hasWindow = typeof msg.window === "number";
    if (hasWindow) {
      const win = document.createElement("span");
      win.className = "notifybar-window";
      win.textContent = msg.windowName ? `${msg.window}:${msg.windowName}` : String(msg.window);
      body.append(win);
      body.title = t("notifyGoTitle", { window: win.textContent });
    }
    const text = document.createElement("span");
    text.className = "notifybar-text";
    text.textContent = msg.text || t("notifyDone");
    body.append(text);
    body.addEventListener("click", () => {
      bar.remove();
      if (hasWindow) selectWindow(msg.window!);
    });
    const close = document.createElement("button");
    close.type = "button";
    close.className = "notifybar-close";
    close.setAttribute("aria-label", t("notifyDismiss"));
    close.title = t("notifyDismiss");
    close.append(icon("close", 18));
    close.addEventListener("click", () => bar.remove());
    bar.append(body, close);
    container.prepend(bar);
    while (container.childElementCount > MAX_NOTICES) container.lastElementChild!.remove();
  }
  return { show };
}
