// README 用のスクリーンショット（docs/screenshot.png）を撮る。先に `make` で ../palmterm を作っておく。
//   cd web && npm run screenshot
// ふだんの環境が写らないよう、テスト専用の tmux サーバー（シェルは /bin/sh、決まったプロンプト）と
// 一時ディレクトリの設定で palmterm を動かし、chromium で開いて撮る。

import { execFileSync, spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { chromium } from "playwright-core";

const root = resolve(import.meta.dirname, "../..");
const out = join(root, "docs/screenshot.png");
const dir = mkdtempSync(join(tmpdir(), "palmterm-shot-"));
const tmuxDir = mkdtempSync("/tmp/pt-shot-");
const env = {
  ...process.env,
  TMUX_TMPDIR: tmuxDir,
  TMUX: "",
  SHELL: "/bin/sh",
  ENV: "",
  PS1: "\uf07c palmterm \ue725 main \u276f ", // Nerd Font のアイコンを見せる
  XDG_CONFIG_HOME: dir,
  LS_COLORS: "",
};
const tmux = (...args) => execFileSync("tmux", args, { env, encoding: "utf8" });
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const port = await new Promise((ok) => {
  const s = createServer().listen(0, "127.0.0.1", () => {
    const p = s.address().port;
    s.close(() => ok(p));
  });
});

tmux("-f", "/dev/null", "new-session", "-d", "-s", "demo", "-n", "shell", "-c", root, "-x", "50", "-y", "40");
tmux("set", "-g", "status-style", "bg=green,fg=black");
tmux("set", "-g", "status-right", " palmterm ");
tmux("new-window", "-d", "-n", "claude", "-c", root);
tmux("new-window", "-d", "-n", "logs", "-c", root);

const server = spawn(
  join(root, "palmterm"),
  ["-listen", `127.0.0.1:${port}`, "-token", "demo", "-session", "demo", "-config", join(dir, "config.toml"), "-upload-dir", join(dir, "up")],
  { env, stdio: "ignore" },
);
const browser = await chromium.launch({ executablePath: process.env.CHROMIUM ?? "/usr/bin/chromium" });
try {
  await sleep(500);
  const page = await browser.newPage({ viewport: { width: 412, height: 700 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true });
  await page.goto(`http://127.0.0.1:${port}/auth?token=demo`);
  await page.waitForFunction(() => document.querySelector(".term-row") !== null);
  await page.waitForFunction(() => document.getElementById("status")?.hidden);
  // つないだ直後は仮の大きさなので、画面からの大きさが tmux に届いて落ち着くまで待ってから打つ。
  for (let last = "", stable = 0; stable < 5; await sleep(100)) {
    const size = tmux("list-clients", "-t", "demo", "-F", "#{client_width}x#{client_height}");
    stable = size === last ? stable + 1 : 0;
    last = size;
  }

  const type = async (line) => {
    tmux("send-keys", "-t", "demo:shell", line, "Enter");
    await sleep(300);
  };
  await type("ls --color=always -F");
  await type("go test -count=1 ./...");
  for (let i = 0; i < 100 && !tmux("capture-pane", "-p", "-t", "demo:shell").match(/^(ok|FAIL)/m); i++) await sleep(200);

  await page.locator("#line").fill("Fix the flaky test");
  await page.locator("#line").blur();
  await sleep(1500); // タブの取り直しと描き直しを待つ
  await page.screenshot({ path: out });
  console.log(`saved ${out}`);
} finally {
  await browser.close();
  server.kill();
  try {
    tmux("kill-server");
  } catch {}
  rmSync(dir, { recursive: true, force: true });
  rmSync(tmuxDir, { recursive: true, force: true });
}
