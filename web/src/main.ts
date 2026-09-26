import { WTerm } from "@wterm/dom";
import "@wterm/dom/css";
import "./fonts.css";
import "./style.css";
import { Attachments, uploadFile } from "./attachments";
import { Connection } from "./connection";
import { applyToChar, isSingleChar, Modifiers, type ModName, specialKey, type SpecialKey } from "./keys";
import { setupNerdIcons } from "./nerd";
import { setupTmuxPanel } from "./tmuxpanel";
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

// 画面の端（ナビゲーションバーの裏）までは描かない（viewport-fit=cover にしない）。Android の Chromium 系では、
// そうするとキーボードを出したときの高さがバーの分だけ大きく報告され、下の段がキーボードに潜る。
// ブラウザが入力欄を見せるために見えている範囲をずらしたときも、その位置（offsetTop）に合わせる。
function fitViewport() {
  const vv = window.visualViewport;
  const h = vv?.height ?? window.innerHeight;
  const root = document.documentElement.style;
  root.setProperty("--app-h", `${h}px`);
  window.scrollTo(0, 0);
  root.setProperty("--app-top", `${Math.max(0, vv?.offsetTop ?? 0)}px`);
  showViewportDebug();
}

// ?debug=viewport で開くと、画面の寸法を右上に出す（キーボードまわりのずれを調べる用）。
const viewportDebug = new URLSearchParams(location.search).get("debug") === "viewport";
function showViewportDebug() {
  if (!viewportDebug) return;
  let el = document.getElementById("vp-debug");
  if (!el) {
    el = document.createElement("pre");
    el.id = "vp-debug";
    el.style.cssText =
      "position:fixed;top:0;right:0;z-index:100;margin:0;padding:4px;font:11px/1.3 monospace;background:#000c;color:#0f0;pointer-events:none";
    document.body.appendChild(el);
  }
  const vv = window.visualViewport;
  const barEl = document.getElementById("inputbar")!;
  const bar = barEl.getBoundingClientRect();
  const inset = getComputedStyle(barEl).paddingBottom; // 4px + safe-area-inset-bottom
  el.textContent = [
    `inner ${window.innerWidth}x${window.innerHeight}`,
    `vv ${vv?.width.toFixed(1)}x${vv?.height.toFixed(1)} top=${vv?.offsetTop.toFixed(1)}`,
    `screen ${screen.width}x${screen.height} dpr=${devicePixelRatio}`,
    `inputbar bottom=${bar.bottom.toFixed(1)} pad=${inset}`,
  ].join("\n");
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

conn.onOutput = (data) => {
  term.write(data);
  tmuxPanel.outputSeen(); // キーで作ったウィンドウや名前の変化をタブに映す
};
conn.onOpen = () => {
  // つなぎ直すと tmux が画面を描き直すので、前の状態を一度消しておく。
  term.write("\x1bc");
  tmuxPanel.refresh();
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
  // テキストボックスを出しているときは押された見た目にする。
  modeBtn.setAttribute("aria-pressed", String(next === "line"));
  const label = next === "line" ? "テキストボックスを閉じる（端末に直接入力）" : "テキストボックスを出す";
  modeBtn.setAttribute("aria-label", label);
  modeBtn.title = label;
  if (!focus) return;
  if (next === "line") line.focus();
  else term.focus();
}

// ---- 下の入力欄 ----

// 1行のときはボタンと同じ高さ、改行したら 200px まで伸ばす。
const LINE_MIN_HEIGHT = 36;
function autosizeLine() {
  line.style.height = "";
  if (line.value === "") return; // 空なら CSS の高さ（プレースホルダーの折り返しで伸ばさない）
  const border = line.offsetHeight - line.clientHeight; // box-sizing: border-box なので枠の分を足す
  line.style.height = `${Math.min(Math.max(line.scrollHeight + border, LINE_MIN_HEIGHT), 200)}px`;
}

/**
 * 入力欄の文字を送る。withEnter が false なら Enter を付けない（Tab 補完の前など）。
 * Enter を付けるときは、添付欄の画像を先に送る：アップロードが終わるのを待ち、1枚でも失敗して
 * いたら何も送らずに知らせる。画像のパスは1枚ずつ貼り付けとして送る（Claude Code は画像のパス
 * だけの貼り付けを添付として扱う）。添付の処理は少し遅れて進むので、1枚ごとに待つ。
 * 送れたら true。
 */
async function flushLine(withEnter: boolean): Promise<boolean> {
  if (withEnter && attachments.count > 0) {
    if (sending) return false;
    sending = true;
    try {
      if (attachments.uploading) toast("画像のアップロードを待っています…");
      const result = await attachments.waitAll();
      if (!result.ok) {
        toast(`アップロードできなかった画像が${result.failed}枚あるので、送っていません。× で外すか選び直してください（${result.error}）`);
        return false;
      }
      for (const path of result.paths) {
        conn.send(bracketedPaste(path));
        await sleep(PASTE_SETTLE_MS);
      }
      attachments.clear();
    } finally {
      sending = false;
    }
  }
  const text = line.value;
  line.value = "";
  autosizeLine();
  if (text) conn.send(text.includes("\n") ? bracketedPaste(text) : text);
  if (withEnter) conn.send("\r");
  return true;
}

let sending = false;
const PASTE_SETTLE_MS = 400;
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));


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
  | { label: string; text: string };

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
];

