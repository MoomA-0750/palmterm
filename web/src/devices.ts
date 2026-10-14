// ログインしている端末（ブラウザ）の一覧。自分はログアウト、ほかの端末は取り消せる（どちらも2回押し）。
// なくしたスマホなどの分を、手元の端末から締め出すためのもの。

import { t } from "./i18n";

export interface DeviceItem {
  id: string;
  /** User-Agent から作った名前（例「iPhone · Safari」）。 */
  name: string;
  /** 本人が付けた名前。 */
  label?: string;
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

/** 端末に名前を付ける。空なら付けた名前を消す（自動の名前に戻る）。 */
export async function renameDevice(id: string, name: string): Promise<void> {
  const res = await fetch("/api/devices/rename", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id, name }),
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

export interface DeviceActions {
  /** ログアウト・取り消し（2回押し）。 */
  revoke: (item: DeviceItem) => void;
  /** 名前を付ける（空なら自動の名前に戻す）。 */
  rename: (item: DeviceItem, name: string) => void;
}

export function renderDevices(list: HTMLElement, items: DeviceItem[], actions: DeviceActions) {
  list.replaceChildren(
    ...items.map((item) => {
      const li = document.createElement("li");
      li.className = item.current ? "device current" : "device";
      const info = document.createElement("div");
      info.className = "device-info";
      // 名前を押すと入力欄にして名前を付けられる。付けた名前があれば、自動の名前は下の行に添える。
      const name = document.createElement("button");
      name.type = "button";
      name.className = "device-name";
      name.textContent = (item.label || item.name) + (item.current ? `（${t("thisDevice")}）` : "");
      name.title = t("renameDevice");
      name.addEventListener("click", () => startRename(item, name, actions));
      const meta = document.createElement("div");
      meta.className = "device-meta";
      const times = t("deviceMeta", { created: deviceTime(item.created), lastSeen: deviceTime(item.lastSeen) });
      meta.textContent = item.label ? `${item.name} · ${times}` : times;
      info.append(name, meta);

      const label = item.current ? t("logout") : t("revoke");
      const b = document.createElement("button");
      b.type = "button";
      b.className = "device-revoke";
      b.textContent = label;
      let armTimer: number | undefined;
      b.addEventListener("click", () => {
        if (b.classList.contains("armed")) {
          window.clearTimeout(armTimer);
          actions.revoke(item);
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

/** 名前の所を入力欄にする。Enter か外をタップで決定、Esc でやめる。 */
function startRename(item: DeviceItem, nameBtn: HTMLButtonElement, actions: DeviceActions) {
  const input = document.createElement("input");
  input.className = "device-name-input";
  input.value = item.label ?? "";
  input.placeholder = item.name;
  input.maxLength = 40;
  input.setAttribute("aria-label", t("renameDevice"));
  input.enterKeyHint = "done";
  input.autocapitalize = "off";
  input.autocomplete = "off";
  input.spellcheck = false;
  let finished = false;
  const finish = (commit: boolean) => {
    if (finished) return;
    finished = true;
    const value = input.value.trim();
    input.replaceWith(nameBtn);
    if (commit && value !== (item.label ?? "")) actions.rename(item, value);
  };
  input.addEventListener("keydown", (e) => {
    if (e.isComposing) return;
    if (e.key === "Enter") {
      e.preventDefault();
      finish(true);
    } else if (e.key === "Escape") {
      e.preventDefault();
      finish(false);
    }
  });
  input.addEventListener("blur", () => finish(true));
  nameBtn.replaceWith(input);
  input.focus();
  input.select();
}

