// フォントの一覧。組み込みの JetBrains Mono、画面から足したフォント（PC の設定ファイルのフォルダの fonts/）、
// PC に入っているフォント（名前ごとに1行）を、出どころの印を付けて並べ、名前で絞り込める。足したフォントは追加と削除（2回押し）ができる。
// 各行の「使う」で、すぐ端末の文字に使い、設定ファイルの [font] に書き戻す（足したフォントは file = "fonts/名前"、
// stack の書き方なら並びをそれ1つ。PC と組み込みのフォントは stack = ["名前"]）。先頭の「既定のフォント」の「使う」は file（stack）を消す。
// 一覧の上の「使う順」の欄は [font] の stack の並びで、各行の「足す」で欄の最後に入れ、欄の中で順を変えたり外したりできる。
// 欄を変えるたびに stack をその順に書き戻し、すぐ端末の文字に当てる（空にすると stack の行を消して既定に戻す）。

import { t } from "./i18n";

export interface FontFile {
  name: string;
  size: number;
}

export interface FontList {
  fonts: FontFile[];
  /** 保存先のフォルダ（PC のパス）。 */
  dir: string;
  /** [font] の file にいま書いてあるもの（stack なら並びの先頭）。 */
  file: string;
  /** PC に入っているフォントの名前（ファイルのパスはサーバーが出さない）。 */
  system?: string[];
  /** 組み込みのフォントの名前（JetBrains Mono）。 */
  builtin?: string;
  /** [font] の stack の並び（/api/config から。版 1 の書き方なら無い）。 */
  stack?: string[];
}

/** 一覧の行の出どころ（印に出す）。config は設定ファイルに手で書いたもの（一覧に無いパスやスマホのフォントの名前）。 */
export type FontSource = "added" | "pc" | "builtin" | "config";

/** 選べるファイル（input の accept）。サーバーは中身の先頭でも確かめる。 */
export const FONT_ACCEPT = ".ttf,.otf,.woff,.woff2";

