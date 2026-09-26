// 入力欄モードの添付欄。選んだ画像をすぐアップロードし、サムネイルと × を並べる。
// 送信のときに、アップロードが終わるのを待ってから、保存したパスを渡す。

import { t } from "./i18n";
import { icon } from "./icons";

export interface UploadedFile {
  path: string; // 端末に入れるパス（HEIC などは変換後の JPEG）
  name: string; // GET /api/upload/{name} で取るときの名前
}

/** 1つのファイルをアップロードする。失敗したらサーバーの説明を付けて投げる。 */
export async function uploadFile(file: File): Promise<UploadedFile> {
  const form = new FormData();
  form.append("file", file, file.name || "image.png");
  const res = await fetch("/api/upload", { method: "POST", body: form });
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`);
  const files: UploadedFile[] = (await res.json()).files;
  if (!files?.[0]) throw new Error(t("noFileInResponse"));
  return files[0];
}

type Status = "uploading" | "done" | "error";

interface Item {
  status: Status;
  uploaded?: UploadedFile;
  error?: string;
  done: Promise<void>;
  el: HTMLElement;
  img: HTMLImageElement;
  objectUrl: string;
  /** ブラウザが元のファイルを表示できなかった（HEIC など）。アップロード後に変換後の JPEG を出す。 */
  needServerPreview: boolean;
}

/** 送る画像1枚。present は、待っている間に × で外されていないか。remove で添付欄から外す。 */
export interface BatchEntry {
  path: string;
  present: () => boolean;
  remove: () => void;
}

export type WaitResult = { ok: true; entries: BatchEntry[] } | { ok: false; failed: number; error: string };

export class Attachments {
  private items: Item[] = [];

  constructor(
    private bar: HTMLElement,
    private onChange: () => void = () => {},
  ) {}

  get count(): number {
    return this.items.length;
  }

  get uploading(): boolean {
    return this.items.some((i) => i.status === "uploading");
  }

  add(files: File[]) {
    for (const file of files) this.addOne(file);
    this.render();
  }

  private addOne(file: File) {
    const el = document.createElement("div");
    el.className = "att";
    const img = document.createElement("img");
    img.alt = file.name;
    const remove = document.createElement("button");
    remove.type = "button";
    remove.className = "att-remove";
    remove.append(icon("close", 14));
    remove.setAttribute("aria-label", t("removeAttachment", { name: file.name }));
    const badge = document.createElement("span");
    badge.className = "att-badge";
    el.append(img, badge, remove);

    const item: Item = {
      status: "uploading",
      done: Promise.resolve(),
      el,
      img,
      objectUrl: URL.createObjectURL(file),
      needServerPreview: false,
    };
    // まずは手元のファイルをそのまま見せる（大きな JPEG をサーバーから取り直さずに済む）。
    img.addEventListener("error", () => {
      if (item.needServerPreview) return;
      item.needServerPreview = true;
      this.showServerPreview(item);
    });
    img.src = item.objectUrl;

    // ボタンを押してもソフトキーボードを閉じない。
    remove.addEventListener("pointerdown", (e) => e.preventDefault());
    remove.addEventListener("click", () => this.remove(item));

    item.done = uploadFile(file).then(
      (uploaded) => {
        item.status = "done";
        item.uploaded = uploaded;
        if (item.needServerPreview) this.showServerPreview(item);
        this.render();
      },
      (e) => {
        item.status = "error";
        item.error = e instanceof Error ? e.message : String(e);
        this.render();
      },
    );
    this.items.push(item);
  }

  private showServerPreview(item: Item) {
    if (item.uploaded) item.img.src = `/api/upload/${encodeURIComponent(item.uploaded.name)}`;
  }

  private remove(item: Item) {
    URL.revokeObjectURL(item.objectUrl);
    this.items = this.items.filter((i) => i !== item);
    this.render();
  }

  /**
   * 今並んでいる画像のアップロードが終わるのを待つ。失敗したものが残っていたら ok: false。
   * 待っている間に足された画像は含めない（次に送る）。送った画像は、呼んだ側が1枚ずつ remove する。
   */
  async waitAll(): Promise<WaitResult> {
    const batch = [...this.items];
    await Promise.all(batch.map((i) => i.done));
    const present = (i: Item) => this.items.includes(i);
    const failed = batch.filter((i) => present(i) && i.status !== "done");
    if (failed.length > 0) return { ok: false, failed: failed.length, error: failed[0].error ?? "" };
    return {
      ok: true,
      entries: batch.filter(present).map((i) => ({
        path: i.uploaded!.path,
        present: () => present(i),
        remove: () => present(i) && this.remove(i),
      })),
    };
  }

  private render() {
    this.bar.hidden = this.items.length === 0;
    for (const item of this.items) {
      item.el.dataset.status = item.status;
      const badge = item.el.querySelector(".att-badge")!;
      badge.textContent = item.status === "uploading" ? "…" : item.status === "error" ? "!" : "";
      item.el.title = item.status === "error" ? t("attachmentError", { error: item.error ?? "" }) : (item.uploaded?.path ?? "");
    }
    this.bar.replaceChildren(...this.items.map((i) => i.el));
    this.onChange();
  }
}
