// @vitest-environment node
// 実際のブラウザ（chromium）で画面を動かす通しのテスト。先に `make` で ../palmterm を作っておく。
// palmterm は、このテストだけの tmux サーバー（TMUX_TMPDIR を分ける、シェルは /bin/sh）と
// 一時ディレクトリの設定ファイル・アップロード先で動かすので、ふだんの環境には触れない。

import { type ChildProcess, execFileSync, spawn } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { inflateSync } from "node:zlib";
import { type Browser, type CDPSession, chromium, type Page } from "playwright-core";
import { afterAll, beforeAll, beforeEach, describe, expect, it } from "vitest";

const BINARY = resolve(__dirname, "../../palmterm");
const CHROMIUM = process.env.CHROMIUM ?? "/usr/bin/chromium";
const TOKEN = "e2etoken";
const SESSION = "e2e";

const dir = mkdtempSync(join(tmpdir(), "palmterm-e2e-"));
const tmuxDir = mkdtempSync("/tmp/pt-e2e-"); // tmux のソケットのパスは短くないといけない
const configPath = join(dir, "config.toml");
const uploadDir = join(dir, "uploads");
// XDG_CONFIG_HOME も一時ディレクトリにする（LAN 用の証明書をふだんの ~/.config に作らない）。
// XDG_RUNTIME_DIR も分ける（ブラウザの受け渡しのソケットと palmterm-open をそこに作る）。
// XDG_DATA_HOME も分ける（PC に入っているフォントとして、テスト用のフォントを fonts に置く）。
const env = { ...process.env, TMUX_TMPDIR: tmuxDir, TMUX: "", SHELL: "/bin/sh", ENV: "", PS1: "$ ", XDG_CONFIG_HOME: dir, XDG_RUNTIME_DIR: tmuxDir, XDG_DATA_HOME: join(dir, "data") };
// ブラウザ（スマホ）にだけ入っているフォントの置き場所。
const browserData = join(dir, "browser-data");

/**
 * テスト用のフォント：JetBrains Mono の WOFF を TrueType に戻し、名前を family（14文字）に付け替えたもの。
 * 名前の長さが同じなので、名前の所を書き換えるだけで済む。
 */
function testFont(family: string): Buffer {
  if (family.length !== 14) throw new Error("名前は JetBrains Mono と同じ14文字にする");
  const woff = readFileSync(resolve(__dirname, "../node_modules/@fontsource/jetbrains-mono/files/jetbrains-mono-latin-400-normal.woff"));
  const n = woff.readUInt16BE(12);
  const tables = Array.from({ length: n }, (_, i) => {
    const e = 44 + 20 * i;
    const [offset, comp, orig] = [woff.readUInt32BE(e + 4), woff.readUInt32BE(e + 8), woff.readUInt32BE(e + 12)];
    const raw = woff.subarray(offset, offset + comp);
    return { tag: woff.subarray(e, e + 4), checksum: woff.readUInt32BE(e + 16), data: comp < orig ? inflateSync(raw) : raw };
  });
  const head = Buffer.alloc(12 + 16 * n);
  woff.copy(head, 0, 4, 8); // 元の形式（TrueType）
  head.writeUInt16BE(n, 4);
  const parts: Buffer[] = [head];
  let at = head.length;
  tables.forEach((t, i) => {
    t.tag.copy(head, 12 + 16 * i);
    head.writeUInt32BE(t.checksum, 12 + 16 * i + 4);
    head.writeUInt32BE(at, 12 + 16 * i + 8);
    head.writeUInt32BE(t.data.length, 12 + 16 * i + 12);
    const padded = Buffer.alloc((t.data.length + 3) & ~3);
    t.data.copy(padded);
    parts.push(padded);
    at += padded.length;
  });
  const font = Buffer.concat(parts);
  for (const [from, to] of [
    [Buffer.from("JetBrains Mono", "utf16le").swap16(), Buffer.from(family, "utf16le").swap16()],
    [Buffer.from("JetBrains Mono"), Buffer.from(family)],
  ]) {
    for (let i = font.indexOf(from); i >= 0; i = font.indexOf(from, i + 1)) to.copy(font, i);
  }
  return font;
}
mkdirSync(join(dir, "data", "fonts"), { recursive: true });
writeFileSync(join(dir, "data", "fonts", "PalmtermProbe-Regular.ttf"), testFont("Palmterm Probe"));
mkdirSync(join(dir, "myfonts"));
writeFileSync(join(dir, "myfonts", "files.ttf"), testFont("Palmterm Files"));
mkdirSync(join(browserData, "fonts"), { recursive: true });
writeFileSync(join(browserData, "fonts", "PalmtermLocal-Regular.ttf"), testFont("Palmterm Local"));

let server: ChildProcess;
let browser: Browser;
let page: Page;
let base = "";
let lanBase = "";

function tmux(...args: string[]): string {
  return execFileSync("tmux", args, { env, encoding: "utf8" }).trimEnd();
}

async function freePort(): Promise<number> {
  return new Promise((ok) => {
    const s = createServer();
    s.listen(0, "127.0.0.1", () => {
      const port = (s.address() as { port: number }).port;
      s.close(() => ok(port));
    });
  });
}

async function waitUntil(what: string, cond: () => boolean | Promise<boolean>, ms = 5000) {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    if (await cond()) return;
    await new Promise((r) => setTimeout(r, 30));
  }
  throw new Error(`待っても ${what} になりませんでした`);
}

// tmux の問い合わせに端末（wterm）が自分で返す答え（画面の大きさ・カーソルの位置など）。入力ではないので除く。
const TERMINAL_REPLY = /^\x1b(\[[?>]?[\d;]*[tcnR]|\][^\x07]*(\x07|\x1b\\)|P.*\x1b\\)$/s;

/** 画面が端末に送った入力（バイナリのフレーム）を、送った順に文字列で返す。 */
async function sent(): Promise<string[]> {
  const all = await page.evaluate(() => (window as unknown as { __sent: string[] }).__sent);
  return all.filter((s) => !TERMINAL_REPLY.test(s));
}
async function clearSent() {
  await page.evaluate(() => ((window as unknown as { __sent: string[] }).__sent.length = 0));
}

