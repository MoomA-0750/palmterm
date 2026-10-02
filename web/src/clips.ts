// palmterm のクリップボード：tmux のバッファ（tmux や Claude Code でコピーした文字）の一覧。
// ブラウザは押されていないのにクリップボードへ書くことを許さないので、項目ごとの「コピー」で入れてもらう。

import { t } from "./i18n";

export interface ClipItem {
  /** 同じ中身の tmux のバッファ（消すときは全部消す）。 */
  names: string[];
  /** コピーした時刻（Unix 秒）。 */
  created: number;
  text: string;
  truncated?: boolean;
}

async function parseList(res: Response): Promise<ClipItem[]> {
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`);
  return ((await res.json()) as { items: ClipItem[] }).items;
}

export async function fetchClips(): Promise<ClipItem[]> {
  return parseList(await fetch("/api/clipboard", { cache: "no-store" }));
}

/** 項目を消して、消したあとの一覧を返す。 */
export async function deleteClip(item: ClipItem): Promise<ClipItem[]> {
  return parseList(
    await fetch("/api/clipboard/delete", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ names: item.names }),
    }),
  );
}

/** 知らせに出す短い見本（最初の空でない行）。 */
export function clipPreview(text: string, max = 60): string {
  const first = text.split("\n").find((l) => l.trim() !== "")?.trim() ?? "";
  const chars = [...first];
  return chars.length > max ? `${chars.slice(0, max).join("")}…` : first;
}

/** コピーした時刻（今日なら時:分、それより前なら月/日 時:分）と文字数。 */
export function clipMeta(item: ClipItem, now = new Date()): string {
  const at = new Date(item.created * 1000);
  const pad = (n: number) => String(n).padStart(2, "0");
  const time = `${pad(at.getHours())}:${pad(at.getMinutes())}`;
  const sameDay = at.toDateString() === now.toDateString();
  const when = sameDay ? time : `${at.getMonth() + 1}/${at.getDate()} ${time}`;
  const size = [...item.text].length;
  return `${when} · ${size}${item.truncated ? ` ${t("clipTruncated")}` : ""}`;
}

export interface ClipActions {
  copy: (item: ClipItem) => void;
  toLine: (item: ClipItem) => void;
  /** 削除は2回押し（1回目で「消す?」にする）。 */
  remove: (item: ClipItem) => void;
}

const ARM_MS = 3000;

export function renderClips(list: HTMLElement, items: ClipItem[], actions: ClipActions) {
  if (items.length === 0) {
    const empty = document.createElement("li");
    empty.className = "clips-empty";
    empty.textContent = t("clipsEmpty");
    list.replaceChildren(empty);
    return;
  }
  list.replaceChildren(
    ...items.map((item) => {
      const li = document.createElement("li");
      const text = document.createElement("pre");
      text.className = "clip-text";
      text.textContent = item.text;
      const row = document.createElement("div");
      row.className = "clip-actions";
      const meta = document.createElement("span");
      meta.className = "clip-meta";
      meta.textContent = clipMeta(item);
      const button = (label: string, cls: string, fire: (btn: HTMLButtonElement) => void) => {
        const b = document.createElement("button");
        b.type = "button";
        b.className = cls;
        b.textContent = label;
        b.addEventListener("click", () => fire(b));
        return b;
      };
      let armTimer: number | undefined;
      row.append(
        meta,
        button(t("clipDelete"), "clip-delete", (b) => {
          if (b.classList.contains("armed")) {
            window.clearTimeout(armTimer);
            actions.remove(item);
            return;
          }
          b.classList.add("armed");
          b.textContent = t("clipDeleteConfirm");
          armTimer = window.setTimeout(() => {
            b.classList.remove("armed");
            b.textContent = t("clipDelete");
          }, ARM_MS);
        }),
        button(t("clipToLine"), "clip-to-line", () => actions.toLine(item)),
        button(t("clipCopy"), "clip-copy", () => actions.copy(item)),
      );
      li.append(text, row);
      return li;
    }),
  );
}
