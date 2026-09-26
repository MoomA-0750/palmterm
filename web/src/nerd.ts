// Nerd Font のアイコンを、次のマスが空白（または行末）なら2マス分の大きさで描く。
// WezTerm・kitty・Ghostty と同じ考え方。そうでなければ1マスに収めた形のまま。
//
// wterm は ASCII 以外の文字を1文字ずつ別の span（幅1マス、はみ出しは切る）で描くので、
// 描き直された行を見張って、アイコンの span に nf-wide を付け外しする（見た目は style.css）。
// MutationObserver の処理は描画の前に走るので、ちらつかない。

function isIcon(cp: number): boolean {
  const pua = (cp >= 0xe000 && cp <= 0xf8ff) || (cp >= 0xf0000 && cp <= 0xffffd);
  // Powerline の区切りは、隣とつながるよう常に1マスいっぱいに描く。
  const powerline = cp >= 0xe0b0 && cp <= 0xe0d7;
  return pua && !powerline;
}

/** 次のマスの文字。行末なら null。 */
function nextCellText(span: Element): string | null {
  let next = span.nextElementSibling;
  if (!next && span.parentElement?.classList.contains("term-link")) {
    next = span.parentElement.nextElementSibling;
  }
  if (!next) return null;
  if (next.classList.contains("term-link")) next = next.firstElementChild ?? next;
  return next.textContent ?? "";
}

// 私用領域（U+E000〜F8FF と、U+F0000 以降の上位サロゲート）の文字を含むか。
const MAYBE_ICON = /[\ue000-\uf8ff\udb80-\udbbf]/;

function processRow(row: Element) {
  // たいていの行にはアイコンがないので、行の文字をまとめて見て、なければ span を1つずつ調べない。
  if (!MAYBE_ICON.test(row.textContent ?? "")) {
    for (const span of row.querySelectorAll(".nf-wide")) span.classList.remove("nf-wide");
    return;
  }
  for (const span of row.querySelectorAll("span")) {
    const text = span.textContent ?? "";
    const cp = text.codePointAt(0);
    const icon = cp !== undefined && text.length <= 2 && [...text].length === 1 && isIcon(cp);
    if (!icon) {
      if (span.classList.contains("nf-wide")) span.classList.remove("nf-wide");
      continue;
    }
    const next = nextCellText(span);
    span.classList.toggle("nf-wide", next === null || next.startsWith(" "));
  }
}

export function setupNerdIcons(termEl: HTMLElement) {
  const observer = new MutationObserver((records) => {
    const rows = new Set<Element>();
    for (const r of records) {
      const target = r.target instanceof Element ? r.target : r.target.parentElement;
      const row = target?.closest(".term-row");
      if (row) rows.add(row);
      for (const node of r.addedNodes) {
        if (node instanceof Element) {
          if (node.classList.contains("term-row")) rows.add(node);
          else node.querySelectorAll(".term-row").forEach((el) => rows.add(el));
        }
      }
    }
    rows.forEach(processRow);
  });
  observer.observe(termEl, { childList: true, subtree: true, characterData: true });
  termEl.querySelectorAll(".term-row").forEach(processRow);
}