/** キーバーのボタン（表示か読み上げ名で探す）。 */
function key(label: string) {
  return page.locator(`#keybar button:text-is("${label}"), #keybar button[aria-label="${label}"]`).first();
}

async function openPage() {
  await page.goto(`${base}/`);
  await page.waitForFunction(() => document.querySelector(".term-row") !== null);
  await waitUntil("つながる", async () => (await page.locator("#status").isHidden()) && tmux("list-clients").includes(SESSION));
}

function paneText(): string {
  return tmux("capture-pane", "-p", "-J", "-t", SESSION);
}

beforeAll(async () => {
  if (!existsSync(BINARY)) throw new Error(`${BINARY} がありません。先に make してください`);
  tmux("-f", "/dev/null", "new-session", "-d", "-s", SESSION, "-x", "100", "-y", "30");
  const port = await freePort();
  const lanPort = await freePort();
  base = `http://127.0.0.1:${port}`;
  lanBase = `https://127.0.0.1:${lanPort}`;
  server = spawn(
    BINARY,
    ["-listen", `127.0.0.1:${port}`, "-lan", `127.0.0.1:${lanPort}`, "-token", TOKEN, "-session", SESSION, "-config", configPath, "-upload-dir", uploadDir],
    { env, stdio: "ignore" },
  );
  await waitUntil("サーバーが立ち上がる", async () => fetch(`${base}/auth?token=${TOKEN}`, { redirect: "manual" }).then((r) => r.status === 303, () => false));

  browser = await chromium.launch({ executablePath: CHROMIUM, headless: true, env: { ...process.env, XDG_DATA_HOME: browserData } });
  const context = await browser.newContext({ viewport: { width: 400, height: 760 } });
  await context.addInitScript(() => {
    const w = window as unknown as { __sent: string[]; __sockets: WebSocket[] };
    w.__sent = [];
    w.__sockets = [];
    const Orig = WebSocket;
    window.WebSocket = class extends Orig {
      constructor(url: string | URL, protocols?: string | string[]) {
        super(url, protocols);
        w.__sockets.push(this);
      }
    } as typeof WebSocket;
    const decoder = new TextDecoder();
    const orig = WebSocket.prototype.send;
    WebSocket.prototype.send = function (data) {
      if (typeof data !== "string") w.__sent.push(decoder.decode(data as Uint8Array));
      return orig.call(this, data);
    };
  });
  page = await context.newPage();
  await page.goto(`${base}/auth?token=${TOKEN}`);
}, 30_000);

afterAll(async () => {
  await browser?.close();
  server?.kill();
  try {
    tmux("kill-server");
  } catch {
    // もう止まっている
  }
  rmSync(dir, { recursive: true, force: true });
  rmSync(tmuxDir, { recursive: true, force: true });
});

/** tmux を決まった状態に戻す：ウィンドウとペインを1つずつにし、コピーモードを抜け、入力行と履歴を消す。 */
function resetTmux() {
  tmux("kill-window", "-a", "-t", `${SESSION}:0`);
  tmux("kill-pane", "-a", "-t", `${SESSION}:0`);
  if (tmux("display-message", "-p", "-t", SESSION, "#{pane_in_mode}") === "1") tmux("send-keys", "-t", SESSION, "-X", "cancel");
  tmux("send-keys", "-t", SESSION, "C-c");
  tmux("send-keys", "-t", SESSION, "clear", "Enter");
  tmux("clear-history", "-t", SESSION);
}

beforeEach(async () => {
  resetTmux();
  rmSync(configPath, { force: true });
  await openPage();
  await clearSent();
});

describe("キーバー", () => {
  it("既定の並びと英語の表示", async () => {
    const labels = await page.$$eval("#keybar button", (bs) => bs.map((b) => b.textContent || `[${b.getAttribute("aria-label")}]`));
    expect(labels).toEqual([
      "Esc", "Tab", "Ctrl", "Alt", "Shift", "[Left]", "[Down]", "[Up]", "[Right]", "[Enter]",
      "^C", "^D", "Home", "End", "PgUp", "PgDn", "|", "~", "/", "-", "`",
    ]);
    expect(await page.getAttribute("#line", "placeholder")).toBe("Send with Ctrl+Enter or the send button");
    expect(await page.getAttribute("#send", "aria-label")).toBe("Send (Ctrl+Enter)");
  });

  it("押したキーの信号を送る", async () => {
    for (const label of ["Esc", "^C", "Enter", "Left", "PgUp", "|"]) await key(label).click();
    // tmux はカーソルキーをアプリ用の形（ESC O …）にするよう求めてくる
    expect(await sent()).toEqual(["\x1b", "\x03", "\r", "\x1bOD", "\x1b[5~", "|"]);
  });

  it("修飾キー：1回押すと次のキーにだけ効き、すばやく2回で固定、固定中に押すと解除", async () => {
    await key("Ctrl").click();
    expect(await key("Ctrl").getAttribute("data-state")).toBe("once");
    await key("End").click();
    await key("End").click();
    expect(await sent()).toEqual(["\x1b[1;5F", "\x1bOF"]);
    expect(await key("Ctrl").getAttribute("data-state")).toBe("off");

    await key("Alt").click();
    await key("Alt").click();
    expect(await key("Alt").getAttribute("data-state")).toBe("lock");
    await clearSent();
    await key("Home").click();
    await key("Home").click();
    expect(await sent()).toEqual(["\x1b[1;3H", "\x1b[1;3H"]);
    await key("Alt").click();
    expect(await key("Alt").getAttribute("data-state")).toBe("off");

    // 画面の修飾キーは、物理キーボードの特殊キーにも効き、そこで使い切る
    await key("Ctrl").click();
    await page.mouse.click(200, 150);
    await clearSent();
    await page.keyboard.press("ArrowUp");
    await page.keyboard.type("c");
    expect(await sent()).toEqual(["\x1b[1;5A", "c"]);
    expect(await key("Ctrl").getAttribute("data-state")).toBe("off");

    await key("Shift").click();
    await page.waitForTimeout(500);
    await key("Shift").click();
    expect(await key("Shift").getAttribute("data-state")).toBe("off");
  });
});

