// 端末のフォント（設定ファイルの [font]）。サーバーが [font] から決めた並び（fontFaces）を、先頭から順に使う。
// PC から配られたフォントはその URL で読み、PC に無いものは同じ名前でスマホに入っているフォントを探す。
// 並びのどれも持たない文字は、今までどおり JetBrains Mono などで描く。
// Nerd Font のアイコンは、どのフォントにしても palmterm が配る Nerd Symbols で描く（fonts.css）。

/** 設定ファイルの [font]（サーバーはそのまま渡す）。stack は版 2 の書き方で、family・file は版 1 の書き方。 */
export interface FontConfig {
  family?: string;
  file?: string;
  stack?: string[];
}

/** サーバーが [font] から決めたフォントの並びの1つ。url は PC から配るもの、family はスマホ（か組み込み）のフォントの名前。 */
export interface FontEntry {
  url?: string;
  family?: string;
}

/** PC から配られたフォントに付ける名前（ページの中だけで使う。後ろに並びの番号を付ける）。 */
export const SERVED_FONT = "palmterm font";

/** 既定の並び（fonts.css の --mono-font）のうち、JetBrains Mono から後ろ。 */
const FALLBACK = `"JetBrains Mono", "Noto Sans Mono CJK JP", "Noto Sans CJK JP", "Hiragino Sans", ui-monospace, monospace`;

function cssString(s: string): string {
  return `"${s.replace(/[\\"]/g, (c) => `\\${c}`).replace(/[\n\r\f]/g, " ")}"`;
}

function servedName(i: number): string {
  return `${SERVED_FONT} ${i + 1}`;
}

/**
 * 端末の font-family。並びが空なら null（fonts.css の既定のまま）。
 * 並びの順に、配られたフォント（あれば）とその名前を置く。ブラウザは文字ごとに、先頭から順にその文字を持つフォントで描く。
 * Nerd Symbols は私用領域の文字だけを受け持つので先頭に置き、選んだフォントがアイコンを持っていても
 * 今までどおりの形で描く。
 */
export function fontFamily(faces: FontEntry[] | undefined): string | null {
  const names: string[] = [];
  faces?.forEach((f, i) => {
    if (f.url) names.push(cssString(servedName(i)));
    const family = f.family?.trim();
    if (family) names.push(cssString(family));
  });
  if (names.length === 0) return null;
  return [`"Nerd Symbols"`, ...names, FALLBACK].join(", ");
}

/**
 * フォントの並びを画面に当てる。配られたフォントは読み込み終わるまで文字を出さない（終わると wterm が測り直す）。
 * 画面でフォントを選び直したときも呼ぶので、前に配られたフォントは外し、並びが無くなれば既定に戻す。
 */
export function applyFont(faces: FontEntry[] | undefined, root: HTMLElement) {
  for (const face of [...(document.fonts ?? [])]) {
    if (face.family.replace(/^"|"$/g, "").startsWith(SERVED_FONT)) document.fonts.delete(face);
  }
  const stack = fontFamily(faces);
  if (!stack) {
    root.style.removeProperty("--mono-font");
    return;
  }
  faces?.forEach((f, i) => {
    if (!f.url) return;
    const face = new FontFace(servedName(i), `url(${JSON.stringify(f.url)})`, { display: "block" });
    document.fonts.add(face);
    void face.load().catch(() => {
      // 読めなかったら、並びの次のフォントで描く
    });
  });
  root.style.setProperty("--mono-font", stack);
}

/** [font] のファイルを使えなかったときの知らせの文言の名前（stack なら飛ばした、版 1 なら family か JetBrains Mono で描く）。 */
export function fontErrorKey(cfg: FontConfig | undefined): "fontStackError" | "fontFileError" {
  return cfg?.stack ? "fontStackError" : "fontFileError";
}
