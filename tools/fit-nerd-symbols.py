"""Symbols Nerd Font Mono から、JetBrains Mono のマスに合わせたアイコン用フォントを2つ作る。

元のフォントはアイコンが 1em 四方で作られているが、JetBrains Mono の1マスは幅 0.6em なので、
そのままだとはみ出して切れる。WezTerm・kitty・Ghostty と同じく、次のマスが空白なら2マスを使って
本来の大きさで描き、そうでなければ1マスに収める。そのためにフォントを2つ作り、画面側で使い分ける。

- narrow: アイコンを縦横比を保って1マスに収める
- wide:   アイコンを2マスの中央に、文字と釣り合う大きさで置く（1マス目からはみ出して描く）
どちらも Powerline の区切り（U+E0B0〜U+E0D7）は1マスの幅と行の高さいっぱいに引き伸ばし、
送り幅は JetBrains Mono と同じ 600/1000 にそろえる。

使い方: python fit-nerd-symbols.py SymbolsNerdFontMono-Regular.ttf 出力先ディレクトリ
（fonttools と brotli が必要）
"""

import os
import sys

from fontTools.pens.boundsPen import BoundsPen
from fontTools.pens.transformPen import TransformPen
from fontTools.pens.ttGlyphPen import TTGlyphPen
from fontTools.ttLib import TTFont

# JetBrains Mono の寸法（unitsPerEm 1000）
UPM = 1000
ADVANCE = 600
ASCENT = 1020
DESCENT = -300
# wterm の行の高さは 1.2em。JetBrains Mono ではベースラインから上に 960、下に 240 が行の範囲になる
# （内容の高さ 1320 が行 1200 より大きいので、上下に 60 ずつはみ出す分を引く）。
ROW_TOP = 960
ROW_BOTTOM = -240
ROW_CENTER = (ROW_TOP + ROW_BOTTOM) / 2

POWERLINE = range(0xE0B0, 0xE0D8)

# (名前, 入れる箱の幅, 箱の高さ)。箱は行の中央に置き、アイコンは縦横比を保って箱に収める。
# wide の高さ 840 は英大文字（730）より少し大きい程度で、Termux などの見え方に近い。
VARIANTS = {
    "narrow": (ADVANCE - 40, ROW_TOP - ROW_BOTTOM - 120),
    "wide": (2 * ADVANCE - 120, 840),
}


def build(src: str, dst: str, box_w: float, box_h: float) -> None:
    font = TTFont(src)
    glyph_set = font.getGlyphSet()
    stretch = {name for cp, name in font.getBestCmap().items() if cp in POWERLINE}
    # アイコンの中心の x。narrow は1マスの中央、wide は2マスの中央（1マス目の右端）。
    center_x = ADVANCE / 2 if box_w <= ADVANCE else ADVANCE

    new_glyphs = {}
    for name in font.getGlyphOrder():
        bounds_pen = BoundsPen(glyph_set)
        glyph_set[name].draw(bounds_pen)
        pen = TTGlyphPen(None)
        if bounds_pen.bounds is not None:
            x0, y0, x1, y1 = bounds_pen.bounds
            w, h = max(x1 - x0, 1), max(y1 - y0, 1)
            if name in stretch:
                sx, sy = ADVANCE / w, (ROW_TOP - ROW_BOTTOM) / h
                dx, dy = -x0 * sx, ROW_BOTTOM - y0 * sy
            else:
                sx = sy = min(box_w / w, box_h / h)
                dx = center_x - (x0 + w / 2) * sx
                dy = ROW_CENTER - (y0 + h / 2) * sy
            glyph_set[name].draw(TransformPen(pen, (sx, 0, 0, sy, dx, dy)))
        new_glyphs[name] = pen.glyph()

    glyf = font["glyf"]
    for name, g in new_glyphs.items():
        glyf[name] = g
        g.recalcBounds(glyf)
        font["hmtx"][name] = (ADVANCE, getattr(g, "xMin", 0))

    # 拡大・縮小したのでヒンティングは外す。
    for tag in ("fpgm", "prep", "cvt ", "hdmx", "LTSH", "VDMX", "gasp"):
        if tag in font:
            del font[tag]

    font["head"].unitsPerEm = UPM
    hhea = font["hhea"]
    hhea.ascent, hhea.descent, hhea.lineGap = ASCENT, DESCENT, 0
    hhea.advanceWidthMax = ADVANCE
    os2 = font["OS/2"]
    os2.sTypoAscender, os2.sTypoDescender, os2.sTypoLineGap = ASCENT, DESCENT, 0
    os2.usWinAscent, os2.usWinDescent = ASCENT, -DESCENT
    os2.xAvgCharWidth = ADVANCE

    font.flavor = "woff2"
    font.save(dst)


def main(src: str, out_dir: str) -> None:
    for variant, (box_w, box_h) in VARIANTS.items():
        build(src, os.path.join(out_dir, f"NerdSymbols-{variant}.woff2"), box_w, box_h)


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
