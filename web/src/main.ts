import { WTerm } from "@wterm/dom";
import "@wterm/dom/css";
import "./fonts.css";
import "./style.css";
import { Connection } from "./connection";
import { applyToChar, isSingleChar, Modifiers, type ModName, specialKey, type SpecialKey } from "./keys";
import { setupNerdIcons } from "./nerd";
import { HistoryView } from "./history";
import { setupTouch } from "./touch";

const $ = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;

const termEl = $<HTMLDivElement>("term");
const keybar = $<HTMLDivElement>("keybar");
const line = $<HTMLTextAreaElement>("line");
const modeBtn = $<HTMLButtonElement>("mode");
const sendBtn = $<HTMLButtonElement>("send");
const statusEl = $<HTMLDivElement>("status");
const toastEl = $<HTMLDivElement>("toast");

// ---- 設定（この端末のブラウザにだけ保存） ----

function loadSetting(key: string, def: string): string {
  try {
    return localStorage.getItem(`palmterm.${key}`) ?? def;
  } catch {
    return def;
  }
}
function saveSetting(key: string, value: string) {
  try {
    localStorage.setItem(`palmterm.${key}`, value);
  } catch {
    // 保存できない環境でも動作は続ける
  }
}

// wterm は行の高さを --term-row-height で決め、文字サイズに合わせては変えないので、ここで合わせる。
// 比率はアイコン用フォント（tools/fit-nerd-symbols.py の LINE_HEIGHT）と同じにする。
const LINE_HEIGHT = 1.3;

function applyFontSize(size: number) {
  termEl.style.setProperty("--term-font-size", `${size}px`);
  termEl.style.setProperty("--term-row-height", `${Math.round(size * LINE_HEIGHT)}px`);
}

let fontSize = Number(loadSetting("fontSize", "13")) || 13;
applyFontSize(fontSize);

// ---- 画面の高さをソフトキーボードに合わせる ----

function fitViewport() {
  const h = window.visualViewport?.height ?? window.innerHeight;
  document.documentElement.style.setProperty("--app-h", `${h}px`);
  window.scrollTo(0, 0);
}
window.visualViewport?.addEventListener("resize", fitViewport);
window.visualViewport?.addEventListener("scroll", fitViewport);
window.addEventListener("resize", fitViewport);
fitViewport();

// ---- 端末と接続 ----

const mods = new Modifiers();
const term = new WTerm(termEl, { cursorBlink: false });
const conn = new Connection(() => ({ cols: term.cols, rows: term.rows }));

// 画面から出るデータ（直接入力のキー・マウス操作・端末からの応答）は全部ここを通る。
// 修飾キーが待っていれば、1文字の入力にだけ効かせる。
term.onData = (data) => {
  if (mods.active() && isSingleChar(data)) {
    data = applyToChar(data, mods);
    mods.consume();
  }
  conn.send(data);
};
term.onBinary = (data) => conn.send(data);
term.onResize = (cols, rows) => conn.resize(cols, rows);

conn.onOutput = (data) => term.write(data);
conn.onOpen = () => {
  // つなぎ直すと tmux が画面を描き直すので、前の状態を一度消しておく。
  term.write("\x1bc");
};
conn.onStatus = (status) => {
  statusEl.hidden = status === "open";
  statusEl.textContent = status === "connecting" ? "接続中…" : "切断されました。つなぎ直しています…";
};

function appCursor(): boolean {
  return term.bridge?.cursorKeysApp() ?? false;
}

function bracketedPaste(text: string): string {
  const body = text.replace(/\r?\n/g, "\r");
  return term.bridge?.bracketedPaste() ? `\x1b[200~${body}\x1b[201~` : body;
}

// ---- 入力のモード ----
// line: 下の入力欄で編集してから送る（OS のカーソル移動・IME・予測変換が使える）。
//       Enter は改行で、送信ボタンか Ctrl+Enter で送る（Claude Code への複数行の指示など）
// direct: 1文字ずつすぐ送る（vim や TUI 向け）。既定はこちら。
// 保存の名前は既定を line から direct に変えたときに inputMode へ改めた（前の保存を引き継がないため）。

type InputMode = "line" | "direct";
let mode: InputMode = loadSetting("inputMode", "direct") === "line" ? "line" : "direct";

function setMode(next: InputMode, focus = true) {
  mode = next;
  saveSetting("inputMode", next);
  document.body.dataset.mode = next;
  modeBtn.textContent = next === "line" ? "行" : "直";
  modeBtn.setAttribute("aria-label", next === "line" ? "入力欄モード（タップで直接入力へ）" : "直接入力モード（タップで入力欄へ）");
  if (!focus) return;
  if (next === "line") line.focus();
  else term.focus();
}

// ---- 下の入力欄 ----

function autosizeLine() {
  line.style.height = "auto";
  line.style.height = `${Math.min(line.scrollHeight, 200)}px`;
}