const modButtons = new Map<ModName, HTMLButtonElement>();

function pressKey(def: KeyDef) {
  if ("mod" in def) {
    mods.tap(def.mod);
    return;
  }
  if ("key" in def) {
    // Tab 補完は、入力欄に書きかけの文字を先に送ってから。
    if (def.key === "tab" && mode === "line" && !mods.active()) {
      const seq = specialKey(def.key, mods, appCursor());
      flushLine(false).then(() => conn.send(seq));
      return;
    }
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

const tmuxPanel = setupTmuxPanel({
  tabs: $("tabs"),
  newWindow: $("new-window"),
  button: $("tmux-btn"),
  panel: $("tmuxpanel"),
  sendPrefix: () => conn.send("\x02"),
  fontSize: { get: () => fontSize, change: changeFontSize },
  toast,
  bind: bindKeyButton,
});
// 端末に触れたらパネルを閉じる。
termEl.addEventListener("pointerdown", () => tmuxPanel.close());
onTap(termEl, (x, y) => {
  const cell = cellAt(x, y);
  if (cell) tmuxPanel.selectPaneAt(cell.col, cell.row);
});

/** 動かさずに短く触れて離したとき（スワイプ・長押しの選択・ピンチは除く）。 */
function onTap(el: HTMLElement, fire: (x: number, y: number) => void) {
  const TAP_MS = 350;
  const TAP_PX = 10;
  let start: { id: number; x: number; y: number; t: number } | null = null;
  el.addEventListener("pointerdown", (e) => {
    start = e.isPrimary && e.button === 0 ? { id: e.pointerId, x: e.clientX, y: e.clientY, t: performance.now() } : null;
  });
  el.addEventListener("pointermove", (e) => {
    if (start && e.pointerId === start.id && Math.hypot(e.clientX - start.x, e.clientY - start.y) > TAP_PX) start = null;
  });
  el.addEventListener("pointercancel", () => (start = null));
  // 2本目の指が来たらピンチなので取りやめる。
  el.addEventListener("touchstart", (e) => e.touches.length > 1 && (start = null), { passive: true });
  el.addEventListener("pointerup", (e) => {
    const s = start;
    start = null;
    if (!s || e.pointerId !== s.id || performance.now() - s.t > TAP_MS) return;
    if (!document.getSelection()?.isCollapsed) return; // 文字を選んでいる
    fire(e.clientX, e.clientY);
  });
}

/** 画面の位置を、端末のセル（0 から）に直す。 */
function cellAt(x: number, y: number): { col: number; row: number } | null {
  const firstRow = termEl.querySelector(".term-row:not(.term-scrollback-row)");
  if (!firstRow) return null;
  const rect = firstRow.getBoundingClientRect();
  // 文字の幅は wterm が測った値を使う（公開されていないので、なければ行の幅から出す）。
  const charWidth = (term as unknown as { _charWidth?: number })._charWidth || rect.width / term.cols;
  const rowHeight = parseFloat(getComputedStyle(termEl).getPropertyValue("--term-row-height")) || rect.height;
  const col = Math.floor((x - rect.left) / charWidth);
  const row = Math.floor((y - rect.top) / rowHeight);
  if (col < 0 || row < 0 || col >= term.cols || row >= term.rows) return null;
  return { col, row };
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

/**
 * クリップボードの文字を読む。読めなかったら理由を知らせて null を返す。
 * 許可を求める画面が出た最初の1回は、許可しても失敗することがあるので、許可済みならもう一度読む。
 */
async function readClipboard(): Promise<string | null> {
  if (!window.isSecureContext || !navigator.clipboard?.readText) {
    toast("この接続では貼付ボタンを使えません（HTTPS が必要）。入力欄を長押しして貼り付けてください");
    return null;
  }
  let error: unknown;
  try {
    return await navigator.clipboard.readText();
  } catch (e) {
    error = e;
  }
  const state = await clipboardPermission();
  if (state === "granted") {
    try {
      return await navigator.clipboard.readText();
    } catch (e) {
      error = e;
    }
  }
  if (state === "denied") {
    toast("クリップボードの読み取りが許可されていません。ブラウザのサイトの設定で許可してください");
  } else if (error instanceof DOMException && error.name === "NotAllowedError") {
    toast("クリップボードを読めませんでした（許可の確認が済んでいないか、取り消されました）。もう一度押してください");
  } else {
    const detail = error instanceof Error ? `${error.name}: ${error.message}` : String(error);
    toast(`クリップボードを読めませんでした（${detail}）`);
  }
  return null;
}

async function clipboardPermission(): Promise<PermissionState | null> {
  try {
    return (await navigator.permissions.query({ name: "clipboard-read" as PermissionName })).state;
  } catch {
    return null; // Safari などは clipboard-read を問い合わせられない
  }
}

$("paste").addEventListener("click", async () => {
  const text = await readClipboard();
  if (text === null) return;
  if (!text) {
    toast("クリップボードに文字がありません");
    return;
  }
  if (mode === "line") {
    line.setRangeText(text, line.selectionStart, line.selectionEnd, "end");
    autosizeLine();
    line.focus();
  } else {
    conn.send(bracketedPaste(text));
  }
});

// ---- 画像のアップロード ----
// 入力欄モードでは添付欄に並べて、送信のときに渡す（プレビューで確かめてから送れる）。
// 直接入力モードでは、アップロードしたらすぐパスを貼り付ける。
// どちらも Claude Code などに画像を渡すため。HEIC などはサーバーが JPEG に変換してから返す。

const uploadInput = $<HTMLInputElement>("upload-input");
const attachments = new Attachments($("attachments"));

async function addImages(files: File[]) {
  if (files.length === 0) return;
  if (mode === "line") {
    attachments.add(files);
    return;
  }
  toast(`アップロード中…（${files.length}件）`);
  for (const file of files) {
    try {
      const { path } = await uploadFile(file);
      conn.send(bracketedPaste(path));
      await sleep(PASTE_SETTLE_MS);
    } catch (e) {
      toast(`${file.name} をアップロードできませんでした: ${e instanceof Error ? e.message : e}`);
      return;
    }
  }
  toast(`アップロードしました（${files.length}件）`);
}

$("upload").addEventListener("click", () => uploadInput.click());
uploadInput.addEventListener("change", () => {
  const files = [...(uploadInput.files ?? [])];
  uploadInput.value = "";
  addImages(files);
});

// 画像の貼り付け（入力欄・端末）も同じように扱う。
function pastedFiles(e: ClipboardEvent): File[] {
  return [...(e.clipboardData?.files ?? [])];
}
line.addEventListener("paste", (e) => {
  const files = pastedFiles(e);
  if (files.length === 0) return;
  e.preventDefault();
  addImages(files);
});
termEl.addEventListener(
  "paste",
  (e) => {
    const files = pastedFiles(e);
    if (files.length === 0) return;
    e.preventDefault();
    e.stopPropagation();
    addImages(files);
  },
  true,
);

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
setupTouch({
  el: termEl,
  term,
  send: (data) => conn.send(data),
  scrollPane: (lines) => conn.scroll(lines),
  appCursor,
  getFontSize: () => fontSize,
  setFontSize: (size, save) => {
    fontSize = size;
    applyFontSize(size);
    if (save) saveSetting("fontSize", String(size));
  },
});
conn.connect();
