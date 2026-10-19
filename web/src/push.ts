// 通知（Web Push）。設定画面の「通知」で、この端末に palmterm notify の知らせを OS の通知で届けるようにする。
// ブラウザのプッシュの宛先（購読）を作ってサーバーに渡す（POST /api/push/subscribe）。通知を出すのは
// Service Worker（public/sw.js）。iPhone・iPad では、ホーム画面に追加した palmterm から開いたときだけ使える。
// 画面が見えている端末には、サーバーはプッシュを送らず帯だけを出すので、見えている・隠れたをサーバーに知らせる。

export type PushState = "on" | "off" | "denied" | "needsHomeScreen" | "insecure" | "unsupported" | "error";

export interface PushStatus {
  state: PushState;
  /** state が error のときの理由。 */
  error?: string;
}

/** 通知を使えるかを決める、端末とブラウザの様子。 */
export interface PushEnv {
  secure: boolean;
  serviceWorker: boolean;
  pushManager: boolean;
  notification: boolean;
  /** iPhone・iPad（iPadOS の Safari は Mac と名乗るので、触れる Mac も含める）。 */
  ios: boolean;
  /** ホーム画面から開いた（display-mode: standalone）。 */
  standalone: boolean;
  permission: NotificationPermission | "unsupported";
}

/** 通知を使えない理由（使えるなら null）。iPhone・iPad でホーム画面から開いていなければ、まずそれを言う。 */
export function pushBlocker(env: PushEnv): PushState | null {
  if (env.ios && !env.standalone) return "needsHomeScreen";
  if (!env.secure) return "insecure";
  if (!env.serviceWorker || !env.pushManager || !env.notification) return "unsupported";
  if (env.permission === "denied") return "denied";
  return null;
}

export function currentEnv(): PushEnv {
  const ua = navigator.userAgent;
  const ios = /iPad|iPhone|iPod/.test(ua) || (/Macintosh/.test(ua) && navigator.maxTouchPoints > 1);
  const standalone =
    (window.matchMedia?.("(display-mode: standalone)").matches ?? false) ||
    (navigator as Navigator & { standalone?: boolean }).standalone === true;
  return {
    secure: window.isSecureContext,
    serviceWorker: "serviceWorker" in navigator,
    pushManager: "PushManager" in window,
    notification: "Notification" in window,
    ios,
    standalone,
    permission: "Notification" in window ? Notification.permission : "unsupported",
  };
}

function b64urlToBytes(s: string): Uint8Array<ArrayBuffer> {
  const b = atob(s.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(s.length / 4) * 4, "="));
  const out = new Uint8Array(b.length);
  for (let i = 0; i < b.length; i++) out[i] = b.charCodeAt(i);
  return out;
}

function bytesToB64url(b: Uint8Array): string {
  return btoa(String.fromCharCode(...b)).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

async function serverState(): Promise<{ publicKey: string; subscribed: boolean }> {
  const res = await fetch("/api/push", { cache: "no-store" });
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`);
  return res.json();
}

async function post(path: string, body?: unknown, keepalive = false) {
  const res = await fetch(path, {
    method: "POST",
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
    keepalive,
  });
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`);
}

const message = (e: unknown) => (e instanceof Error ? e.message : String(e));

/** 見えていると知らせ直す間隔（サーバーは 3 分たつと見えていないとみなす）。 */
const PRESENCE_MS = 60_000;

export function setupPush(env: () => PushEnv = currentEnv) {
  const usable = () => pushBlocker(env()) === null;
  const registration = () => navigator.serviceWorker.register("/sw.js");

  /** 今の様子：使えない理由か、この端末で有効か。 */
  async function state(): Promise<PushStatus> {
    const blocker = pushBlocker(env());
    if (blocker) return { state: blocker };
    try {
      const [server, reg] = await Promise.all([serverState(), navigator.serviceWorker.getRegistration("/")]);
      const sub = await reg?.pushManager.getSubscription();
      // サーバーの鍵（vapid.pem）を作り直していたら、前の宛先にはもう送れないので切れているとみなす。
      const key = sub?.options.applicationServerKey;
      const sameKey = !key || bytesToB64url(new Uint8Array(key)) === server.publicKey;
      return { state: server.subscribed && sub && sameKey && Notification.permission === "granted" ? "on" : "off" };
    } catch (e) {
      return { state: "error", error: message(e) };
    }
  }

  /** 通知の許可を求め、許されたら宛先を作ってサーバーに渡す。押したその場で呼ぶ（iOS は許可を求めるのに操作が要る）。 */
  async function enable(): Promise<PushStatus> {
    const blocker = pushBlocker(env());
    if (blocker) return { state: blocker };
    try {
      const permission = await Notification.requestPermission();
      if (permission === "denied") return { state: "denied" };
      if (permission !== "granted") return { state: "off" };
      const [server, reg] = await Promise.all([serverState(), registration()]);
      await navigator.serviceWorker.ready;
      // 前の宛先は、サーバーの鍵が変わっていると使えないので作り直す。
      await (await reg.pushManager.getSubscription())?.unsubscribe();
      const sub = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: b64urlToBytes(server.publicKey) });
      await post("/api/push/subscribe", sub.toJSON());
      return { state: "on" };
    } catch (e) {
      return { state: "error", error: message(e) };
    }
  }

  /** この端末では通知を受けない（サーバーの宛先を消し、ブラウザの宛先も消す）。 */
  async function disable(): Promise<PushStatus> {
    try {
      await post("/api/push/unsubscribe");
      const reg = await navigator.serviceWorker.getRegistration("/");
      await (await reg?.pushManager.getSubscription())?.unsubscribe();
      return { state: "off" };
    } catch (e) {
      return { state: "error", error: message(e) };
    }
  }

  // ---- 見えている・隠れたをサーバーに知らせる ----
  const page = crypto.randomUUID?.() ?? `${Date.now()}-${Math.random()}`;
  const visible = () => document.visibilityState === "visible";
  function reportPresence(shown = visible()) {
    post("/api/push/presence", { page, visible: shown }, true).catch(() => {});
  }

  /** 見えている・隠れたを知らせ始め、Service Worker を入れておく（通知を押したときに開けるように）。 */
  function start() {
    reportPresence();
    document.addEventListener("visibilitychange", () => reportPresence());
    window.addEventListener("pagehide", () => reportPresence(false));
    window.addEventListener("pageshow", () => reportPresence());
    setInterval(() => visible() && reportPresence(true), PRESENCE_MS);
    if (usable()) registration().catch(() => {});
  }

  return { state, enable, disable, start, reportPresence };
}

/** 通知を押して開いたときの URL（?window=1&windowId=@3）から、切り替えるウィンドウを取り出す。 */
export function windowFromURL(search: string): { index: number; id?: string } | null {
  const q = new URLSearchParams(search);
  const index = Number.parseInt(q.get("window") ?? "", 10);
  if (!Number.isInteger(index) || index < 0) return null;
  const id = q.get("windowId") ?? undefined;
  return { index, id: id && /^@\d+$/.test(id) ? id : undefined };
}