/** 入力欄の文字を送る。withEnter が false なら Enter を付けない（Tab 補完の前など）。 */
function flushLine(withEnter: boolean) {
  const text = line.value;
  line.value = "";
  autosizeLine();
  if (text) conn.send(text.includes("\n") ? bracketedPaste(text) : text);
  if (withEnter) conn.send("\r");
}

line.addEventListener("input", autosizeLine);
line.addEventListener("beforeinput", (e) => {
  // 修飾キーが待っているときの1文字は、入力欄に入れずにすぐ送る（Ctrl → c など）。
  if (mods.active() && e.inputType === "insertText" && e.data && isSingleChar(e.data)) {
    e.preventDefault();
    conn.send(applyToChar(e.data, mods));
    mods.consume();
    return;
  }
  // Enter は改行。キーバーの Ctrl を押してからの Enter は送信（ソフトキーボード用）。
  if ((e.inputType === "insertLineBreak" || e.inputType === "insertParagraph") && mods.has("ctrl")) {
    e.preventDefault();
    mods.consume();
    flushLine(true);
    return;
  }
  // 空のときの Backspace は、端末側の文字を消す。
  if (e.inputType === "deleteContentBackward" && line.value === "") {
    e.preventDefault();
    conn.send("\x7f");
  }
});
line.addEventListener("keydown", (e) => {
  if (e.isComposing) return;
  // Ctrl+Enter（Mac は ⌘+Enter も）で送信。ただの Enter は改行。
  if (e.key === "Enter" && (e.ctrlKey || e.metaKey || mods.has("ctrl"))) {
    e.preventDefault();
    if (mods.has("ctrl")) mods.consume();
    flushLine(true);
  } else if (e.key === "Backspace" && line.value === "") {
    e.preventDefault();
    conn.send("\x7f");
  }
});
sendBtn.addEventListener("click", () => flushLine(true));
modeBtn.addEventListener("click", () => setMode(mode === "line" ? "direct" : "line"));

// ---- キーバー ----

type KeyDef =
  | { label: string; mod: ModName }
  | { label: string; key: SpecialKey; repeat?: boolean }
  | { label: string; text: string }
  | { label: string; action: () => void };

const keyDefs: KeyDef[] = [
  { label: "Esc", key: "esc" },
  { label: "Tab", key: "tab" },
  { label: "Ctrl", mod: "ctrl" },
  { label: "Alt", mod: "alt" },
  { label: "Shift", mod: "shift" },
  { label: "←", key: "left", repeat: true },
  { label: "↓", key: "down", repeat: true },
  { label: "↑", key: "up", repeat: true },
  { label: "→", key: "right", repeat: true },
  { label: "^C", text: "\x03" },
  { label: "^D", text: "\x04" },
  { label: "tmux", text: "\x02" },
  { label: "⌫", key: "backspace", repeat: true },
  { label: "⏎", key: "enter" },
  { label: "Home", key: "home" },
  { label: "End", key: "end" },
  { label: "PgUp", key: "pageup", repeat: true },
  { label: "PgDn", key: "pagedown", repeat: true },
  { label: "|", text: "|" },
  { label: "~", text: "~" },
  { label: "/", text: "/" },
  { label: "-", text: "-" },
  { label: "`", text: "`" },
  { label: "A−", action: () => changeFontSize(-1) },
  { label: "A+", action: () => changeFontSize(1) },
];

const modButtons = new Map<ModName, HTMLButtonElement>();

function pressKey(def: KeyDef) {
  if ("mod" in def) {
    mods.tap(def.mod);
    return;
  }
  if ("action" in def) {
    def.action();
    return;
  }
  if ("key" in def) {
    // Tab 補完は、入力欄に書きかけの文字を先に送ってから。
    if (def.key === "tab" && mode === "line" && !mods.active()) flushLine(false);
    conn.send(specialKey(def.key, mods, appCursor()));
  } else {
    conn.send(isSingleChar(def.text) ? applyToChar(def.text, mods) : def.text);
  }
  mods.consume();
}

for (const def of keyDefs) {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.textContent = def.label;
  if ("mod" in def) {
    btn.classList.add("mod");
    modButtons.set(def.mod, btn);
  }
  bindKeyButton(btn, () => pressKey(def), "repeat" in def && !!def.repeat);
  keybar.appendChild(btn);
}

mods.onChange = () => {
  for (const [name, btn] of modButtons) btn.dataset.state = mods.get(name);
};

/**
 * キーバーのボタン。押してもフォーカスを奪わない（ソフトキーボードを閉じない）。
 * repeat のものは押し続けると繰り返す。横にスクロールしたときは押したことにしない。
 */
