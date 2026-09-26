// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Attachments } from "./attachments";

/** アップロードを手で終わらせられるようにした fetch。 */
function fakeUploads() {
  const pending: { name: string; resolve: (ok: boolean) => void }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_url: string, init: RequestInit) => {
      const name = ((init.body as FormData).get("file") as File).name;
      const ok = await new Promise<boolean>((resolve) => pending.push({ name, resolve }));
      return ok
        ? new Response(JSON.stringify({ files: [{ path: `/up/${name}`, name }] }))
        : new Response("broken image", { status: 500 });
    }),
  );
  const settle = async (name: string, ok = true) => {
    await vi.waitFor(() => expect(pending.some((p) => p.name === name)).toBe(true));
    pending.find((p) => p.name === name)!.resolve(ok);
    await new Promise((r) => setTimeout(r, 0));
  };
  return { settle };
}

const file = (name: string) => new File(["x"], name, { type: "image/png" });

describe("Attachments", () => {
  beforeEach(() => {
    URL.createObjectURL = () => "blob:x";
    URL.revokeObjectURL = () => {};
  });
  afterEach(() => vi.unstubAllGlobals());

  it("送り始めた時点の画像だけを、終わるのを待って並んだ順に返す。送ったものを外すと残りだけになる", async () => {
    const up = fakeUploads();
    const bar = document.createElement("div");
    const att = new Attachments(bar);
    att.add([file("a.png"), file("b.png")]);
    const waiting = att.waitAll();
    att.add([file("late.png")]); // 待っている間に足した
    await up.settle("b.png");
    await up.settle("a.png");
    const result = await waiting;
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.entries.map((e) => e.path)).toEqual(["/up/a.png", "/up/b.png"]);
    for (const e of result.entries) e.remove();
    expect(att.count).toBe(1); // 足した画像は残る
    expect(bar.hidden).toBe(false);
  });

  it("待っている間に外した画像は present() が false", async () => {
    const up = fakeUploads();
    const bar = document.createElement("div");
    const att = new Attachments(bar);
    att.add([file("a.png"), file("b.png")]);
    const waiting = att.waitAll();
    (bar.querySelectorAll(".att-remove")[0] as HTMLButtonElement).click();
    await up.settle("a.png");
    await up.settle("b.png");
    const result = await waiting;
    if (!result.ok) throw new Error("ok のはず");
    expect(result.entries.filter((e) => e.present()).map((e) => e.path)).toEqual(["/up/b.png"]);
  });

  it("終わらないアップロードを × で外したら、待つのをやめる（送信が止まらない）", async () => {
    const up = fakeUploads();
    const bar = document.createElement("div");
    const att = new Attachments(bar);
    att.add([file("stuck.png"), file("b.png")]);
    const waiting = att.waitAll();
    await up.settle("b.png");
    (bar.querySelectorAll(".att-remove")[0] as HTMLButtonElement).click(); // stuck.png は終わらせない
    const result = await Promise.race([waiting, new Promise((r) => setTimeout(() => r("まだ待っている"), 100))]);
    expect(result).toEqual({ ok: true, entries: [expect.objectContaining({ path: "/up/b.png" })] });
  });

  it("アップロードに失敗した画像があれば ok: false", async () => {
    const up = fakeUploads();
    const att = new Attachments(document.createElement("div"));
    att.add([file("a.png"), file("bad.heic")]);
    const waiting = att.waitAll();
    await up.settle("a.png");
    await up.settle("bad.heic", false);
    expect(await waiting).toEqual({ ok: false, failed: 1, error: "broken image" });
  });
});
