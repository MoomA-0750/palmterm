import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Refresher } from "./refresher";

describe("Refresher", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  /** 取り直しの記録。changed で「結果が変わったか」を返す。 */
  function setup(changed = () => false) {
    const calls: number[] = [];
    const r = new Refresher(async () => {
      calls.push(Date.now());
      return changed();
    });
    return { r, calls };
  }

  it("出力があれば少し待って取り直し、出力が続いても間隔を空ける", async () => {
    const { r, calls } = setup(() => true); // 毎回変わる → 間隔は 1 秒のまま
    const start = Date.now();
    for (let t = 0; t < 3000; t += 50) {
      r.outputSeen();
      await vi.advanceTimersByTimeAsync(50);
    }
    expect(calls.map((c) => c - start)).toEqual([250, 1250, 2250]);
  });

  it("結果が変わらない間は間隔を広げ（最大 8 秒）、変わったら 1 秒に戻す", async () => {
    let change = false;
    const { r, calls } = setup(() => change);
    const start = Date.now();
    const pump = async (ms: number) => {
      for (let t = 0; t < ms; t += 50) {
        r.outputSeen();
        await vi.advanceTimersByTimeAsync(50);
      }
    };
    await pump(20_000);
    const gaps = calls.slice(1).map((c, i) => c - calls[i]);
    expect(calls[0] - start).toBe(250);
    expect(gaps).toEqual([2000, 4000, 8000]); // 変わらないたびに倍に（最大 8 秒）

    change = true;
    const before = calls.length;
    await pump(10_000);
    const after = calls.slice(before - 1);
    expect(after.slice(1).map((c, i) => c - after[i])[0]).toBe(8000); // 変わったと分かるまでは広いまま
    expect(after.slice(2).map((c, i) => c - after[i + 1]).every((g) => g === 1000)).toBe(true);
  });

  it("取り直している最中に頼まれたら、終わったあとにもう一度だけ取り直す", async () => {
    let finish!: () => void;
    const calls: string[] = [];
    const r = new Refresher(
      () =>
        new Promise<boolean>((resolve) => {
          calls.push("start");
          finish = () => resolve(true);
        }),
    );
    const first = r.refresh();
    r.refresh();
    r.refresh();
    expect(calls).toEqual(["start"]);
    finish();
    await first;
    await vi.advanceTimersByTimeAsync(0);
    expect(calls).toEqual(["start", "start"]);
  });

  it("reset すると間隔を 1 秒に戻す（タブやパネルを操作したあと）", async () => {
    const { r, calls } = setup(() => false);
    const pump = async (ms: number) => {
      for (let t = 0; t < ms; t += 50) {
        r.outputSeen();
        await vi.advanceTimersByTimeAsync(50);
      }
    };
    await pump(8000); // 250・2250・6250 に取り直し、次は 8 秒後の予定
    expect(calls).toHaveLength(3);
    r.reset();
    await pump(1500);
    expect(calls).toHaveLength(4); // 8 秒待たずに取り直す
  });

  it("取り直している最中に reset したら、その取り直しの結果では間隔を広げない", async () => {
    let finish!: (changed: boolean) => void;
    let calls = 0;
    const r = new Refresher(
      () =>
        new Promise<boolean>((resolve) => {
          calls++;
          finish = resolve;
        }),
    );
    // 変わらない結果を重ねて、間隔を 8 秒まで広げる
    for (let i = 0; i < 3; i++) {
      const p = r.refresh();
      finish(false);
      await p;
    }
    const inFlight = r.refresh();
    r.reset(); // 操作した（間隔は 1 秒に戻るはず）
    finish(false); // 操作の前に始めた取り直しが「変わらない」で終わる
    await inFlight;
    const before = calls;
    for (let t = 0; t < 1500; t += 50) {
      r.outputSeen();
      await vi.advanceTimersByTimeAsync(50);
    }
    expect(calls).toBe(before + 1); // 1 秒あまりで次を取り直す
  });
});