/** 印の文字を打ち、それが出ている端末の行の位置を返す。 */
async function markerRow(marker: string) {
  tmux("send-keys", "-t", SESSION, "clear", "Enter");
  tmux("send-keys", "-t", SESSION, `echo ${marker}`, "Enter");
  const row = page.locator(".term-row", { hasText: `echo ${marker}` }).first();
  await row.waitFor();
  return (await row.boundingBox())!;
}

describe("入力", () => {
  it("入力欄にフォーカスがあっても、マウスでなぞると端末の文字を選べる", async () => {
    const box = await markerRow("MOUSESELECT");
    await page.locator("#line").focus();
    const y = box.y + box.height / 2;
    await page.mouse.move(box.x + 2, y);
    await page.mouse.down();
    await page.mouse.move(box.x + 90, y, { steps: 8 });
    await page.mouse.up();
    expect(await page.evaluate(() => getSelection()!.toString())).toContain("echo MOUSE");
    expect(await page.evaluate(() => document.activeElement?.id)).not.toBe("line");
    await page.evaluate(() => getSelection()!.removeAllRanges());
  });

  it("テキストボックスは Enter で改行、Ctrl+Enter で送る", async () => {
    await page.locator("#line").click();
    await page.keyboard.type("echo one");
    await page.keyboard.press("Enter");
    expect(await page.inputValue("#line")).toBe("echo one\n");
    await page.keyboard.press("Backspace");
    await page.keyboard.press("Control+Enter");
    expect(await sent()).toEqual(["echo one", "\r"]);
    expect(await page.inputValue("#line")).toBe("");
    await waitUntil("one が出る", () => paneText().includes("\none\n"));
  });

  it("つながっていないときに送っても、文章を消さずに知らせる", async () => {
    await page.locator("#line").fill("keep me");
    await page.evaluate(() => (window as unknown as { __sockets: WebSocket[] }).__sockets.at(-1)!.close());
    await page.locator("#send").click();
    expect(await page.inputValue("#line")).toBe("keep me");
    expect(await page.locator("#toast").textContent()).toContain("Not connected");
    expect(await sent()).toEqual([]);
    // つなぎ直したら送れる
    await waitUntil("つなぎ直す", async () => page.locator("#status").isHidden());
    await page.locator("#send").click();
    expect(await sent()).toEqual(["keep me", "\r"]);
  });

  it("送信ボタンで送り、空のときの Backspace は端末の文字を消す", async () => {
    await page.locator("#line").fill("echo two");
    await page.locator("#send").click();
    await page.locator("#line").click();
    await page.keyboard.press("Backspace");
    expect(await sent()).toEqual(["echo two", "\r", "\x7f"]);
  });

  it("送信ボタンは、文字か添付があるときだけ出す。空の Ctrl+Enter は Enter だけを送る", async () => {
    const send = page.locator("#send");
    const lineWidth = async () => (await page.locator("#line").boundingBox())!.width;
    // 空で添付もないときは出さず、テキストボックスがその分（ボタンの幅と間の 4px）伸びる
    expect(await send.isHidden()).toBe(true);
    const wide = await lineWidth();

    await page.locator("#line").click();
    await page.keyboard.type("a");
    expect(await send.isVisible()).toBe(true);
    const sendWidth = (await send.boundingBox())!.width;
    expect(wide - (await lineWidth())).toBeCloseTo(sendWidth + 4, 0);
    await page.keyboard.press("Backspace");
    expect(await send.isHidden()).toBe(true);
    expect(await lineWidth()).toBeCloseTo(wide, 0);

    // 文字が空でも画像を添付すると出て、× で外すと消える
    await page.locator("#upload-input").setInputFiles({ name: "only.png", mimeType: "image/png", buffer: Buffer.from("89504e470d0a1a0a", "hex") });
    await waitUntil("送信ボタンが出る", () => send.isVisible());
    await page.locator(".att-remove").click();
    expect(await send.isHidden()).toBe(true);

    // 送って空になると消える
    await page.locator("#line").fill("echo three");
    expect(await send.isVisible()).toBe(true);
    await send.click();
    expect(await page.inputValue("#line")).toBe("");
    expect(await send.isHidden()).toBe(true);

    // 空のままの Ctrl+Enter と、キーバーの Ctrl → Enter は Enter だけを送る
    await clearSent();
    await page.locator("#line").click();
    await page.keyboard.press("Control+Enter");
    expect(await sent()).toEqual(["\r"]);
    await key("Ctrl").click();
    await page.locator("#line").focus();
    await page.keyboard.press("Enter");
    expect(await sent()).toEqual(["\r", "\r"]);
    expect(await page.inputValue("#line")).toBe("");
    expect(await send.isHidden()).toBe(true);
  });

  it("端末をタップすれば直接入力。Tab はテキストボックスの書きかけを送らない", async () => {
    await page.locator("#line").fill("LEFTOVER");
    await page.mouse.click(200, 150);
    await page.keyboard.type("ab");
    await key("Tab").click();
    expect((await sent()).join("")).toBe("ab\t");
    expect(await page.inputValue("#line")).toBe("LEFTOVER");

    // テキストボックスに戻ってからの Tab は、書きかけを先に送る
    await page.locator("#line").click();
    await clearSent();
    await key("Tab").click();
    await waitUntil("送る", async () => (await sent()).length === 2);
    expect(await sent()).toEqual(["LEFTOVER", "\t"]);
  });
});

