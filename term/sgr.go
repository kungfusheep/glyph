package term

import glyph "github.com/kungfusheep/glyph"

// sgr applies a Select Graphic Rendition sequence to the current pen. It walks
// the parameter list, handling attribute toggles, the 16 named colours, their
// bright variants, and the extended 38/48 forms (256-index and truecolor).
func (s *screen) sgr() {
	if len(s.params) == 0 {
		s.pen = glyph.Style{}
		return
	}
	for i := 0; i < len(s.params); i++ {
		p := s.params[i]
		switch {
		case p == 0:
			s.pen = glyph.Style{}
		case p == 1:
			s.pen.Attr = s.pen.Attr.With(glyph.AttrBold)
		case p == 2:
			s.pen.Attr = s.pen.Attr.With(glyph.AttrDim)
		case p == 3:
			s.pen.Attr = s.pen.Attr.With(glyph.AttrItalic)
		case p == 4:
			s.pen.Attr = s.pen.Attr.With(glyph.AttrUnderline)
		case p == 5:
			s.pen.Attr = s.pen.Attr.With(glyph.AttrBlink)
		case p == 7:
			s.pen.Attr = s.pen.Attr.With(glyph.AttrInverse)
		case p == 9:
			s.pen.Attr = s.pen.Attr.With(glyph.AttrStrikethrough)
		case p == 22:
			s.pen.Attr = s.pen.Attr.Without(glyph.AttrBold).Without(glyph.AttrDim)
		case p == 23:
			s.pen.Attr = s.pen.Attr.Without(glyph.AttrItalic)
		case p == 24:
			s.pen.Attr = s.pen.Attr.Without(glyph.AttrUnderline)
		case p == 25:
			s.pen.Attr = s.pen.Attr.Without(glyph.AttrBlink)
		case p == 27:
			s.pen.Attr = s.pen.Attr.Without(glyph.AttrInverse)
		case p == 29:
			s.pen.Attr = s.pen.Attr.Without(glyph.AttrStrikethrough)
		case p >= 30 && p <= 37:
			s.pen.FG = glyph.Ansi16(uint8(p - 30))
		case p == 38:
			c, adv, ok := extendedColor(s.params[i:])
			if ok {
				s.pen.FG = c
			}
			i += adv
		case p == 39:
			s.pen.FG = glyph.DefaultColor()
		case p >= 40 && p <= 47:
			s.pen.BG = glyph.Ansi16(uint8(p - 40))
		case p == 48:
			c, adv, ok := extendedColor(s.params[i:])
			if ok {
				s.pen.BG = c
			}
			i += adv
		case p == 49:
			s.pen.BG = glyph.DefaultColor()
		case p >= 90 && p <= 97:
			s.pen.FG = glyph.Ansi16(uint8(p-90) + 8)
		case p >= 100 && p <= 107:
			s.pen.BG = glyph.Ansi16(uint8(p-100) + 8)
		}
	}
}

// extendedColor parses the 38/48 extended color forms starting at params[0]
// (the 38 or 48 selector). It returns the color, how many EXTRA params it
// consumed beyond the selector, and whether a color was produced.
//
//	5;n       -> 256-palette index n      (consumes 2)
//	2;r;g;b   -> truecolor                (consumes 4)
func extendedColor(params []int) (glyph.Color, int, bool) {
	if len(params) < 2 {
		return glyph.Color{}, 0, false
	}
	switch params[1] {
	case 5:
		if len(params) < 3 {
			return glyph.Color{}, 1, false
		}
		return glyph.Ansi256(uint8(params[2])), 2, true
	case 2:
		if len(params) < 5 {
			return glyph.Color{}, len(params) - 1, false
		}
		return glyph.RGB(uint8(params[2]), uint8(params[3]), uint8(params[4])), 4, true
	}
	return glyph.Color{}, 1, false
}
