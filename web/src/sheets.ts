// 板（設定・ログイン中の端末・フォント・履歴/クリップボード）を、大きい画面では中央に浮かせる。
// 浮かせる見た目は style.css の同じ条件の @media が受け持ち、ここは周りを暗くする幕と、
// 幕を押す・Esc で一番上の板を閉じることを受け持つ。小さい画面では今までどおり全画面で、幕は出さない。

/** 板を浮かせる画面の大きさ。style.css の @media と同じにする。 */
export const FLOAT_QUERY = "(min-width: 700px) and (min-height: 600px)";

export interface Sheet {
  el: HTMLElement;
  /** 板の「閉じる」。幕や Esc で閉じるときもこれを押す（閉じたあとの片付けを同じにする）。 */
  close: HTMLElement;
}

/** sheets は下から上の順（後ろのものほど上に重なる）。 */
export function setupSheets(sheets: Sheet[], win: Window = window) {
  const doc = win.document;
  const backdrop = doc.createElement("div");
  backdrop.id = "sheet-backdrop";
  backdrop.hidden = true;
  doc.body.append(backdrop);

  // 別の板の中に入っている板（2列の設定画面の右の一覧）は、外の板と一緒に扱う。
  const nested = (s: Sheet) => sheets.some((o) => o !== s && o.el.contains(s.el));
  const top = () => [...sheets].reverse().find((s) => !s.el.hidden && !nested(s));
  const floating = () => win.matchMedia(FLOAT_QUERY).matches;

  // 幕は一番上の板のすぐ下に置く（設定画面から開いた一覧なら、設定画面も暗くなる）。
  function update() {
    const s = top();
    backdrop.hidden = !s;
    if (s) backdrop.style.zIndex = String((parseInt(win.getComputedStyle(s.el).zIndex, 10) || 10) - 1);
  }
  const observer = new MutationObserver(update);
  for (const s of sheets) observer.observe(s.el, { attributes: true, attributeFilter: ["hidden"] });
  update();

  backdrop.addEventListener("click", () => top()?.close.click());

  // 端末や入力欄より先に受け取る。板の中の入力欄（端末の名前を書き換え中など）の Esc はそちらに任せる。
  win.addEventListener(
    "keydown",
    (e) => {
      if (e.key !== "Escape" || e.isComposing || !floating()) return;
      const s = top();
      if (!s) return;
      const target = e.target as HTMLElement | null;
      if (target && s.el.contains(target) && target.matches("input, textarea")) return;
      e.preventDefault();
      e.stopPropagation();
      s.close.click();
    },
    true,
  );

  return { backdrop };
}