describe("tmux のタブとパネル", () => {
  it("＋で増やし、タブで切り替え、今のタブを押して名前を変え、× を2回で閉じる", async () => {
    await page.locator("#new-window").click();
    await waitUntil("2つ目のタブ", async () => (await page.locator(".tmux-tab").count()) === 2);
    expect(tmux("display-message", "-p", "-t", SESSION, "#{window_index}")).toBe("1");

    await page.locator(".tmux-tab").nth(0).locator(".tmux-tab-name").click();
    await waitUntil("0 に戻る", () => tmux("display-message", "-p", "-t", SESSION, "#{window_index}") === "0");

    await page.locator(".tmux-tab.active .tmux-tab-name").click();
    await page.locator(".tmux-tab-input").fill("work");
    await page.keyboard.press("Enter");
    await waitUntil("名前が変わる", () => tmux("display-message", "-p", "-t", `${SESSION}:0`, "#{window_name}") === "work");
    await waitUntil("タブに出る", async () => (await page.locator(".tmux-tab.active").textContent())?.includes("0:work") ?? false);

    const close = page.locator(".tmux-tab").nth(1).locator(".tmux-tab-close");
    await close.click();
    expect(await close.textContent()).toBe("Close?");
    expect(tmux("list-windows", "-t", SESSION).split("\n")).toHaveLength(2);
    await close.click();
    await waitUntil("1つになる", () => tmux("list-windows", "-t", SESSION).split("\n").length === 1);
  });

  it("palmterm の外で作ったウィンドウや付けた名前も、出力をきっかけにタブに映る", async () => {
    tmux("new-window", "-t", SESSION, "-n", "outside");
    await waitUntil("タブに映る", async () => (await page.locator(".tmux-tab").allTextContents()).some((t) => t.includes("1:outside")));
  });

  it("パネルで分割し、端末をタップしたペインに移り、文字サイズを変える", async () => {
    await page.locator("#tmux-btn").click();
    expect(await page.locator("#tmuxpanel").isVisible()).toBe(true);
    await page.locator('#tmuxpanel button[title="Split side by side"]').click();
    expect(await page.locator("#tmuxpanel").isHidden()).toBe(true);
    await waitUntil("2つのペイン", () => tmux("display-message", "-p", "-t", SESSION, "#{window_panes}") === "2");
    await waitUntil("タブにペインの数", async () => (await page.locator(".tmux-tab-panes").count()) === 1);

    const leftPane = tmux("display-message", "-p", "-t", `${SESSION}:.0`, "#{pane_id}");
    await page.mouse.click(30, 100);
    await waitUntil("左のペインに移る", () => tmux("display-message", "-p", "-t", SESSION, "#{pane_id}") === leftPane);
    const box = await page.locator("#term").boundingBox();
    await page.mouse.click(box!.x + box!.width - 30, 100);
    await waitUntil("右のペインに移る", () => tmux("display-message", "-p", "-t", SESSION, "#{pane_id}") !== leftPane);

    await page.locator("#tmux-btn").click();
    const size = page.locator(".tmux-size");
    const before = Number.parseInt((await size.textContent()) ?? "");
    await page.locator('#tmuxpanel button[title="Larger text"]').click();
    expect(await size.textContent()).toBe(`${before + 1}px`);
    expect(await page.locator("#tmuxpanel").isVisible()).toBe(true); // 文字サイズでは閉じない
    await page.locator('#tmuxpanel button[title="Smaller text"]').click();
    await page.locator("#tmux-btn").click();
    expect(await page.locator("#tmuxpanel").isHidden()).toBe(true);

    tmux("kill-pane", "-t", SESSION);
  });
  it("パネルでキーバーを隠すと端末が広がり、読み込み直しても隠れたまま。別のブラウザでは出たまま。もう一度押すと出る", async () => {
    const termHeight = async () => (await page.locator("#term").boundingBox())!.height;
    const shownHeight = await termHeight();
    const keyrowHeight = (await page.locator("#keyrow").boundingBox())!.height;

    await page.locator("#tmux-btn").click();
    await page.locator('#tmuxpanel button[title="Hide the key bar"]').click();
    expect(await page.locator("#tmuxpanel").isHidden()).toBe(true);
    expect(await page.locator("#keybar").isHidden()).toBe(true);
    // タブ、tmux ボタン（タブバーの右端に移る）、入力バーは残る
    expect(await page.locator("#tabs").isVisible()).toBe(true);
    expect(await page.locator("#tabbar #tmux-btn").isVisible()).toBe(true);
    expect(await page.locator("#inputbar").isVisible()).toBe(true);
    // 端末はキーの段の分だけ広がる（タブバーは下の余白 4px を持つ）
    await waitUntil("端末が広がる", async () => Math.abs((await termHeight()) - (shownHeight + keyrowHeight - 4)) < 1);

    await openPage();
    expect(await page.locator("#keybar").isHidden()).toBe(true);
    expect(await page.locator("#tabbar #tmux-btn").isVisible()).toBe(true);

    const other = await browser.newContext({ viewport: { width: 400, height: 760 } });
    try {
      const otherPage = await other.newPage();
      await otherPage.goto(`${base}/auth?token=${TOKEN}`);
      await otherPage.waitForFunction(() => document.querySelector("#keybar button") !== null);
      expect(await otherPage.locator("#keybar").isVisible()).toBe(true);
    } finally {
      await other.close();
    }

    await page.locator("#tmux-btn").click();
    await page.locator('#tmuxpanel button[title="Show the key bar"]').click();
    expect(await page.locator("#keybar").isVisible()).toBe(true);
    expect(await page.locator("#keyrow #tmux-btn").isVisible()).toBe(true);
    await waitUntil("端末が元の高さに戻る", async () => Math.abs((await termHeight()) - shownHeight) < 1);
  });
});

