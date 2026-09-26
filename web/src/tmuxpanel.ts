// キーバーの右端の tmux ボタンと、押すと出る操作パネル。
// ウィンドウの切り替え・追加、ペインの分割・移動などを1回のタップで行う。
// 操作はサーバーが tmux のコマンドで直接行うので、プレフィックスキーの割り当てに左右されない。

interface TmuxWindow {
  index: number;
  name: string;
  active: boolean;
  panes: number;
  zoomed: boolean;
}

export interface TmuxPanelOptions {
  button: HTMLButtonElement;
  panel: HTMLElement;
  /** tmux のプレフィックス（Ctrl+B）をそのまま送る。パネルにない操作用。 */
  sendPrefix: () => void;
  toast: (message: string) => void;
  /** キーバーと同じ押し方（フォーカスを奪わない、横スクロールでは押さない）でボタンをつなぐ。 */
  bind: (btn: HTMLButtonElement, fire: () => void, repeat: boolean) => void;
}

// 押したあともパネルを開いたままにする操作（続けて何回か押すもの）。
const KEEP_OPEN = new Set(["pane-left", "pane-right", "pane-up", "pane-down", "pane-next"]);

export function setupTmuxPanel(opts: TmuxPanelOptions) {
  const { button, panel } = opts;
  const tabs = document.createElement("div");
  tabs.className = "tmux-tabs";
  let windows: TmuxWindow[] = [];
  let killArmed: number | undefined;

  const row = (label: string, ...children: HTMLElement[]) => {
    const r = document.createElement("div");
    r.className = "tmux-row";
    const l = document.createElement("span");
    l.className = "tmux-label";
    l.textContent = label;
    r.append(l, ...children);
    return r;
  };

  const actionBtn = (label: string, title: string, fire: () => void, cls = "") => {
    const b = document.createElement("button");
    b.type = "button";
    b.textContent = label;
    b.title = title;
    b.setAttribute("aria-label", title);
    if (cls) b.className = cls;
    opts.bind(b, fire, false);
    return b;
  };
  const act = (label: string, title: string, action: string, cls = "") =>
    actionBtn(label, title, () => run(action), cls);

  const newWindow = act("＋", "新しいウィンドウ", "new-window", "tmux-new");
  const zoomBtn = act("⛶ 拡大", "ペインを拡大 / 戻す", "zoom");
  const killBtn = actionBtn("✕ 閉じる", "ペインを閉じる", () => {
    // 間違えて押しても消えないよう、2回目で閉じる。
    if (killArmed === undefined) {
      killBtn.textContent = "もう一度で閉じる";
      killBtn.classList.add("armed");
      killArmed = window.setTimeout(disarmKill, 3000);
      return;
    }
    disarmKill();
    run("kill-pane");
  });
  const disarmKill = () => {
    window.clearTimeout(killArmed);
    killArmed = undefined;
    killBtn.textContent = "✕ 閉じる";
    killBtn.classList.remove("armed");
  };

  const windowRow = row("ウィンドウ", tabs);
  panel.append(
    windowRow,
    row(
      "分割",
      act("◫ 左右", "左右に分割", "split-h"),
      act("⊟ 上下", "上下に分割", "split-v"),
      zoomBtn,
      killBtn,
    ),
    row(
      "ペイン",
      act("←", "左のペインへ", "pane-left"),
      act("↓", "下のペインへ", "pane-down"),
      act("↑", "上のペインへ", "pane-up"),
      act("→", "右のペインへ", "pane-right"),
      act("⟳ 次", "次のペインへ", "pane-next"),
    ),
    row(
      "その他",
      act("コピーモード", "tmux のコピーモード", "copy-mode"),
      actionBtn("Ctrl+B", "tmux のプレフィックスを送る（続けてキーを押す）", () => {
        close();
        opts.sendPrefix();
      }),
    ),
  );

  function renderTabs() {
    const btns = windows.map((w) =>
      actionBtn(
        `${w.index}:${w.name}${w.panes > 1 ? ` ⊞${w.panes}` : ""}`,
        `ウィンドウ ${w.index}（${w.name}）へ`,
        () => (w.active ? close() : run("select-window", w.index)),
        w.active ? "tmux-tab active" : "tmux-tab",
      ),
    );
    tabs.replaceChildren(...btns, newWindow);
    tabs.querySelector(".active")?.scrollIntoView({ inline: "nearest", block: "nearest" });
    const active = windows.find((w) => w.active);
    zoomBtn.classList.toggle("on", !!active?.zoomed);
  }

  async function refresh() {
    try {
      const res = await fetch("/api/windows", { cache: "no-store" });
      if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`);
      windows = (await res.json()).windows ?? [];
      renderTabs();
    } catch (e) {
      opts.toast(`tmux のウィンドウを取れませんでした（${e instanceof Error ? e.message : e}）`);
    }
  }

  async function run(action: string, window?: number) {
    if (!KEEP_OPEN.has(action)) close();
    try {
      const res = await fetch("/api/tmux", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action, window }),
      });
      if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`);
      windows = (await res.json()).windows ?? [];
      renderTabs();
    } catch (e) {
      opts.toast(`tmux の操作ができませんでした（${e instanceof Error ? e.message : e}）`);
    }
  }

  function open() {
    panel.hidden = false;
    button.setAttribute("aria-expanded", "true");
    refresh();
  }

  function close() {
    if (panel.hidden) return;
    disarmKill();
    panel.hidden = true;
    button.setAttribute("aria-expanded", "false");
  }

  opts.bind(button, () => (panel.hidden ? open() : close()), false);

  return { close, refresh };
}
