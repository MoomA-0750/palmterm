import { WTerm } from "@wterm/dom";
import "@wterm/dom/css";
import "./fonts.css";
import "./style.css";
import { Attachments, uploadFile } from "./attachments";
import { type ClipItem, clipPreview, deleteClip, fetchClips, renderClips } from "./clips";
import { Connection } from "./connection";
import { type Lang, setLang, t } from "./i18n";
import { icon } from "./icons";
import { DEFAULT_KEYS, type KeyButton, type KeyConfig, parseKey } from "./keyconfig";
import { applyToChar, isSingleChar, keySequenceWithMods, Modifiers, type ModName, specialKey, withMods } from "./keys";
import { setupNerdIcons } from "./nerd";
import { pasteSequence } from "./paste";
import { SoftKeyboard } from "./softkeyboard";
import { setupTmuxPanel } from "./tmuxpanel";
import { setupTouch } from "./touch";

const $ = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;

const termEl = $<HTMLDivElement>("term");
const keybar = $<HTMLDivElement>("keybar");
const line = $<HTMLTextAreaElement>("line");
const sendBtn = $<HTMLButtonElement>("send");
const statusEl = $<HTMLDivElement>("status");
const toastEl = $<HTMLDivElement>("toast");

// ---- 設定ファイル（サーバーの ~/.config/palmterm/config.toml：言語とキーバー） ----

interface ServerConfig {
  language: Lang;
  keys?: KeyConfig[];
  path: string;
  error?: string;
}
const serverConfig: ServerConfig = (await fetch("/api/config", { cache: "no-store" })
  .then((r) => (r.ok ? r.json() : null))
  .catch(() => null)) ?? { language: "en", path: "" };
setLang(serverConfig.language);
applyStaticTexts();

/** index.html に書いてある文言を、設定の言語に置き換える。 */
function applyStaticTexts() {
  const label = (id: string, text: string, title = true) => {
    const el = $(id);
    el.setAttribute("aria-label", text);
    if (title) el.title = text;
  };
  label("term", t("term"), false);
  label("tmuxpanel", t("tmuxButton"), false);
  label("tabs", t("tabs"), false);
  label("new-window", t("newWindow"));
  label("keybar", t("keybar"), false);
  label("tmux-btn", t("tmuxButton"));
  $("tmux-btn").title = t("tmuxButtonTitle");
  label("attachments", t("attachments"), false);
  // Mac や iPad（iPadOS は MacIntel と名乗る）のキーボードでは ⌘+Return と案内する
  const sendKey = /Mac|iPhone|iPad|iPod/.test(navigator.platform) ? "⌘+Return" : "Ctrl+Enter";
  $<HTMLTextAreaElement>("line").placeholder = t("linePlaceholder", { sendKey });
  label("send", t("send", { sendKey }));
  label("copy", t("copy"));
  label("upload", t("upload"));
  $("linkbar-open").textContent = t("linkOpen");
  $("linkbar-close").textContent = t("linkDismiss");
  $("copy-close").textContent = t("copyClose");
  $("copy-refresh").textContent = t("copyRefresh");
  $("copy-to-line").textContent = t("copyToLine");
  $("copy-selection").textContent = t("copySelection");
  $("tab-history").textContent = t("tabHistory");
  $("tab-clips").textContent = t("tabClips");
  $("clipbar-copy").textContent = t("clipNoticeCopy");
  $("clipbar-close").textContent = t("clipNoticeDismiss");
}

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

const MIN_FONT = 8;
const MAX_FONT = 32;
let fontSize = 13;

/** 文字サイズを変える。save が false なら保存しない（ピンチの途中など）。 */
function setFontSize(size: number, save = true) {
  fontSize = Math.min(MAX_FONT, Math.max(MIN_FONT, size));
  termEl.style.setProperty("--term-font-size", `${fontSize}px`);
  termEl.style.setProperty("--term-row-height", `${Math.round(fontSize * LINE_HEIGHT)}px`);
  if (save) saveSetting("fontSize", String(fontSize));
}
setFontSize(Number(loadSetting("fontSize", "13")) || 13, false);

// ---- 画面の高さをソフトキーボードに合わせる ----

// 画面の端（ナビゲーションバーの裏）までは描かない（viewport-fit=cover にしない）。Android の Chromium 系では、
// そうするとキーボードを出したときの高さがバーの分だけ大きく報告され、下の段がキーボードに潜る。
// ブラウザが入力欄を見せるために見えている範囲をずらしたときも、その位置（offsetTop）に合わせる。
const softKeyboard = new SoftKeyboard();

/** ソフトキーボードが出ていそうか。 */
function keyboardVisible(): boolean {
  const vv = window.visualViewport;
  return softKeyboard.visible(vv?.width ?? window.innerWidth, vv?.height ?? window.innerHeight);
}