describe("設定ファイル", () => {
  it("日本語と独自のキー。書き間違えたキーは飛ばして知らせる", async () => {
    writeFileSync(
      configPath,
      `language = "ja"
[[keys]]
key = "shift+tab"
label = "S-Tab"
[[keys]]
key = "nope+x"
[[keys]]
text = ":wq"
`,
    );
    await openPage();
    const labels = await page.$$eval("#keybar button", (bs) => bs.map((b) => b.textContent));
    expect(labels).toEqual(["S-Tab", ":wq"]);
    expect(await page.getAttribute("#line", "placeholder")).toBe("Ctrl+Enter か送信ボタンで送る");
    expect(await page.locator("#toast").textContent()).toContain("キー 2 番目");
    await clearSent();
    await key("S-Tab").click();
    expect(await sent()).toEqual(["\x1b[Z"]);
  });

  it("[theme] でテーマを選ぶと端末とボタンの色が変わり、書いた色だけ上書きする。書き間違えた色は知らせて既定に戻す", async () => {
    const colors = () =>
      page.evaluate(() => {
        const term = getComputedStyle(document.getElementById("term")!);
        const key = getComputedStyle(document.querySelector("#keybar button")!);
        return {
          termBg: term.backgroundColor,
          termFg: term.getPropertyValue("--term-fg").trim(),
          red: term.getPropertyValue("--term-color-1").trim(),
          blue: term.getPropertyValue("--term-color-4").trim(),
          keyBg: key.backgroundColor,
          tmuxBtn: getComputedStyle(document.getElementById("tmux-btn")!).color,
        };
      });
    const before = await colors();
    expect(before.termBg).toBe("rgb(30, 30, 30)");

    writeFileSync(configPath, `[theme]\nname = "light"\n`);
    await openPage();
    const light = await colors();
    expect(light.termBg).toBe("rgb(255, 255, 255)");
    expect(light.keyBg).not.toBe(before.keyBg);
    expect(light.red).toBe("#cf222e");

    writeFileSync(configPath, `[theme]\nname = "light"\nbackground = "#fafafa"\naccent = "#0969da"\nblue = "#0000ff"\nred = "#12345"\n`);
    await openPage();
    const custom = await colors();
    expect(custom.termBg).toBe("rgb(250, 250, 250)");
    expect(custom.tmuxBtn).toBe("rgb(9, 105, 218)");
    expect(custom.blue).toBe("#0000ff");
    expect(custom.red).toBe("#cf222e"); // 書き間違えた色は light の色のまま
    expect(custom.termFg).toBe(light.termFg); // 書いていない色もそのまま
    expect(custom.keyBg).toBe(light.keyBg);
    expect(await page.locator("#toast").textContent()).toContain("[theme] red");

    rmSync(configPath, { force: true });
    await openPage();
    expect(await colors()).toEqual(before);
  });

  it("[font] の family・file のフォントを PC から配って端末の文字に使う。PC に無ければスマホのもの、それも無ければ JetBrains Mono。アイコンは変わらない", async () => {
    // 文字とアイコン（U+F418）を出し、それぞれ実際にどのフォントで描いたかをブラウザに訊く。
    tmux("send-keys", "-t", SESSION, "printf 'WWW \\357\\220\\230 X\\n'", "Enter");
    const cdp = await page.context().newCDPSession(page);
    await cdp.send("DOM.enable");
    await cdp.send("CSS.enable");
    const usedFont = async (what: "text" | "icon"): Promise<string> => {
      let found = "";
      await waitUntil(`${what} のフォントがわかる`, async () => {
        const marked = await page.evaluate((what) => {
          document.querySelectorAll("[data-probe]").forEach((e) => e.removeAttribute("data-probe"));
          const want = what === "text" ? "WWW" : "\uf418";
          const walker = document.createTreeWalker(document.getElementById("term")!, NodeFilter.SHOW_TEXT);
          for (let n = walker.nextNode(); n; n = walker.nextNode()) {
            const text = n.textContent ?? "";
            if (text.includes(want) && !text.includes("printf")) {
              n.parentElement!.setAttribute("data-probe", "");
              return true;
            }
          }
          return false;
        }, what);
        if (!marked || !(await page.evaluate(() => document.fonts.ready.then(() => true)))) return false;
        const { root } = await cdp.send("DOM.getDocument");
        const { nodeId } = await cdp.send("DOM.querySelector", { nodeId: root.nodeId, selector: "[data-probe]" });
        if (!nodeId) return false;
        const { fonts } = await cdp.send("CSS.getPlatformFontsForNode", { nodeId });
        found = fonts.map((f) => f.familyName).join(",");
        return found !== "";
      });
      return found;
    };
    const icon = await usedFont("icon");
    expect(await usedFont("text")).toBe("JetBrains Mono");

    // PC に入っているフォントの名前。スマホ（ブラウザ）には無いので、PC から配ったものを使う。
    writeFileSync(configPath, `[font]\nfamily = "Palmterm Probe"\n`);
    await openPage();
    expect(await usedFont("text")).toBe("Palmterm Probe");
    expect(await usedFont("icon")).toBe(icon);

    // PC に無くスマホにある名前なら、スマホのものを使う。
    writeFileSync(configPath, `[font]\nfamily = "Palmterm Local"\n`);
    await openPage();
    expect(await usedFont("text")).toBe("Palmterm Local");
    expect(await usedFont("icon")).toBe(icon);

    // どちらにも無ければ JetBrains Mono。
    writeFileSync(configPath, `[font]\nfamily = "Palmterm Nowhere"\n`);
    await openPage();
    expect(await usedFont("text")).toBe("JetBrains Mono");

    // file は設定ファイルのあるフォルダから数える。
    writeFileSync(configPath, `[font]\nfile = "myfonts/files.ttf"\n`);
    await openPage();
    expect(await usedFont("text")).toBe("Palmterm Files");
    expect(await usedFont("icon")).toBe(icon);
    // ログインしていないブラウザには配らない。
    const res = await fetch(`${base}/api/font`, { redirect: "manual" });
    expect(res.status).not.toBe(200);

    // 使えないファイルは知らせて、JetBrains Mono に戻す。
    writeFileSync(configPath, `[font]\nfile = "myfonts/missing.ttf"\n`);
    await openPage();
    expect(await page.locator("#toast").textContent()).toContain("[font]");
    expect(await usedFont("text")).toBe("JetBrains Mono");
    await cdp.detach();
  });
});

describe("複数行の送信", () => {
  it("ブラケットペーストで送り、中に終わりの印を仕込まれても貼り付けから抜け出せない", async () => {
    // 抜け出せてしまうと、続くコマンド（ここでは何もしない ":"）が実行される
    await page.locator("#line").fill("echo safe\x1b[201~\n: injected\n");
    await clearSent();
    await page.locator("#send").click();
    await waitUntil("送る", async () => (await sent()).includes("\r"));
    // tmux はブラケットペーストを有効にしている。印は最初と最後の1組だけ
    expect(await sent()).toEqual(["\x1b[200~echo safe[201~\r: injected\r\x1b[201~", "\r"]);
  });
});

