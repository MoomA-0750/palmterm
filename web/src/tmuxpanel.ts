// tmux のウィンドウのタブバー（常に出す）と、キーバーの右端の tmux ボタンで開くペインの操作パネル。
// 操作はサーバーが tmux のコマンドで直接行うので、プレフィックスキーの割り当てに左右されない。

import { t } from "./i18n";
import { icon, type IconName } from "./icons";

interface TmuxWindow {
  index: number;
  name: string;
  active: boolean;
  panes: number;
  zoomed: boolean;
}

export interface TmuxPanelOptions {
  /** ウィンドウのタブを並べる所（横にスクロールする）。 */
  tabs: HTMLElement;
  /** タブバーの右端の ＋。 */
  newWindow: HTMLButtonElement;
  button: HTMLButtonElement;
  panel: HTMLElement;
  /** tmux のプレフィックス（Ctrl+B）をそのまま送る。パネルにない操作用。 */
  sendPrefix: () => void;
  /** 端末の文字サイズ（パネルの「文字」の行で変える）。 */
  fontSize: { get: () => number; change: (delta: number) => void };
  toast: (message: string) => void;
  /** キーバーと同じ押し方（フォーカスを奪わない、横スクロールでは押さない）でボタンをつなぐ。 */
  bind: (btn: HTMLButtonElement, fire: () => void, repeat: boolean) => void;
}

// 押したあともパネルを開いたままにする操作（続けて何回か押すもの）。
const KEEP_OPEN = new Set(["pane-left", "pane-right", "pane-up", "pane-down", "pane-next"]);

// 端末に何か出力があったら、少し待ってからタブを取り直す（キーで作ったウィンドウや名前の変化を映す）。
// 出力が続いている間も、この間隔より頻繁には問い合わせない。
const REFRESH_DELAY_MS = 250;
const REFRESH_MIN_INTERVAL_MS = 1000;

// 閉じるボタンは、間違えて押しても消えないよう、2回目で閉じる。1回目から戻るまでの時間。
const ARM_MS = 3000;

