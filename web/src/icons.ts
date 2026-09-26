// ボタンに使う線画のアイコン（24×24、線の色は文字の色）。記号の文字はフォントや端末で見た目が変わるので使わない。

const PATHS = {
  left: ["M19 12H5", "m12 19-7-7 7-7"],
  right: ["M5 12h14", "m12 5 7 7-7 7"],
  up: ["M12 19V5", "m5 12 7-7 7 7"],
  down: ["M12 5v14", "m19 12-7 7-7-7"],
  backspace: [
    "M10 5a2 2 0 0 0-1.34.52l-6.33 5.74a1 1 0 0 0 0 1.48l6.33 5.74A2 2 0 0 0 10 19h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2z",
    "m12 9 6 6",
    "m18 9-6 6",
  ],
  enter: ["M20 4v7a4 4 0 0 1-4 4H4", "m9 10-5 5 5 5"],
  close: ["M18 6 6 18", "m6 6 12 12"],
  plus: ["M12 5v14", "M5 12h14"],
  splitH: ["M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z", "M12 3v18"],
  splitV: ["M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z", "M3 12h18"],
  maximize: ["M8 3H5a2 2 0 0 0-2 2v3", "M21 8V5a2 2 0 0 0-2-2h-3", "M3 16v3a2 2 0 0 0 2 2h3", "M16 21h3a2 2 0 0 0 2-2v-3"],
  cycle: ["M21 12a9 9 0 1 1-2.64-6.36L21 8", "M21 3v5h-5"],
  panes: ["M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z", "M12 3v18", "M12 12h9"],
  textSmaller: ["m14 12 4 4 4-4", "M18 16V7", "m2 16 4.04-9.69a.5.5 0 0 1 .92 0L11 16", "M3.3 13h6.4"],
  textLarger: ["m14 11 4-4 4 4", "M18 16V7", "m2 16 4.04-9.69a.5.5 0 0 1 .92 0L11 16", "M3.3 13h6.4"],
} as const;

export type IconName = keyof typeof PATHS;

export function isIconName(name: string): name is IconName {
  return Object.hasOwn(PATHS, name);
}

const SVG_NS = "http://www.w3.org/2000/svg";

export function icon(name: IconName, size = 20): SVGSVGElement {
  const svg = document.createElementNS(SVG_NS, "svg");
  svg.setAttribute("viewBox", "0 0 24 24");
  svg.setAttribute("width", String(size));
  svg.setAttribute("height", String(size));
  svg.setAttribute("aria-hidden", "true");
  svg.setAttribute("fill", "none");
  svg.setAttribute("stroke", "currentColor");
  svg.setAttribute("stroke-width", "2");
  svg.setAttribute("stroke-linecap", "round");
  svg.setAttribute("stroke-linejoin", "round");
  svg.classList.add("icon");
  for (const d of PATHS[name]) {
    const path = document.createElementNS(SVG_NS, "path");
    path.setAttribute("d", d);
    svg.appendChild(path);
  }
  return svg;
}