function fitViewport() {
  keyboardVisible(); // キーボードが出ていないときの高さを覚えておく
  const vv = window.visualViewport;
  const h = vv?.height ?? window.innerHeight;
  const root = document.documentElement.style;
  root.setProperty("--app-h", `${h}px`);
  window.scrollTo(0, 0);
  root.setProperty("--app-top", `${Math.max(0, vv?.offsetTop ?? 0)}px`);
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
// 修飾キーが待っていれば、キーの入力（1文字か、物理キーボードの矢印などの特殊キー）にだけ効かせる。
term.onData = (data) => {
  if (mods.active()) {
    const modified = isSingleChar(data) ? applyToChar(data, mods) : keySequenceWithMods(data, mods, appCursor());
    if (modified !== null) {
      data = modified;
      mods.consume();
    }
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
// tmux の中のプログラムが開こうとした URL（palmterm-open 経由）。ブラウザは押されていないのに新しいタブを
// 開くことを許さないので、知らせを出して、押したら開く。
const linkbar = $<HTMLDivElement>("linkbar");
const copyMode = $<HTMLDivElement>("copymode");
/** コピーの画面で最後に開いていたタブ（ボタンで開き直したときもそのタブにする）。最初はクリップボード。 */
let copyTab: "history" | "clips" = "clips";
let pendingLink = "";
conn.onNotice = (msg) => {
  if (msg.type === "clipboard") {
    clipboardChanged();
    return;
  }
  if (msg.type !== "open" || !msg.url || !/^https?:\/\//.test(msg.url)) return;
  pendingLink = msg.url;
  $("linkbar-text").textContent = `${t("linkRequest")} ${msg.url}`;
  linkbar.hidden = false;
};
$("linkbar-open").addEventListener("click", () => {
  window.open(pendingLink, "_blank", "noopener");
  linkbar.hidden = true;
});
$("linkbar-close").addEventListener("click", () => (linkbar.hidden = true));

// tmux でコピーされた（tmux がバッファにコピーして OSC 52 を送ってきた）ときの知らせ。押すとスマホの
// クリップボードに入れる（ブラウザは押されていないのにクリップボードへ書くことを許さないため）。
const clipbar = $<HTMLDivElement>("clipbar");
const CLIPBAR_MS = 15_000;
let pendingClip: ClipItem | null = null;
let clipbarTimer: number | undefined;
let clipsSeen = "";

async function clipboardChanged() {
  let items: ClipItem[];
  try {
    items = await fetchClips();
  } catch {
    return; // 一覧を開けば読み直せる
  }
  if (!copyMode.hidden && copyTab === "clips") showClips(items);
  const latest = items[0];
  // 同じ文字をもう一度コピーしたときも知らせる（一覧ではまとめているので、バッファの名前で見分ける）
  const key = latest ? latest.names.join(",") : "";
  if (!latest || key === clipsSeen) return;
  clipsSeen = key;
  pendingClip = latest;
  $("clipbar-text").textContent = `${t("clipNotice")} ${clipPreview(latest.text)}`;
  clipbar.hidden = false;
  window.clearTimeout(clipbarTimer);
  clipbarTimer = window.setTimeout(() => (clipbar.hidden = true), CLIPBAR_MS);
}
$("clipbar-copy").addEventListener("click", async () => {
  clipbar.hidden = true;
  if (pendingClip) toast((await copyText(pendingClip.text)) ? t("copied") : t("copyFailed"));
});
$("clipbar-close").addEventListener("click", () => (clipbar.hidden = true));

conn.onStatus = (status) => {
  statusEl.hidden = status === "open";
  statusEl.textContent = status === "connecting" ? t("connecting") : t("disconnected");
};

function appCursor(): boolean {
  return term.bridge?.cursorKeysApp() ?? false;
}

function bracketedPaste(text: string): string {
  return pasteSequence(text, term.bridge?.bracketedPaste() ?? false);
}

// ---- 入力 ----
// 下のテキストボックス（常に出す）：編集してから送る（OS のカーソル移動・IME・予測変換が使える）。
//   Enter は改行で、送信ボタンか Ctrl+Enter で送る（Claude Code への複数行の指示など）
// 端末をタップすると、端末に1文字ずつすぐ送る（vim や TUI 向け）。

/**
 * 今テキストボックスに入力しているか。最後に文字を入れる所としてフォーカスしたのがどちらかで決める
 * （下の段のボタンを押すとフォーカスがボタンに移るので、今のフォーカスでは決められない）。
 */
let lastInput: "line" | "term" = "line";
line.addEventListener("focus", () => (lastInput = "line"));
termEl.addEventListener("focusin", () => (lastInput = "term"));
function usingLine(): boolean {
  return lastInput === "line";
}

/** テキストボックスのカーソルの所に文字を入れる（貼り付け・コピーモードから）。 */
function insertIntoLine(text: string) {
  line.setRangeText(text, line.selectionStart, line.selectionEnd, "end");
  autosizeLine();
  line.focus();
}

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
let sending = false;
// Claude Code は画像のパスの貼り付けを少し遅れて添付に変えるので、1枚ごとにこれだけ待つ。
const PASTE_SETTLE_MS = 400;
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

async function flushLine(withEnter: boolean): Promise<boolean> {
  // つながっていないと送った文字は届かないので、消さずに知らせる（つなぎ直したら送り直せる）。
  if (!conn.isOpen()) {
    toast(t("notConnected"));
    return false;
  }
  if (withEnter && attachments.count > 0) {
    if (sending) return false;
    sending = true;
    try {
      if (attachments.uploading) toast(t("waitingUpload"));
      const result = await attachments.waitAll();
      if (!result.ok) {
        toast(t("uploadFailedNotSent", { count: result.failed, error: result.error }));
        return false;
      }
      for (const entry of result.entries) {
        if (!entry.present()) continue; // 待っている間に × で外された
        if (!conn.isOpen()) {
          toast(t("notConnected")); // 残りの画像と文章はそのまま残す
          return false;
        }
        conn.send(bracketedPaste(entry.path));
        entry.remove();
        await sleep(PASTE_SETTLE_MS);
      }
    } finally {
      sending = false;
    }
    // 画像のあとの待ちの間に切れていたら、文章は消さずに残す。
    if (!conn.isOpen()) {
      toast(t("notConnected"));
      return false;
    }
  }
  const text = line.value;
  line.value = "";
  autosizeLine();
  if (text) conn.send(text.includes("\n") ? bracketedPaste(text) : text);
  if (withEnter) conn.send("\r");
  return true;
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

// ---- キーバー ----

// 並びは設定ファイルの [[keys]]（書いていなければ既定の並び）。書き方がおかしいキーは飛ばして知らせる。
const keyErrors: string[] = [];
const keyButtons: KeyButton[] = [];
(serverConfig.keys?.length ? serverConfig.keys : DEFAULT_KEYS).forEach((k, i) => {
  try {
    keyButtons.push(parseKey(k));
  } catch (e) {
    keyErrors.push(t("keyConfigError", { index: i + 1, error: e instanceof Error ? e.message : String(e) }));
  }
});

const modButtons: [ModName, HTMLButtonElement][] = [];

function pressKey(def: KeyButton) {
  const a = def.action;
  switch (a.type) {
    case "mod":
      mods.tap(a.mod);
      return;
    case "text":
      conn.send(isSingleChar(a.text) ? applyToChar(a.text, mods) : a.text);
      break;
    case "char":
      conn.send(applyToChar(a.char, withMods(mods, a.mods)));
      break;
    case "key": {
      const seq = specialKey(a.key, withMods(mods, a.mods), appCursor());
      // Tab 補完は、入力欄に書きかけの文字を先に送ってから。
      if (a.key === "tab" && a.mods.length === 0 && usingLine() && !mods.active()) {
        flushLine(false).then(() => conn.send(seq));
        return;
      }
      conn.send(seq);
      break;
    }
  }
  mods.consume();
}

for (const def of keyButtons) {
  const btn = document.createElement("button");
  btn.type = "button";
  if (def.icon) {
    btn.append(icon(def.icon));
    btn.setAttribute("aria-label", def.label);
    btn.title = def.label;
  } else {
    btn.textContent = def.label;
  }
  if (def.action.type === "mod") {
    btn.classList.add("mod");
    modButtons.push([def.action.mod, btn]);
  }
  bindKeyButton(btn, () => pressKey(def), def.repeat);
  keybar.appendChild(btn);
}

const tmuxPanel = setupTmuxPanel({
  tabs: $("tabs"),
  newWindow: $("new-window"),
  button: $("tmux-btn"),
  panel: $("tmuxpanel"),
  sendPrefix: () => conn.send("\x02"),
  fontSize: { get: () => fontSize, change: (delta) => setFontSize(fontSize + delta) },
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
 * キーバーのボタン。押してもフォーカスを奪わない（出ているソフトキーボードを閉じない）。
 * ただし、キーボードが出ていないときに指で押したら、入力欄のフォーカスを外す。入力欄にフォーカスが
 * 残ったままタップすると、ブラウザがキーボードを出してしまうため。
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
    if (e.pointerType === "touch" && !keyboardVisible()) (document.activeElement as HTMLElement | null)?.blur();
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

// ---- 知らせとコピー ----

function toast(message: string, ms = 2500) {
  toastEl.textContent = message;
  toastEl.hidden = false;
  window.clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => (toastEl.hidden = true), ms);
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

// ---- 画像のアップロード ----
// テキストボックスに入力しているときは添付欄に並べて、送信のときに渡す（プレビューで確かめてから送れる）。
// 端末に直接入力しているときは、アップロードしたらすぐパスを貼り付ける。
// どちらも Claude Code などに画像を渡すため。HEIC などはサーバーが JPEG に変換してから返す。

const uploadInput = $<HTMLInputElement>("upload-input");
const attachments = new Attachments($("attachments"));

async function addImages(files: File[]) {
  if (files.length === 0) return;
  if (usingLine()) {
    attachments.add(files);
    return;
  }
  toast(t("uploading", { count: files.length }));
  for (const file of files) {
    try {
      const { path } = await uploadFile(file);
      conn.send(bracketedPaste(path));
      await sleep(PASTE_SETTLE_MS);
    } catch (e) {
      toast(t("uploadFailed", { name: file.name, error: e instanceof Error ? e.message : String(e) }));
      return;
    }
  }
  toast(t("uploaded", { count: files.length }));
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
// 「クリップボード」のタブでは、palmterm のクリップボード（tmux のバッファ）を一覧にする。

const copyTextEl = $<HTMLPreElement>("copy-text");
const clipsEl = $<HTMLUListElement>("clips");

async function loadHistory(): Promise<string> {
  try {
    const res = await fetch("/api/history?lines=3000", { cache: "no-store" });
    if (res.ok) return await res.text();
  } catch {
    // 下で画面の内容に切り替える
  }
  toast(t("historyFallback"));
  return term.readText();
}

async function openCopyMode(tab: "history" | "clips" = copyTab) {
  copyMode.hidden = false;
  copyTab = tab;
  $("tab-history").setAttribute("aria-selected", String(tab === "history"));
  $("tab-clips").setAttribute("aria-selected", String(tab === "clips"));
  $("history-bar").hidden = tab !== "history";
  copyTextEl.hidden = tab !== "history";
  clipsEl.hidden = tab !== "clips";
  if (tab === "clips") {
    await loadClips();
    return;
  }
  copyTextEl.textContent = t("loading");
  const text = await loadHistory();
  copyTextEl.textContent = text;
  copyTextEl.scrollTop = copyTextEl.scrollHeight;
}

async function loadClips() {
  if (clipsEl.childElementCount === 0) clipsEl.textContent = t("loading");
  try {
    showClips(await fetchClips());
  } catch (e) {
    clipsEl.textContent = t("clipsError", { error: e instanceof Error ? e.message : String(e) });
  }
}

function showClips(items: ClipItem[]) {
  renderClips(clipsEl, items, {
    copy: async (item) => toast((await copyText(item.text)) ? t("copied") : t("copyFailed")),
    toLine: (item) => {
      closeCopyMode();
      insertIntoLine(item.text);
    },
    remove: async (item) => {
      try {
        showClips(await deleteClip(item));
      } catch (e) {
        toast(t("clipsError", { error: e instanceof Error ? e.message : String(e) }));
      }
    },
  });
}

function closeCopyMode() {
  copyMode.hidden = true;
  copyTextEl.textContent = "";
  window.getSelection()?.removeAllRanges();
}

/** コピーモードで選んだ文字。選んでいなければ知らせて空を返す。 */
function selectedCopyText(): string {
  const sel = window.getSelection();
  if (!sel || sel.isCollapsed || !copyTextEl.contains(sel.anchorNode)) {
    toast(t("selectFirst"));
    return "";
  }
  return sel.toString();
}

$("copy").addEventListener("click", () => openCopyMode());
$("copy-close").addEventListener("click", closeCopyMode);
$("copy-refresh").addEventListener("click", () => openCopyMode());
$("tab-history").addEventListener("click", () => openCopyMode("history"));
$("tab-clips").addEventListener("click", () => openCopyMode("clips"));
$("copy-selection").addEventListener("click", async () => {
  const text = selectedCopyText();
  if (!text) return;
  toast((await copyText(text)) ? t("copied") : t("copyFailed"));
});
$("copy-to-line").addEventListener("click", () => {
  const text = selectedCopyText();
  if (!text) return;
  closeCopyMode();
  insertIntoLine(text);
});

// ---- 起動 ----

await term.init();

// 設定ファイルの書き間違いは、読める所だけ使って知らせる（長めに出す）。
const configProblems = [
  ...(serverConfig.error ? [t("configError", { path: serverConfig.path, error: serverConfig.error })] : []),
  ...keyErrors,
];
if (configProblems.length) toast(configProblems.join("\n"), 10000);

setupNerdIcons(termEl);
setupTouch({
  el: termEl,
  term,
  send: (data) => conn.send(data),
  scrollPane: (lines) => conn.scroll(lines),
  appCursor,
  getFontSize: () => fontSize,
  setFontSize,
});
conn.connect();
