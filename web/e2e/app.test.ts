// @vitest-environment node
// 実際のブラウザ（chromium）で画面を動かす通しのテスト。先に `make` で ../palmterm を作っておく。
// palmterm は、このテストだけの tmux サーバー（TMUX_TMPDIR を分ける、シェルは /bin/sh）と
// 一時ディレクトリの設定ファイル・アップロード先で動かすので、ふだんの環境には触れない。

import { type ChildProcess, execFileSync, spawn } from "node:child_process";
import { existsSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { type Browser, chromium, type Page } from "playwright-core";
import { afterAll, beforeAll, beforeEach, describe, expect, it } from "vitest";

const BINARY = resolve(__dirname, "../../palmterm");
const CHROMIUM = process.env.CHROMIUM ?? "/usr/bin/chromium";
const TOKEN = "e2etoken";
const SESSION = "e2e";

const dir = mkdtempSync(join(tmpdir(), "palmterm-e2e-"));
const tmuxDir = mkdtempSync("/tmp/pt-e2e-"); // tmux のソケットのパスは短くないといけない
const configPath = join(dir, "config.toml");
const uploadDir = join(dir, "uploads");
const env = { ...process.env, TMUX_TMPDIR: tmuxDir, TMUX: "", SHELL: "/bin/sh", ENV: "", PS1: "$ " };

let server: ChildProcess;
let browser: Browser;
let page: Page;
let base = "";

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
  base = `http://127.0.0.1:${port}`;
  server = spawn(BINARY, ["-listen", `127.0.0.1:${port}`, "-token", TOKEN, "-session", SESSION, "-config", configPath, "-upload-dir", uploadDir], { env, stdio: "ignore" });
  await waitUntil("サーバーが立ち上がる", async () => fetch(`${base}/auth?token=${TOKEN}`, { redirect: "manual" }).then((r) => r.status === 303, () => false));

  browser = await chromium.launch({ executablePath: CHROMIUM, headless: true });
  const context = await browser.newContext({ viewport: { width: 400, height: 760 } });
  await context.addInitScript(() => {
    const w = window as unknown as { __sent: string[] };
    w.__sent = [];
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

beforeEach(async () => {
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

    await key("Shift").click();
    await page.waitForTimeout(500);
    await key("Shift").click();
    expect(await key("Shift").getAttribute("data-state")).toBe("off");
  });
});

describe("入力", () => {
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

  it("送信ボタンで送り、空のときの Backspace は端末の文字を消す", async () => {
    await page.locator("#line").fill("echo two");
    await page.locator("#send").click();
    await page.locator("#line").click();
    await page.keyboard.press("Backspace");
    expect(await sent()).toEqual(["echo two", "\r", "\x7f"]);
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

  it("コピーモードで tmux の履歴を出す", async () => {
    tmux("send-keys", "-t", SESSION, "echo history$((2+3))", "Enter");
    await waitUntil("出力", () => paneText().includes("history5"));
    await page.locator("#copy").click();
    await waitUntil("履歴が出る", async () => (await page.locator("#copy-text").textContent())?.includes("\nhistory5\n") ?? false);
    await page.locator("#copy-close").click();
    expect(await page.locator("#copymode").isHidden()).toBe(true);
  });
});
