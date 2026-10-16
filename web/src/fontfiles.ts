// 画面から足したフォント（PC の設定ファイルのフォルダの fonts/）の一覧。追加と削除（2回押し）ができる。
// 各行の「使う」で、すぐ端末の文字に使い、設定ファイルの [font] に file = "fonts/名前" と書き戻す
// （手で書いても使える）。先頭の「既定のフォント」の「使う」は file を消す。

import { t } from "./i18n";

export interface FontFile {
  name: string;
  size: number;
}

export interface FontList {
  fonts: FontFile[];
  /** 保存先のフォルダ（PC のパス）。 */
  dir: string;
  /** [font] の file にいま書いてあるもの。 */
  file: string;
}

/** 選べるファイル（input の accept）。サーバーは中身の先頭でも確かめる。 */
export const FONT_ACCEPT = ".ttf,.otf,.woff,.woff2";

async function check(res: Response): Promise<Response> {
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`);
  return res;
}

export async function fetchFonts(): Promise<FontList> {
  return (await check(await fetch("/api/fonts", { cache: "no-store" }))).json();
}

/** フォントを1つ送って保存する。保存した名前（同じ名前があれば番号付き）を返す。 */
export async function uploadFont(file: File): Promise<string> {
  const form = new FormData();
  form.append("file", file);
  const res = await check(await fetch("/api/fonts", { method: "POST", body: form }));
  return ((await res.json()) as { name: string }).name;
}

export async function deleteFont(name: string): Promise<void> {
  await check(
    await fetch("/api/fonts/delete", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    }),
  );
}

/** 設定ファイルに書く1行。 */
export function fontConfigLine(name: string): string {
  return `file = "fonts/${name}"`;
}

export function fontSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

const ARM_MS = 3000;

function useButton(name: string, use: () => void): HTMLButtonElement {
  const b = document.createElement("button");
  b.type = "button";
  b.className = "font-use";
  b.textContent = t("fontUse");
  b.title = t("fontUseTitle", { name });
  b.setAttribute("aria-label", b.title);
  b.addEventListener("click", use);
  return b;
}

/** use は「使う」を押したとき（null は既定のフォントに戻す）。 */
export function renderFonts(
  list: HTMLElement,
  data: FontList,
  remove: (font: FontFile) => void,
  use: (font: FontFile | null) => void = () => {},
) {
  // 既定のフォント（[font] の file を使わない）に戻す行。
  const def = document.createElement("li");
  const defInUse = data.file === "";
  def.className = defInUse ? "font-default in-use" : "font-default";
  const defInfo = document.createElement("div");
  defInfo.className = "font-info";
  const defName = document.createElement("div");
  defName.className = "font-name";
  defName.textContent = t("fontDefault") + (defInUse ? `（${t("fontInUse")}）` : "");
  const defMeta = document.createElement("div");
  defMeta.className = "font-meta";
  defMeta.textContent = t("fontDefaultMeta");
  defInfo.append(defName, defMeta);
  def.append(defInfo);
  if (!defInUse) def.append(useButton(t("fontDefault"), () => use(null)));

  if (data.fonts.length === 0) {
    const empty = document.createElement("li");
    empty.className = "fonts-empty";
    empty.textContent = t("fontsEmpty");
    // file を書いているときだけ、既定に戻す行も出す
    list.replaceChildren(...(defInUse ? [] : [def]), empty);
    return;
  }

  list.replaceChildren(
    def,
    ...data.fonts.map((font) => {
      const li = document.createElement("li");
      const inUse = data.file === `fonts/${font.name}`;
      li.className = inUse ? "font-file in-use" : "font-file";
      const info = document.createElement("div");
      info.className = "font-info";
      const name = document.createElement("div");
      name.className = "font-name";
      name.textContent = font.name + (inUse ? `（${t("fontInUse")}）` : "");
      const meta = document.createElement("div");
      meta.className = "font-meta";
      meta.textContent = `${fontSize(font.size)} · ${fontConfigLine(font.name)}`;
      info.append(name, meta);

      const b = document.createElement("button");
      b.type = "button";
      b.className = "font-delete";
      b.textContent = t("clipDelete");
      let armTimer: number | undefined;
      b.addEventListener("click", () => {
        if (b.classList.contains("armed")) {
          window.clearTimeout(armTimer);
          remove(font);
          return;
        }
        b.classList.add("armed");
        b.textContent = t("clipDeleteConfirm");
        armTimer = window.setTimeout(() => {
          b.classList.remove("armed");
          b.textContent = t("clipDelete");
        }, ARM_MS);
      });
      if (inUse) li.append(info, b);
      else li.append(info, useButton(font.name, () => use(font)), b);
      return li;
    }),
  );
}

/**
 * フォントの画面（index.html には置かず、ここで作って body に足す）。開くたびに一覧を取り直す。
 * 結果（保存した・断られた理由）は toast で知らせる。choose はフォントを使う（空は既定に戻す）。書き戻せたら true。
 */
export function setupFonts(toast: (message: string) => void, choose: (name: string) => Promise<boolean>) {
  const el = document.createElement("div");
  el.id = "fonts";
  el.hidden = true;
  const bar = document.createElement("div");
  bar.className = "copy-bar";
  const close = document.createElement("button");
  close.type = "button";
  close.id = "fonts-close";
  close.textContent = t("copyClose");
  const title = document.createElement("span");
  title.id = "fonts-title";
  title.textContent = t("fonts");
  const spacer = document.createElement("span");
  spacer.className = "spacer";
  const add = document.createElement("button");
  add.type = "button";
  add.id = "fonts-add";
  add.textContent = t("fontAdd");
  const input = document.createElement("input");
  input.type = "file";
  input.id = "fonts-input";
  input.accept = FONT_ACCEPT;
  input.hidden = true;
  bar.append(close, title, spacer, add, input);
  const hint = document.createElement("p");
  hint.id = "fonts-hint";
  const list = document.createElement("ul");
  list.id = "font-list";
  el.append(bar, hint, list);
  document.body.append(el);

  const error = (e: unknown) => (e instanceof Error ? e.message : String(e));

  async function refresh() {
    if (list.childElementCount === 0) list.textContent = t("loading");
    try {
      const data = await fetchFonts();
      hint.textContent = t("fontsHint", { dir: data.dir });
      renderFonts(list, data, remove, use);
    } catch (e) {
      list.textContent = t("fontsError", { error: error(e) });
    }
  }

  async function remove(font: FontFile) {
    try {
      await deleteFont(font.name);
    } catch (e) {
      toast(t("fontsError", { error: error(e) }));
    }
    await refresh();
  }

  async function use(font: FontFile | null) {
    if (await choose(font?.name ?? "")) toast(t("fontChosen", { name: font?.name ?? t("fontDefault") }));
    await refresh();
  }

  add.addEventListener("click", () => input.click());
  input.addEventListener("change", async () => {
    const file = input.files?.[0];
    input.value = "";
    if (!file) return;
    toast(t("fontUploading", { name: file.name }));
    try {
      const name = await uploadFont(file);
      toast(t("fontSaved", { line: fontConfigLine(name) }));
    } catch (e) {
      toast(t("fontUploadFailed", { error: error(e) }));
    }
    await refresh();
  });
  close.addEventListener("click", () => (el.hidden = true));

  return {
    open() {
      el.hidden = false;
      void refresh();
    },
  };
}