function bindKeyButton(btn: HTMLButtonElement, fire: () => void, repeat: boolean) {
  let timer: number | undefined;
  let startX = 0;
  let fired = false;
  let moved = false;
  const stop = () => {
    window.clearTimeout(timer);
    timer = undefined;
  };
  btn.addEventListener("pointerdown", (e) => {
    e.preventDefault();
    startX = e.clientX;
    fired = false;
    moved = false;
    if (!repeat) return;
    const tick = (delay: number) => {
      timer = window.setTimeout(() => {
        fired = true;
        fire();
        tick(60);
      }, delay);
    };
    tick(400);
  });
  btn.addEventListener("pointermove", (e) => {
    if (Math.abs(e.clientX - startX) > 10) {
      moved = true;
      stop();
    }
  });
  btn.addEventListener("pointerup", () => {
    stop();
    if (!fired && !moved) fire();
  });
  btn.addEventListener("pointercancel", () => {
    moved = true;
    stop();
  });
  btn.addEventListener("pointerleave", stop);
  // キーボードやスクリーンリーダーからの操作（pointer を伴わない click）。
  btn.addEventListener("click", (e) => {
    if (e.detail === 0) fire();
  });
}

function changeFontSize(delta: number) {
  fontSize = Math.min(32, Math.max(8, fontSize + delta));
  applyFontSize(fontSize);
  saveSetting("fontSize", String(fontSize));
}

// ---- コピー・貼り付け ----

function toast(message: string) {
  toastEl.textContent = message;
  toastEl.hidden = false;
  window.clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => (toastEl.hidden = true), 2500);
}
let toastTimer: number | undefined;

async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    // HTTPS でない接続では clipboard API が使えないので、古い方法で試す。
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.style.cssText = "position:fixed;top:0;left:0;opacity:0";
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand("copy");
    ta.remove();
    return ok;
  }
}

$("paste").addEventListener("click", async () => {
  let text: string;
  try {
    text = await navigator.clipboard.readText();
  } catch {
    toast("この接続では貼付ボタンを使えません（HTTPS が必要）。入力欄を長押しして貼り付けてください");
    return;
  }
  if (!text) return;
  if (mode === "line") {
    line.setRangeText(text, line.selectionStart, line.selectionEnd, "end");
    autosizeLine();
    line.focus();
  } else {
    conn.send(bracketedPaste(text));
  }
});

// ---- コピーモード：履歴を普通のテキストとして出し、OS の選択でコピーする ----

const copyMode = $<HTMLDivElement>("copymode");
const copyTextEl = $<HTMLPreElement>("copy-text");

async function loadHistory(): Promise<string> {
  try {
    const res = await fetch("/api/history?lines=3000", { cache: "no-store" });
    if (res.ok) return await res.text();
  } catch {
    // 下で画面の内容に切り替える
  }
  toast("tmux の履歴を取れなかったので、今の画面の内容を表示します");
  return term.readText();
}

async function openCopyMode() {
  copyMode.hidden = false;
  copyTextEl.textContent = "読み込み中…";
  const text = await loadHistory();
  copyTextEl.textContent = text;
  copyTextEl.scrollTop = copyTextEl.scrollHeight;
}

function closeCopyMode() {
  copyMode.hidden = true;
  copyTextEl.textContent = "";
  window.getSelection()?.removeAllRanges();
}

function selectedCopyText(): string {
  const sel = window.getSelection();
  if (!sel || sel.isCollapsed || !copyTextEl.contains(sel.anchorNode)) return "";
  return sel.toString();
}

$("copy").addEventListener("click", openCopyMode);
$("copy-close").addEventListener("click", closeCopyMode);
$("copy-refresh").addEventListener("click", openCopyMode);
$("copy-selection").addEventListener("click", async () => {
  const text = selectedCopyText();
  if (!text) {
    toast("先に長押しで範囲を選んでください");
    return;
  }
  toast((await copyText(text)) ? "コピーしました" : "コピーできませんでした");
});
$("copy-to-line").addEventListener("click", () => {
  const text = selectedCopyText();
  if (!text) {
    toast("先に長押しで範囲を選んでください");
    return;
  }
  closeCopyMode();
  setMode("line", false);
  line.setRangeText(text, line.selectionStart, line.selectionEnd, "end");
  autosizeLine();
  line.focus();
});

// ---- 起動 ----

await term.init();
setMode(mode, false);

// 入力欄モードで端末をタップしても、ソフトキーボードは出さない（マウス操作は届く）。
// 入力欄を使っている最中なら、入力欄にフォーカスを戻す。
const termInput = termEl.querySelector("textarea");
termInput?.addEventListener("focus", (e) => {
  if (mode !== "line") return;
  if (e.relatedTarget === line) line.focus();
  else termInput.blur();
});
setupNerdIcons(termEl);
const history = new HistoryView($("term-wrap"), termEl);
// 何か入力したら、履歴表示を閉じて今の画面に戻る。
conn.onInput = () => history.close();
setupTouch({
  el: termEl,
  term,
  history,
  send: (data) => conn.send(data),
  appCursor,
  getFontSize: () => fontSize,
  setFontSize: (size, save) => {
    fontSize = size;
    applyFontSize(size);
    if (save) saveSetting("fontSize", String(size));
  },
});
conn.connect();
