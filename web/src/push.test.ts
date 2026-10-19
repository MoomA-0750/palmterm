import { describe, expect, it } from "vitest";
import { type PushEnv, pushBlocker, windowFromURL } from "./push";

const ok: PushEnv = { secure: true, serviceWorker: true, pushManager: true, notification: true, ios: false, standalone: false, permission: "default" };

describe("通知を使えるか", () => {
  it("HTTPS で Service Worker とプッシュに対応していれば使える（許可はまだでも、押せば求める）", () => {
    expect(pushBlocker(ok)).toBeNull();
    expect(pushBlocker({ ...ok, permission: "granted" })).toBeNull();
  });

  it("iPhone・iPad では、ホーム画面から開いたときだけ使える（Safari のタブでは理由を出す）", () => {
    // iOS の Safari のタブには PushManager が無いが、出す理由は「ホーム画面から」
    expect(pushBlocker({ ...ok, ios: true, pushManager: false, notification: false })).toBe("needsHomeScreen");
    expect(pushBlocker({ ...ok, ios: true, standalone: true })).toBeNull();
    // ホーム画面から開いても、古い iOS（16.4 より前）は対応していない
    expect(pushBlocker({ ...ok, ios: true, standalone: true, pushManager: false })).toBe("unsupported");
  });

  it("HTTPS でない・対応していない・許可されていないときは、その理由", () => {
    expect(pushBlocker({ ...ok, secure: false })).toBe("insecure");
    expect(pushBlocker({ ...ok, serviceWorker: false })).toBe("unsupported");
    expect(pushBlocker({ ...ok, permission: "denied" })).toBe("denied");
  });
});

describe("通知を押して開いたときのウィンドウ", () => {
  it("?window=1&windowId=@3 からウィンドウを取り出す。ID の形でなければ番号だけ、番号が無ければ何もしない", () => {
    expect(windowFromURL("?window=1&windowId=%403")).toEqual({ index: 1, id: "@3" });
    expect(windowFromURL("?window=2&windowId=x")).toEqual({ index: 2, id: undefined });
    expect(windowFromURL("")).toBeNull();
    expect(windowFromURL("?window=-1")).toBeNull();
  });
});