export function setupTmuxPanel(opts: TmuxPanelOptions) {
  const { button, panel, tabs } = opts;
  let windows: TmuxWindow[] = [];
  let lastJson = "";
  let lastActive = -1;

  /** label は文字だけ、またはアイコン（と横に添える文字）。 */
  type Label = string | { icon: IconName; text?: string; size?: number };
  const setLabel = (b: HTMLButtonElement, label: Label) => {
    if (typeof label === "string") {
      b.textContent = label;
      return;
    }
    b.replaceChildren(icon(label.icon, label.size ?? 18));
    if (label.text) {
      const t = document.createElement("span");
      t.textContent = label.text;
      b.append(t);
    }
  };
  const makeBtn = (label: Label, title: string, fire: () => void, cls = "") => {
    const b = document.createElement("button");
    b.type = "button";
    setLabel(b, label);
    b.title = title;
    b.setAttribute("aria-label", title);
    if (cls) b.className = cls;
    opts.bind(b, fire, false);
    return b;
  };

  /** 1回目で「もう一度」の表示にし、時間内にもう一度押されたら fire する。 */
  let disarm: (() => void) | null = null;
  const armOrFire = (btn: HTMLButtonElement, armedLabel: string, fire: () => void) => {
    if (btn.classList.contains("armed")) {
      disarm?.();
      fire();
      return;
    }
    disarm?.();
    const label = [...btn.childNodes];
    const timer = window.setTimeout(() => disarm?.(), ARM_MS);
    btn.textContent = armedLabel;
    btn.classList.add("armed");
    disarm = () => {
      window.clearTimeout(timer);
      btn.replaceChildren(...label);
      btn.classList.remove("armed");
      disarm = null;
    };
  };

  // ---- タブバー ----

  opts.bind(opts.newWindow, () => run("new-window"), false);

  function renderTabs() {
    disarm?.();
    tabs.replaceChildren(
      ...windows.map((w) => {
        const tab = document.createElement("div");
        tab.className = w.active ? "tmux-tab active" : "tmux-tab";
        const name = makeBtn(
          `${w.index}:${w.name}`,
          t(w.active ? "renameWindow" : "gotoWindow", { index: w.index, name: w.name }),
          // 今いるタブをもう一度押したら名前を変える。
          () => (w.active ? startRename(w, name) : run("select-window", w.index)),
          "tmux-tab-name",
        );
        if (w.panes > 1) {
          // ペインが複数あるウィンドウには、分割のアイコンと数を添える。
          const count = document.createElement("span");
          count.className = "tmux-tab-panes";
          count.append(icon("panes", 14), String(w.panes));
          name.append(count);
        }
        const x = makeBtn(
          { icon: "close", size: 14 },
          t("closeWindow", { index: w.index, name: w.name }),
          () => armOrFire(x, t("closeConfirm"), () => run("kill-window", w.index)),
          "tmux-tab-close",
        );
        tab.append(name, x);
        return tab;
      }),
    );
    const active = windows.find((w) => w.active);
    if (active && active.index !== lastActive) {
      tabs.querySelector(".active")?.scrollIntoView({ inline: "nearest", block: "nearest" });
    }
    lastActive = active?.index ?? -1;
    zoomBtn.classList.toggle("on", !!active?.zoomed);
  }

  function setWindows(list: TmuxWindow[]) {
    const json = JSON.stringify(list);
    if (json === lastJson) return; // 変わっていなければ描き直さない（押しかけの × を戻さない）
    lastJson = json;
    windows = list;
    if (!renaming) renderTabs(); // 名前を入力している間は描き直さない（終わったら描く）
  }

  let renaming = false;

  /** タブの名前の所を入力欄にする。Enter か外をタップで決定、Esc でやめる。空にすると自動の名前に戻す。 */
  function startRename(w: TmuxWindow, nameBtn: HTMLButtonElement) {
    disarm?.();
    renaming = true;
    const input = document.createElement("input");
    input.className = "tmux-tab-input";
    input.value = w.name;
    input.setAttribute("aria-label", t("newName", { index: w.index }));
    input.enterKeyHint = "done";
    input.autocapitalize = "off";
    input.autocomplete = "off";
    input.spellcheck = false;
    let finished = false;
    const finish = (commit: boolean) => {
      if (finished) return;
      finished = true;
      renaming = false;
      const name = input.value;
      renderTabs();
      if (commit && name !== w.name) run("rename-window", w.index, { name });
    };
    input.addEventListener("keydown", (e) => {
      if (e.isComposing) return;
      if (e.key === "Enter") {
        e.preventDefault();
        finish(true);
      } else if (e.key === "Escape") {
        e.preventDefault();
        finish(false);
      }
    });
    input.addEventListener("blur", () => finish(true));
    nameBtn.replaceWith(input);
    input.focus();
    input.select();
  }

  let refreshTimer: number | undefined;
  let lastRefresh = 0;
  let refreshing = false;

  async function refresh() {
    if (refreshing) return;
    refreshing = true;
    lastRefresh = performance.now();
    try {
      const res = await fetch("/api/windows", { cache: "no-store" });
      if (res.ok) setWindows((await res.json()).windows ?? []);
    } catch {
      // つながっていないときは前のタブのままにする（つなぎ直したら取り直す）。
    } finally {
      refreshing = false;
    }
  }

  /** 端末に出力があったときに呼ぶ。 */
  function outputSeen() {
    if (refreshTimer !== undefined) return;
    const wait = Math.max(REFRESH_DELAY_MS, lastRefresh + REFRESH_MIN_INTERVAL_MS - performance.now());
    refreshTimer = window.setTimeout(() => {
      refreshTimer = undefined;
      refresh();
    }, wait);
  }

  // ---- ペインの操作パネル ----

  const row = (label: string, ...children: HTMLElement[]) => {
    const r = document.createElement("div");
    r.className = "tmux-row";
    const l = document.createElement("span");
    l.className = "tmux-label";
    l.textContent = label;
    r.append(l, ...children);
    return r;
  };
  const act = (label: Label, title: string, action: string) => makeBtn(label, title, () => run(action));

  const zoomBtn = act({ icon: "maximize", text: t("zoom") }, t("zoomTitle"), "zoom");
  const killBtn = makeBtn({ icon: "close", text: t("close") }, t("closePane"), () =>
    armOrFire(killBtn, t("closePaneConfirm"), () => run("kill-pane")),
  );

  // 文字サイズは押してもパネルを閉じない（続けて押して合わせる）。
  const sizeLabel = document.createElement("span");
  sizeLabel.className = "tmux-size";
  const showSize = () => (sizeLabel.textContent = `${opts.fontSize.get()}px`);
  const sizeBtn = (label: Label, title: string, delta: number) =>
    makeBtn(label, title, () => {
      opts.fontSize.change(delta);
      showSize();
    });

  panel.append(
    row(
      t("rowSplit"),
      act({ icon: "splitH", text: t("splitH") }, t("splitHTitle"), "split-h"),
      act({ icon: "splitV", text: t("splitV") }, t("splitVTitle"), "split-v"),
      zoomBtn,
      killBtn,
    ),
    row(
      t("rowPane"),
      act({ icon: "left" }, t("paneLeft"), "pane-left"),
      act({ icon: "down" }, t("paneDown"), "pane-down"),
      act({ icon: "up" }, t("paneUp"), "pane-up"),
      act({ icon: "right" }, t("paneRight"), "pane-right"),
      act({ icon: "cycle", text: t("paneNext") }, t("paneNextTitle"), "pane-next"),
    ),
    row(
      t("rowOther"),
      act(t("copyMode"), t("copyModeTitle"), "copy-mode"),
      makeBtn("Ctrl+B", t("prefixTitle"), () => {
        close();
        opts.sendPrefix();
      }),
    ),
    row(
      t("rowText"),
      sizeBtn({ icon: "textSmaller" }, t("textSmaller"), -1),
      sizeLabel,
      sizeBtn({ icon: "textLarger" }, t("textLarger"), 1),
    ),
  );

  async function run(action: string, window?: number, extra: Record<string, string | number> = {}) {
    if (!KEEP_OPEN.has(action)) close();
    try {
      const res = await fetch("/api/tmux", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action, window, ...extra }),
      });
      if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`);
      setWindows((await res.json()).windows ?? []);
    } catch (e) {
      opts.toast(t("tmuxFailed", { error: e instanceof Error ? e.message : String(e) }));
    }
  }

  function open() {
    showSize(); // ピンチで変わっていることがある
    panel.hidden = false;
    button.setAttribute("aria-expanded", "true");
  }

  function close() {
    if (panel.hidden) return;
    disarm?.();
    panel.hidden = true;
    button.setAttribute("aria-expanded", "false");
  }

  /** 端末の画面のセル（col, row）にあるペインを選ぶ。ペインが1つだけのウィンドウでは何もしない。 */
  function selectPaneAt(col: number, row: number) {
    const active = windows.find((w) => w.active);
    if (!active || active.panes < 2 || active.zoomed) return;
    run("select-pane-at", undefined, { col, row });
  }

  opts.bind(button, () => (panel.hidden ? open() : close()), false);

  return { close, refresh, outputSeen, selectPaneAt };
}
