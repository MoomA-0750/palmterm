// 端末のフォント（設定ファイルの [font]）。PC から配られたフォントがあればそれを、無ければ family の名前で
// スマホに入っているフォントを探し、どちらも無ければ今までどおり JetBrains Mono を使う。
// Nerd Font のアイコンは、どのフォントにしても palmterm が配る Nerd Symbols で描く（fonts.css）。

/** 設定ファイルの [font]（サーバーはそのまま渡す）。 */
export interface FontConfig {
  family?: string;
  file?: string;
}

/** PC から配られたフォントに付ける名前（ページの中だけで使う）。 */
export const SERVED_FONT = "palmterm font";

/** 既定の並び（fonts.css の --mono-font）のうち、JetBrains Mono から後ろ。 */
const FALLBACK = `"JetBrains Mono", "Noto Sans Mono CJK JP", "Noto Sans CJK JP", "Hiragino Sans", ui-monospace, monospace`;

function cssString(s: string): string {
  return `"${s.replace(/[\\"]/g, (c) => `\\${c}`).replace(/[\n\r\f]/g, " ")}"`;
}

/**
 * 端末の font-family。[font] を書いていなければ null（fonts.css の既定のまま）。
 * Nerd Symbols は私用領域の文字だけを受け持つので先頭に置き、選んだフォントがアイコンを持っていても
 * 今までどおりの形で描く。
 */
export function fontFamily(cfg: FontConfig | undefined, servedURL: string | undefined): string | null {
  const family = cfg?.family?.trim();
  if (!servedURL && !family) return null;
  return [`"Nerd Symbols"`, servedURL ? cssString(SERVED_FONT) : "", family ? cssString(family) : "", FALLBACK]
    .filter(Boolean)
    .join(", ");
}

/** [font] を画面に当てる。配られたフォントは読み込み終わるまで文字を出さない（終わると wterm が測り直す）。 */
export function applyFont(cfg: FontConfig | undefined, servedURL: string | undefined, root: HTMLElement) {
  const stack = fontFamily(cfg, servedURL);
  if (!stack) return;
  if (servedURL) {
    const face = new FontFace(SERVED_FONT, `url(${JSON.stringify(servedURL)})`, { display: "block" });
    document.fonts.add(face);
    void face.load().catch(() => {
      // 読めなかったら、並びの次のフォントで描く
    });
  }
  root.style.setProperty("--mono-font", stack);
}
