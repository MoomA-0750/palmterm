// ログインしている端末（ブラウザ）の一覧。自分はログアウト、ほかの端末は取り消せる（どちらも2回押し）。
// なくしたスマホなどの分を、手元の端末から締め出すためのもの。

import { t } from "./i18n";

export interface DeviceItem {
  id: string;
  name: string;
  created: string;
  lastSeen: string;
  /** この画面を開いている端末。 */
  current: boolean;
}

export async function fetchDevices(): Promise<DeviceItem[]> {
  const res = await fetch("/api/devices", { cache: "no-store" });
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`);
  return ((await res.json()) as { devices: DeviceItem[] }).devices;
}

/** 端末のログインを取り消す。id を省くとこの端末（ログアウト）。 */
export async function revokeDevice(id?: string): Promise<void> {
  const res = await fetch("/api/devices/revoke", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(id ? { id } : {}),
  });
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`);
}

/** 時刻を短く（今日なら時:分、それより前なら年/月/日）。 */
export function deviceTime(iso: string, now = new Date()): string {
  const at = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  if (at.toDateString() === now.toDateString()) return `${pad(at.getHours())}:${pad(at.getMinutes())}`;
  return `${at.getFullYear()}/${at.getMonth() + 1}/${at.getDate()}`;
}

const ARM_MS = 3000;

export function renderDevices(list: HTMLElement, items: DeviceItem[], revoke: (item: DeviceItem) => void) {
  list.replaceChildren(
    ...items.map((item) => {
      const li = document.createElement("li");
      li.className = item.current ? "device current" : "device";
      const info = document.createElement("div");
      info.className = "device-info";
      const name = document.createElement("div");
      name.className = "device-name";
      name.textContent = item.current ? `${item.name}（${t("thisDevice")}）` : item.name;
      const meta = document.createElement("div");
      meta.className = "device-meta";
      meta.textContent = t("deviceMeta", { created: deviceTime(item.created), lastSeen: deviceTime(item.lastSeen) });
      info.append(name, meta);

      const label = item.current ? t("logout") : t("revoke");
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = label;
      let armTimer: number | undefined;
      b.addEventListener("click", () => {
        if (b.classList.contains("armed")) {
          window.clearTimeout(armTimer);
          revoke(item);
          return;
        }
        b.classList.add("armed");
        b.textContent = item.current ? t("logoutConfirm") : t("revokeConfirm");
        armTimer = window.setTimeout(() => {
          b.classList.remove("armed");
          b.textContent = label;
        }, ARM_MS);
      });
      li.append(info, b);
      return li;
    }),
  );
}
