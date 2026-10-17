// 画面で選んだ言語・テーマ・フォントを、PC の設定ファイル（language、[theme] の name、[font] の file か stack）に書き戻す。
// サーバーは書き換えた行のほかは触らず、書き間違いがあって読めない設定ファイルには書かない（理由を返す）。

export interface AppearanceChange {
  /** 画面の言語（"ja" か "en"）。 */
  language?: string;
  /** 組み込みのテーマの名前。 */
  theme?: string;
  /** fonts/ の直下のフォントの名前。空なら [font] の file を消す（family か JetBrains Mono に戻る）。 */
  font?: string;
  /** PC に入っているフォントか組み込みのフォント（JetBrains Mono）の名前。[font] の stack をその1つにする。 */
  systemFont?: string;
  /** 「使う順」の並び。[font] の stack をこの順にする（空なら stack の行を消して既定に戻す）。 */
  stack?: string[];
}

/** 書き戻して、書き戻したあとの設定（/api/config と同じ形）を返す。断られたら理由を投げる。 */
export async function chooseAppearance<T>(change: AppearanceChange): Promise<T> {
  const res = await fetch("/api/config/appearance", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(change),
  });
  if (!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`);
  return res.json();
}