describe("画像とコピー", () => {
  it("テキストボックスで添付した画像は、送信で先にパスを貼ってから文章と Enter を送る", async () => {
    await page.locator("#line").fill("look");
    const png = Buffer.from("89504e470d0a1a0a", "hex");
    await page.locator("#upload-input").setInputFiles({ name: "shot.png", mimeType: "image/png", buffer: png });
    await waitUntil("アップロードが終わる", async () => (await page.locator('.att[data-status="done"]').count()) === 1);
    await page.locator("#send").click();
    await waitUntil("送り終わる", async () => (await sent()).includes("\r"), 5000);
    const out = await sent();
    expect(out).toHaveLength(3);
    // パスだけの貼り付け（tmux がブラケットペーストを有効にしているので印で囲む）
    expect(out[0]).toMatch(new RegExp(`^\\x1b\\[200~${uploadDir}/\\d{8}-\\d{6}-shot\\.png\\x1b\\[201~$`));
    expect(out.slice(1)).toEqual(["look", "\r"]);
    expect(await page.locator("#attachments").isHidden()).toBe(true);
  });

  it("画像を送った直後に切れても、文章は消さずに残す", async () => {
    await page.locator("#line").fill("after image");
    await page.locator("#upload-input").setInputFiles({ name: "cut.png", mimeType: "image/png", buffer: Buffer.from("89504e47", "hex") });
    await waitUntil("アップロードが終わる", async () => (await page.locator('.att[data-status="done"]').count()) === 1);
    await page.locator("#send").click();
    await waitUntil("パスを送る", async () => (await sent()).some((s) => s.includes("cut.png")));
    await page.evaluate(() => (window as unknown as { __sockets: WebSocket[] }).__sockets.at(-1)!.close()); // 画像のあとの待ちの間に切れる
    await page.waitForTimeout(600);
    expect(await page.inputValue("#line")).toBe("after image");
    expect(await page.locator("#toast").textContent()).toContain("Not connected");
    expect((await sent()).some((s) => s.includes("after image"))).toBe(false);
  });

  it("コピーモードで tmux の履歴を出す", async () => {
    tmux("send-keys", "-t", SESSION, "echo history$((2+3))", "Enter");
    await waitUntil("出力", () => paneText().includes("history5"));
    await page.locator("#copy").click();
    await page.locator("#tab-history").click();
    await waitUntil("履歴が出る", async () => (await page.locator("#copy-text").textContent())?.includes("\nhistory5\n") ?? false);
    await page.locator("#copy-close").click();
    expect(await page.locator("#copymode").isHidden()).toBe(true);
  });
});

describe("タッチ操作", () => {
  let cdp: CDPSession;
  beforeAll(async () => {
    cdp = await page.context().newCDPSession(page);
    await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 5 });
  });
  afterAll(async () => {
    await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: false });
    await cdp.detach();
  });

  /** 指の操作を送る（CDP の Input.dispatchTouchEvent）。points は指ごとの [x, y]。 */
  async function touch(type: "touchStart" | "touchMove" | "touchEnd", points: [number, number][]) {
    await cdp.send("Input.dispatchTouchEvent", { type, touchPoints: points.map(([x, y], id) => ({ x, y, id })) });
  }
  const fontSize = () => page.evaluate(() => document.getElementById("term")!.style.getPropertyValue("--term-font-size"));

  it("2本指で広げると文字が大きくなり、離したら保存する", async () => {
    await page.evaluate(() => localStorage.removeItem("palmterm.fontSize"));
    await openPage();
    expect(await fontSize()).toBe("13px");
    await touch("touchStart", [[150, 200], [250, 200]]);
    for (const d of [60, 80, 100]) await touch("touchMove", [[200 - d, 200], [200 + d, 200]]);
    await touch("touchEnd", []);
    expect(await fontSize()).toBe("26px"); // 指の間が 100px → 200px
    expect(await page.evaluate(() => localStorage.getItem("palmterm.fontSize"))).toBe("26");

    // 上限（32px）より大きくはならない
    await touch("touchStart", [[190, 200], [210, 200]]);
    await touch("touchMove", [[100, 200], [300, 200]]);
    await touch("touchEnd", []);
    expect(await fontSize()).toBe("32px");
    await page.evaluate(() => localStorage.removeItem("palmterm.fontSize"));
  });

  async function tapKey(label: string) {
    const box = (await key(label).boundingBox())!;
    const at: [number, number] = [box.x + box.width / 2, box.y + box.height / 2];
    await touch("touchStart", [at]);
    await touch("touchEnd", []);
  }
  const focused = () => page.evaluate(() => (document.activeElement?.closest("#term") ? "term" : document.activeElement?.id || document.activeElement?.tagName));

  it("キーボードが出ていないときにキーバーを指で押しても、キーボードを出さない（入力欄のフォーカスを外す）", async () => {
    expect(await focused()).toBe("term"); // wterm は起動時に自分の入力欄にフォーカスする
    await tapKey("Ctrl");
    expect(await focused()).toBe("BODY");
    expect(await key("Ctrl").getAttribute("data-state")).toBe("once");
    await tapKey("Esc");
    expect(await sent()).toEqual(["\x1b"]);
  });

  it("キーボードが出ているときは、キーバーを押してもフォーカスを残す（キーボードを閉じない）", async () => {
    await page.locator("#line").focus();
    await page.setViewportSize({ width: 400, height: 400 }); // キーボードが出て画面が縮んだ
    try {
      // 下の段が新しい高さに収まるまで待つ
      await waitUntil("縮んだ画面に収まる", async () => ((await page.locator("#inputbar").boundingBox())?.y ?? 999) < 400);
      await tapKey("Esc");
      expect(await focused()).toBe("line");
      expect(await sent()).toEqual(["\x1b"]);
    } finally {
      await page.setViewportSize({ width: 400, height: 760 });
    }
  });

  it("入力欄にフォーカスがあっても、長押ししてから指を動かすと端末の文字を選べる（スクロールしない）", async () => {
    const box = await markerRow("TOUCHSELECT");
    await page.locator("#line").focus();
    const y = box.y + box.height / 2;
    await touch("touchStart", [[box.x + 2, y]]);
    await page.waitForTimeout(600); // 長押し
    expect(await focused()).toBe("BODY"); // 選んだ文字をコピーできるように入力欄から外す
    for (let x = box.x + 20; x <= box.x + 90; x += 10) await touch("touchMove", [[x, y]]);
    await touch("touchEnd", []);
    expect(await page.evaluate(() => getSelection()!.toString())).toContain("echo TOUCH");
    expect(tmux("display-message", "-p", "-t", SESSION, "#{pane_in_mode}")).toBe("0");
    await page.evaluate(() => getSelection()!.removeAllRanges());
  });

  it("シェルの上で下へスワイプすると tmux の履歴をさかのぼる", async () => {
    await page.evaluate(() => localStorage.removeItem("palmterm.fontSize"));
    await openPage();
    for (let i = 0; i < 40; i++) tmux("send-keys", "-t", SESSION, `echo swipe${i}`, "Enter");
    await waitUntil("出力", () => paneText().includes("swipe39"));
    await touch("touchStart", [[200, 100]]);
    for (let y = 120; y <= 400; y += 20) await touch("touchMove", [[200, y]]);
    await page.waitForTimeout(100);
    await touch("touchEnd", []);
    await waitUntil("コピーモードでさかのぼる", () => {
      const [mode, pos] = tmux("display-message", "-p", "-t", SESSION, "#{pane_in_mode} #{scroll_position}").split(" ");
      return mode === "1" && Number(pos) > 5;
    });
    tmux("send-keys", "-t", SESSION, "-X", "cancel");
  });

  it("シェルの上でマウスのホイールを回しても tmux の履歴をさかのぼる（シェルはマウスを受け取らない）", async () => {
    for (let i = 0; i < 40; i++) tmux("send-keys", "-t", SESSION, `echo wheel${i}`, "Enter");
    await waitUntil("出力", () => paneText().includes("wheel39"));
    await page.mouse.move(200, 200);
    for (let i = 0; i < 5; i++) await page.mouse.wheel(0, -100);
    await waitUntil("コピーモードでさかのぼる", () => {
      const [mode, pos] = tmux("display-message", "-p", "-t", SESSION, "#{pane_in_mode} #{scroll_position}").split(" ");
      return mode === "1" && Number(pos) > 5;
    });
    tmux("send-keys", "-t", SESSION, "-X", "cancel");
  });
});

