// palmterm の Service Worker。palmterm notify の知らせをプッシュ（Web Push）で受けて OS の通知に出し、
// 通知を押すと palmterm を開いて、打ったウィンドウに切り替えさせる。ページの読み込みには関わらない（fetch は扱わない）。
// 中身はサーバーの push.go の pushPayload：{ title, body, window?, windowId? }。

self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (event) => event.waitUntil(self.clients.claim()));

self.addEventListener("push", (event) => {
  let data = {};
  try {
    data = event.data ? event.data.json() : {};
  } catch {
    data = { body: event.data ? event.data.text() : "" };
  }
  const title = data.title || "palmterm";
  event.waitUntil(
    self.registration.showNotification(title, {
      body: data.body || "",
      icon: "/icons/icon-192.png",
      // Android は badge の形（不透明な所）だけを白く描く。背景のある画像だと白い四角になるので、透明な背景に >_ だけを描いた画像
      badge: "/icons/badge-96.png",
      // 同じウィンドウの知らせは前のものに重ねず並べる（tag を付けない）
      data: { window: data.window, windowId: data.windowId },
    }),
  );
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const { window: index, windowId } = event.notification.data || {};
  const hasWindow = typeof index === "number";
  event.waitUntil(
    (async () => {
      const list = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
      const client = list.find((c) => new URL(c.url).origin === self.location.origin);
      if (client) {
        // 開いている palmterm を前に出し、そのウィンドウに切り替えさせる
        if (hasWindow) client.postMessage({ type: "palmterm-select-window", window: index, windowId });
        await client.focus();
        return;
      }
      const url = new URL("/", self.location.origin);
      if (hasWindow) {
        url.searchParams.set("window", String(index));
        if (windowId) url.searchParams.set("windowId", windowId);
      }
      await self.clients.openWindow(url.href);
    })(),
  );
});