async function check(res: Response): Promise<Response> {
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`);
  return res;
}

export async function fetchFonts(): Promise<FontList> {
  const [list, config] = await Promise.all([
    check(await fetch("/api/fonts", { cache: "no-store" })).then((r) => r.json() as Promise<FontList>),
    fetch("/api/config", { cache: "no-store" })
      .then((r) => (r.ok ? (r.json() as Promise<{ font?: { stack?: string[] } }>) : null))
      .catch(() => null),
  ]);
  return { ...list, stack: config?.font?.stack ?? [] };
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

const SOURCE_TEXT = {
  added: "fontSourceAdded",
  pc: "fontSourcePC",
  builtin: "fontSourceBuiltin",
  config: "fontSourceConfig",
} as const;

function sourceBadge(source: FontSource): HTMLElement {
  const b = document.createElement("span");
  b.className = `font-source font-source-${source}`;
  b.textContent = t(SOURCE_TEXT[source]);
  return b;
}

const same = (a: string, b: string) => a.toLowerCase() === b.toLowerCase();

/** stack の1つの出どころ。 */
export function stackSource(entry: string, data: FontList): FontSource {
  if (entry.startsWith("fonts/")) return "added";
  if (data.builtin && same(entry, data.builtin)) return "builtin";
  if (data.system?.some((name) => same(name, entry))) return "pc";
  return "config";
}

function addButton(name: string, entry: string, stack: string[] | undefined, add: (entry: string) => void): HTMLButtonElement[] {
  if (stack?.some((s) => same(s, entry))) return [];
  const b = document.createElement("button");
  b.type = "button";
  b.className = "font-add";
  b.textContent = t("fontStackAdd");
  b.title = t("fontStackAddTitle", { name });
  b.setAttribute("aria-label", b.title);
  b.addEventListener("click", () => add(entry));
  return [b];
}

function smallButton(cls: string, text: string, title: string, disabled: boolean, click: () => void): HTMLButtonElement {
  const b = document.createElement("button");
  b.type = "button";
  b.className = cls;
  b.textContent = text;
  b.title = title;
  b.setAttribute("aria-label", title);
  b.disabled = disabled;
  b.addEventListener("click", click);
  return b;
}

/** 「使う順」の欄。上下のボタンで順を変え、「外す」で抜く。change に新しい並びを渡す。 */
export function renderStack(list: HTMLElement, data: FontList, change: (stack: string[]) => void) {
  const stack = data.stack ?? [];
  if (stack.length === 0) {
    const empty = document.createElement("li");
    empty.className = "font-stack-empty";
    empty.textContent = t("fontStackEmpty");
    list.replaceChildren(empty);
    return;
  }
  const moved = (i: number, j: number) => {
    const next = [...stack];
    [next[i], next[j]] = [next[j], next[i]];
    change(next);
  };
  list.replaceChildren(
    ...stack.map((entry, i) => {
      const li = document.createElement("li");
      li.className = "font-stack-item";
      li.dataset.entry = entry;
      const label = entry.startsWith("fonts/") ? entry.slice("fonts/".length) : entry;
      const info = document.createElement("div");
      info.className = "font-head";
      const name = document.createElement("div");
      name.className = "font-name";
      name.textContent = label;
      info.append(sourceBadge(stackSource(entry, data)), name);
      li.append(
        info,
        smallButton("font-up", "↑", t("fontStackUp", { name: label }), i === 0, () => moved(i, i - 1)),
        smallButton("font-down", "↓", t("fontStackDown", { name: label }), i === stack.length - 1, () => moved(i, i + 1)),
        smallButton("font-remove", t("fontStackRemove"), t("fontStackRemoveTitle", { name: label }), false, () =>
          change(stack.filter((_, j) => j !== i)),
        ),
      );
      return li;
    }),
  );
}

/** 名前と出どころの印、その下の説明。 */
function fontInfo(source: FontSource, label: string, inUse: boolean, meta: string): HTMLElement {
  const info = document.createElement("div");
  info.className = "font-info";
  const head = document.createElement("div");
  head.className = "font-head";
  const name = document.createElement("div");
  name.className = "font-name";
  name.textContent = label + (inUse ? `（${t("fontInUse")}）` : "");
  head.append(sourceBadge(source), name);
  const m = document.createElement("div");
  m.className = "font-meta";
  m.textContent = meta;
  info.append(head, m);
  return info;
}

/** PC か組み込みのフォントの行。「使う」で stack をその1つにし、「足す」で「使う順」の最後に入れる。 */
function namedFontRow(
  source: "pc" | "builtin",
  name: string,
  data: FontList,
  useName: (name: string) => void,
  add: (entry: string) => void,
): HTMLLIElement {
  const li = document.createElement("li");
  const inUse = same(data.file, name);
  li.className = `font-${source === "pc" ? "system" : "builtin"}${inUse ? " in-use" : ""}`;
  li.dataset.name = name;
  li.append(fontInfo(source, name, inUse, `stack = ${JSON.stringify([name])}`));
  li.append(...addButton(name, name, data.stack, add));
  if (!inUse) li.append(useButton(name, () => useName(name)));
  return li;
}

/**
 * use は足したフォントの「使う」を押したとき（null は既定のフォントに戻す）。useName は PC か組み込みのフォントの「使う」で、
 * その名前を渡す。add は「足す」で、stack に書く形（足したフォントは fonts/名前）を渡す。
 */
export function renderFonts(
  list: HTMLElement,
  data: FontList,
  remove: (font: FontFile) => void,
  use: (font: FontFile | null) => void = () => {},
  useName: (name: string) => void = () => {},
  add: (entry: string) => void = () => {},
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

  const builtin = data.builtin ? [namedFontRow("builtin", data.builtin, data, useName, add)] : [];
  const system = (data.system ?? []).map((name) => namedFontRow("pc", name, data, useName, add));

  if (data.fonts.length === 0) {
    const empty = document.createElement("li");
    empty.className = "fonts-empty";
    empty.textContent = t("fontsEmpty");
    list.replaceChildren(def, ...builtin, empty, ...system);
    return;
  }

  list.replaceChildren(
    def,
    ...builtin,
    ...data.fonts.map((font) => {
      const li = document.createElement("li");
      const inUse = data.file === `fonts/${font.name}`;
      li.className = inUse ? "font-file in-use" : "font-file";
      li.dataset.name = font.name;
      const info = fontInfo("added", font.name, inUse, `${fontSize(font.size)} · ${fontConfigLine(font.name)}`);

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
      li.append(info, ...addButton(font.name, `fonts/${font.name}`, data.stack, add));
      if (!inUse) li.append(useButton(font.name, () => use(font)));
      li.append(b);
      return li;
    }),
    ...system,
  );
}

/** 名前に query を含む行だけを出す（大文字小文字は問わない。空なら全部）。既定のフォントの行と案内はいつも出す。 */
export function filterFonts(list: HTMLElement, query: string) {
  const q = query.trim().toLowerCase();
  for (const li of list.querySelectorAll<HTMLElement>("li[data-name]")) {
    li.hidden = q !== "" && !li.dataset.name!.toLowerCase().includes(q);
  }
}

/**
 * フォントの画面（index.html には置かず、ここで作って body に足す）。開くたびに一覧を取り直す。
 * 結果（保存した・断られた理由）は toast で知らせる。choose はフォントを使う（font は足したフォントの名前で、空は既定に戻す。
 * systemFont は PC か組み込みのフォントの名前）。書き戻せたら true。
 */
export function setupFonts(
  toast: (message: string) => void,
  choose: (change: { font: string } | { systemFont: string } | { stack: string[] }) => Promise<boolean>,
) {
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
  const filter = document.createElement("input");
  filter.type = "search";
  filter.id = "fonts-filter";
  filter.placeholder = t("fontsFilter");
  filter.setAttribute("aria-label", t("fontsFilter"));
  filter.autocomplete = "off";
  filter.spellcheck = false;
  const stackBox = document.createElement("section");
  stackBox.id = "font-stack";
  const stackTitle = document.createElement("div");
  stackTitle.id = "font-stack-title";
  stackTitle.textContent = t("fontStack");
  const stackList = document.createElement("ol");
  stackList.id = "font-stack-list";
  stackBox.append(stackTitle, stackList);
  const list = document.createElement("ul");
  list.id = "font-list";
  el.append(bar, hint, stackBox, filter, list);
  document.body.append(el);

  const error = (e: unknown) => (e instanceof Error ? e.message : String(e));

  async function refresh() {
    if (list.childElementCount === 0) list.textContent = t("loading");
    try {
      const data = await fetchFonts();
      hint.textContent = t("fontsHint", { dir: data.dir });
      renderStack(stackList, data, setStack);
      renderFonts(list, data, remove, use, useName, (entry) => setStack([...(data.stack ?? []), entry]));
      filterFonts(list, filter.value);
    } catch (e) {
      list.textContent = t("fontsError", { error: error(e) });
    }
  }

  /** 「使う順」を書き戻してすぐ当てる。書き戻すあいだは欄のボタンを押せなくする（続けて押して並びが食い違わないように）。 */
  async function setStack(stack: string[]) {
    for (const b of el.querySelectorAll<HTMLButtonElement>("#font-stack button, .font-add")) b.disabled = true;
    await choose({ stack });
    await refresh();
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
    if (await choose({ font: font?.name ?? "" })) toast(t("fontChosen", { name: font?.name ?? t("fontDefault") }));
    await refresh();
  }

  async function useName(name: string) {
    if (await choose({ systemFont: name })) toast(t("fontChosen", { name }));
    await refresh();
  }

  filter.addEventListener("input", () => filterFonts(list, filter.value));
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