describe("LAN（HTTPS）", () => {
  it("自分で署名した証明書の HTTPS でログインでき、端末につながり、クリップボードを読める安全な接続になる", async () => {
    const context = await browser.newContext({ viewport: { width: 400, height: 760 }, ignoreHTTPSErrors: true });
    try {
      const lan = await context.newPage();
      await lan.goto(`${lanBase}/auth?token=${TOKEN}`);
      expect(lan.url()).toBe(`${lanBase}/`);
      const cookie = (await context.cookies()).find((c) => c.name === "palmterm_token");
      expect(cookie?.secure).toBe(true);
      await lan.waitForFunction(() => document.querySelector(".term-row") !== null);
      await waitUntil("つながる", async () => lan.locator("#status").isHidden());
      // HTTPS なので、ブラウザのクリップボードの機能（読み取りを含む）が使える安全な接続になっている
      expect(await lan.evaluate(() => window.isSecureContext && typeof navigator.clipboard?.readText === "function")).toBe(true);

      await lan.locator("#line").fill("echo lan$((1+2))");
      await lan.locator("#send").click();
      await waitUntil("lan3 が出る", () => paneText().includes("\nlan3\n"));
    } finally {
      await context.close();
    }
  });
});

describe("別のサイトから", () => {
  it("ログイン済みでも、別のサイトのページからは tmux を操作できない", async () => {
    const before = tmux("list-windows", "-t", SESSION).split("\n").length;
    // localhost は 127.0.0.1 とは別のサイト。そこから Cookie 付きで POST させる。
    const other = await page.context().newPage();
    await other.goto(base.replace("127.0.0.1", "localhost") + "/nothing");
    const status = await other.evaluate(async (target) => {
      try {
        const r = await fetch(`${target}/api/tmux`, { method: "POST", credentials: "include", body: '{"action":"new-window"}' });
        return r.status;
      } catch {
        return "blocked";
      }
    }, base);
    await other.close();
    expect(status === 401 || status === "blocked").toBe(true);
    expect(tmux("list-windows", "-t", SESSION).split("\n")).toHaveLength(before);
  });
});

describe("ログイン中の端末", () => {
  it("一覧で端末に名前を付けられ、ほかの端末を取り消すとその端末はつながらなくなり、自分はログアウトできる", async () => {
    const IPHONE = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1";
    const phoneContext = await browser.newContext({ viewport: { width: 400, height: 760 }, userAgent: IPHONE });
    const pcContext = await browser.newContext({ viewport: { width: 400, height: 760 } });
    try {
      const phone = await phoneContext.newPage();
      await phone.goto(`${base}/auth?token=${TOKEN}`);
      await waitUntil("スマホがつながる", async () => (await phone.locator(".term-row").count()) > 0 && (await phone.locator("#status").isHidden()));
      const pc = await pcContext.newPage();
      await pc.goto(`${base}/auth?token=${TOKEN}`);
      await pc.waitForFunction(() => document.querySelector(".term-row") !== null);

      // tmux ボタンのパネルから一覧を開く。自分には印が付き、ボタンはログアウト
      await pc.locator("#tmux-btn").click();
      await pc.locator("#tmuxpanel button", { hasText: "Devices" }).click();
      const phoneRow = pc.locator(".device", { hasText: "iPhone · Safari" });
      await waitUntil("一覧が出る", async () => (await phoneRow.count()) === 1);
      const me = pc.locator(".device.current");
      expect(await me.count()).toBe(1);
      expect(await me.locator(".device-revoke").textContent()).toBe("Log out");

      // 名前を押して付ける。付けた名前が出て、自動の名前は下の行に残る。空にすると戻る
      await phoneRow.locator(".device-name").click();
      await pc.locator(".device-name-input").fill("仕事用スマホ");
      await pc.locator(".device-name-input").press("Enter");
      await waitUntil("名前が付く", async () => (await phoneRow.locator(".device-name").textContent()) === "仕事用スマホ");
      expect(await phoneRow.locator(".device-meta").textContent()).toContain("iPhone · Safari");
      await phoneRow.locator(".device-name").click();
      await pc.locator(".device-name-input").fill("");
      await pc.locator(".device-name-input").press("Enter");
      await waitUntil("自動の名前に戻る", async () => (await phoneRow.locator(".device-name").textContent()) === "iPhone · Safari");
      await phoneRow.locator(".device-name").click();
      await pc.locator(".device-name-input").fill("仕事用スマホ");
      await pc.locator(".device-name-input").press("Enter");
      await waitUntil("名前が付く", async () => (await phoneRow.locator(".device-name").textContent()) === "仕事用スマホ");

      // スマホを2回押しで取り消す。スマホはつなぎ直さず、ログインし直す案内になる
      await phoneRow.locator(".device-revoke").click();
      expect(await phoneRow.locator(".device-revoke").textContent()).toBe("Revoke?");
      await phoneRow.locator(".device-revoke").click();
      await waitUntil("一覧から消える", async () => (await phoneRow.count()) === 0);
      await waitUntil("スマホがログインし直す案内になる", async () => ((await phone.textContent("body").catch(() => "")) ?? "").includes("/auth?token="), 10_000);

      // 自分をログアウトする
      await me.locator(".device-revoke").click();
      await me.locator(".device-revoke").click();
      await waitUntil("ログインし直す案内になる", async () => ((await pc.textContent("body").catch(() => "")) ?? "").includes("/auth?token="));
      expect((await pcContext.cookies()).find((c) => c.name === "palmterm_token")).toBeUndefined();

      // ほかの端末（いつもの画面）はそのまま使える
      await page.locator("#line").fill("echo still$((2+3))");
      await page.locator("#send").click();
      await waitUntil("still5 が出る", () => paneText().includes("\nstill5\n"));
    } finally {
      await phoneContext.close();
      await pcContext.close();
    }
  });
});

describe("ブラウザの受け渡し", () => {
  it("tmux の中のプログラムが開こうとした URL を画面に知らせ、押すと新しいタブで開く", async () => {
    // tmux の中のプログラムは BROWSER（palmterm-open）でブラウザを開く
    expect(tmux("show-environment", "-g", "BROWSER")).toMatch(/^BROWSER=.*\/palmterm-open$/);
    const target = `${base}/nothing?from=relay`;
    execFileSync(BINARY, ["open", target], { env });
    const bar = page.locator("#linkbar");
    await waitUntil("知らせが出る", async () => bar.isVisible());
    expect(await bar.textContent()).toContain(target);

    const [opened] = await Promise.all([page.context().waitForEvent("page"), page.locator("#linkbar-open").click()]);
    expect(opened.url()).toBe(target);
    await opened.close();
    expect(await bar.isHidden()).toBe(true);

    // × で閉じられる
    execFileSync(BINARY, ["open", target], { env });
    await waitUntil("知らせが出る", async () => bar.isVisible());
    await page.locator("#linkbar-close").click();
    expect(await bar.isHidden()).toBe(true);
  });
});


describe("palmterm のクリップボード", () => {
  it("tmux の中のプログラムがコピーした文字を知らせ、押すとスマホのクリップボードに入れる。一覧からも使える", async () => {
    await page.context().grantPermissions(["clipboard-read", "clipboard-write"], { origin: base });
    await page.evaluate(() => navigator.clipboard.writeText(""));
    // OSC 52 で "from app\nline 2"（tmux の set-clipboard を on にしているのでバッファに入る）
    tmux("send-keys", "-t", SESSION, `printf '\\033]52;c;%s\\a' "$(printf 'from app\\nline 2' | base64)"`, "Enter");
    const bar = page.locator("#clipbar");
    await waitUntil("知らせが出る", async () => bar.isVisible());
    expect(await bar.textContent()).toContain("from app");
    await page.locator("#clipbar-copy").click();
    await waitUntil("スマホのクリップボードに入る", async () => (await page.evaluate(() => navigator.clipboard.readText())) === "from app\nline 2");
    expect(await bar.isHidden()).toBe(true);

    // 一覧：新しい順に並び、入力欄へ入れられ、2回押しで消せる
    tmux("set-buffer", "second");
    // ボタンで開くと、最初はクリップボードのタブ
    await page.locator("#copy").click();
    expect(await page.locator("#tab-clips").getAttribute("aria-selected")).toBe("true");
    const texts = () => page.$$eval("#clips .clip-text", (els) => els.map((e) => e.textContent));
    await waitUntil("一覧が出る", async () => (await texts()).length === 2);
    expect(await texts()).toEqual(["second", "from app\nline 2"]);
    expect(await page.locator("#copy-text").isHidden()).toBe(true);

    await page.locator("#clips li").nth(0).locator(".clip-delete").click();
    await page.locator("#clips li").nth(0).locator(".clip-delete").click();
    await waitUntil("消える", async () => (await texts()).length === 1);
    expect(tmux("list-buffers", "-F", "#{buffer_sample}")).toBe("from app\\nline 2");

    await page.locator("#line").fill("");
    await page.locator("#clips li").nth(0).locator(".clip-to-line").click();
    expect(await page.locator("#copymode").isHidden()).toBe(true);
    expect(await page.inputValue("#line")).toBe("from app\nline 2");

    // ボタンで開き直すと、最後に開いていたタブで開く
    await page.locator("#copy").click();
    await page.locator("#tab-history").click();
    expect(await page.locator("#clips").isHidden()).toBe(true);
    await page.locator("#copy-close").click();
    await page.locator("#copy").click();
    expect(await page.locator("#tab-history").getAttribute("aria-selected")).toBe("true");
    await page.locator("#copy-close").click();
    await page.locator("#line").fill("");
  });
});
