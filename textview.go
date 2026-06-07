package glyph

import (
	"unicode/utf8"
	"unsafe"
)

type TextViewC struct {
	content          *string
	layer            *Layer
	grow             float32
	margin           [4]int16
	lastDataPtr      unsafe.Pointer
	lastLen          int
	lastWidth        int
	declaredBindings []binding
	flexGrowPtr      *float32
	flexGrowCond     conditionNode
}

// TextView creates a scrollable multi-line text display with word wrapping.
// Content is re-wrapped automatically when the text or viewport width changes.
func TextView(content *string) *TextViewC {
	tv := &TextViewC{
		content: content,
		layer:   NewLayer(),
	}
	tv.layer.AlwaysRender = true
	tv.layer.Render = tv.sync
	return tv
}

// Grow sets the flex grow factor so the view expands to fill available space.
// Accepts float32, float64, int, or *float32 for dynamic values.
func (tv *TextViewC) Grow(g any) *TextViewC {
	switch val := g.(type) {
	case float32:
		tv.grow = val
	case float64:
		tv.grow = float32(val)
	case int:
		tv.grow = float32(val)
	case *float32:
		tv.flexGrowPtr = val
	case conditionNode:
		tv.flexGrowCond = val
	}
	return tv
}

// Margin sets uniform margin on all sides.
func (tv *TextViewC) Margin(all int16) *TextViewC {
	tv.margin = [4]int16{all, all, all, all}
	return tv
}

// MarginVH sets vertical and horizontal margin.
func (tv *TextViewC) MarginVH(v, h int16) *TextViewC {
	tv.margin = [4]int16{v, h, v, h}
	return tv
}

// MarginTRBL sets individual margins for top, right, bottom, left.
func (tv *TextViewC) MarginTRBL(t, r, b, l int16) *TextViewC {
	tv.margin = [4]int16{t, r, b, l}
	return tv
}

// Layer returns the underlying layer for external scroll wiring.
func (tv *TextViewC) Layer() *Layer { return tv.layer }

// BindNav registers keys for line-by-line navigation.
func (tv *TextViewC) BindNav(down, up string) *TextViewC {
	tv.declaredBindings = append(tv.declaredBindings,
		binding{pattern: down, handler: func() { tv.layer.ScrollDown(1) }},
		binding{pattern: up, handler: func() { tv.layer.ScrollUp(1) }},
	)
	return tv
}

// BindPageNav registers keys for half-page navigation.
func (tv *TextViewC) BindPageNav(down, up string) *TextViewC {
	tv.declaredBindings = append(tv.declaredBindings,
		binding{pattern: down, handler: func() { tv.layer.HalfPageDown() }},
		binding{pattern: up, handler: func() { tv.layer.HalfPageUp() }},
	)
	return tv
}

func (tv *TextViewC) bindings() []binding { return tv.declaredBindings }

func (tv *TextViewC) sync() {
	c := *tv.content
	w := tv.layer.ViewportWidth()
	if w <= 0 {
		return
	}
	dataPtr := unsafe.StringData(c)
	if w == tv.lastWidth && len(c) == tv.lastLen && unsafe.Pointer(dataPtr) == tv.lastDataPtr {
		return
	}
	tv.lastDataPtr = unsafe.Pointer(dataPtr)
	tv.lastLen = len(c)
	tv.lastWidth = w

	lines := wrapText(c, w)
	if len(lines) == 0 {
		lines = []string{""}
	}
	h := max(len(lines), tv.layer.ViewportHeight())
	buf := NewBuffer(w, h)
	buf.defaultStyle = tv.layer.defaultStyle
	buf.Clear()
	for i, line := range lines {
		buf.WriteStringFast(0, i, line, Style{}, w)
	}
	tv.layer.SetBuffer(buf)
}

func (t *Template) compileTextViewC(v *TextViewC, parent int16, depth int) int16 {
	var layerView LayerViewC
	if v.flexGrowCond != nil {
		layerView = LayerView(v.layer).Grow(v.flexGrowCond)
	} else if v.flexGrowPtr != nil {
		layerView = LayerView(v.layer).Grow(v.flexGrowPtr)
	} else {
		layerView = LayerView(v.layer).Grow(v.grow)
	}
	if v.margin != [4]int16{} {
		layerView = layerView.MarginTRBL(v.margin[0], v.margin[1], v.margin[2], v.margin[3])
	}
	return t.compileLayerViewC(layerView, parent, depth)
}

// wrapTextDraw wraps s to width and writes runes directly to buf at (x, y).
// charWrap=true breaks mid-word, false breaks at word boundaries.
// maxLines limits the output; 0 means unlimited.
// returns the number of lines produced (before clipping by maxLines).
// zero allocation: runes are written directly to the target buffer.
func wrapTextDraw(s string, buf *Buffer, x, y int, width, maxLines int, style Style, charWrap bool) int {
	if width <= 0 {
		return 0
	}
	if charWrap {
		return wrapDrawChar(s, buf, x, y, width, maxLines, style)
	}
	return wrapDrawWord(s, buf, x, y, width, maxLines, style)
}

// wrapTextLines returns the line count that wrapTextDraw would produce.
// used by layout to size the TextBlock before render.
func wrapTextLines(s string, width int, charWrap bool) int {
	if width <= 0 {
		return 0
	}
	if charWrap {
		return wrapDrawChar(s, nil, 0, 0, width, 0, Style{})
	}
	return wrapDrawWord(s, nil, 0, 0, width, 0, Style{})
}

// wrapSpansDraw wraps styled spans to width and writes them to buf.
// It mirrors TextBlock wrapping while preserving each span's style.
type spanJumpFunc func(x, y int, span Span)

func wrapSpansDraw(spans []Span, buf *Buffer, x, y int, width, maxLines int, charWrap bool, jump spanJumpFunc) int {
	if width <= 0 {
		return 0
	}
	if charWrap {
		return wrapSpansChar(spans, buf, x, y, width, maxLines, jump)
	}
	return wrapSpansWord(spans, buf, x, y, width, maxLines, jump)
}

func wrapSpansLines(spans []Span, width int, charWrap bool) int {
	if width <= 0 {
		return 0
	}
	return wrapSpansDraw(spans, nil, 0, 0, width, 0, charWrap, nil)
}

func wrapSpansWord(spans []Span, buf *Buffer, x, y, width, maxLines int, jump spanJumpFunc) int {
	row := 0
	col := 0
	var rw RowWriter
	style := Style{}
	if buf != nil {
		rw = buf.Row(y, style)
	}

	write := func(r rune, span Span, jumped *bool, allowJump bool) {
		if buf == nil || (maxLines > 0 && row >= maxLines) {
			return
		}
		if allowJump && !*jumped && jump != nil && (span.OnSelect != nil || span.OnSelectRef != nil) {
			jump(x+col, y+row, span)
			*jumped = true
		}
		if !span.Style.Equal(style) {
			style = span.Style
			rw = buf.Row(y+row, style)
		}
		rw.Put(x+col, r)
	}
	flush := func() {
		row++
		col = 0
		if buf != nil {
			rw = buf.Row(y+row, style)
		}
	}

	pendingSpace := false
	pendingSpaceStyle := Style{}
	for _, span := range spans {
		text := span.Text
		jumped := false
		for i := 0; i < len(text); {
			if text[i] == '\n' {
				flush()
				pendingSpace = false
				pendingSpaceStyle = Style{}
				i++
				continue
			}
			spaceBefore := pendingSpace
			spaceStyle := pendingSpaceStyle
			pendingSpace = false
			pendingSpaceStyle = Style{}
			for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
				if col > 0 {
					spaceBefore = true
					spaceStyle = span.Style
				}
				i++
			}
			if i >= len(text) {
				pendingSpace = spaceBefore
				pendingSpaceStyle = spaceStyle
				continue
			}
			if text[i] == '\n' {
				pendingSpace = false
				pendingSpaceStyle = Style{}
				continue
			}
			wordStart := i
			wordRunes := 0
			for i < len(text) && text[i] != ' ' && text[i] != '\t' && text[i] != '\n' {
				_, size := utf8.DecodeRuneInString(text[i:])
				i += size
				wordRunes++
			}
			word := text[wordStart:i]

			if col == 0 {
				if wordRunes <= width {
					for _, r := range word {
						write(r, span, &jumped, true)
						col++
					}
				} else {
					for _, r := range word {
						if col >= width {
							flush()
						}
						write(r, span, &jumped, true)
						col++
					}
				}
			} else if spaceBefore && col+1+wordRunes <= width {
				write(' ', Span{Style: spaceStyle}, &jumped, false)
				col++
				for _, r := range word {
					write(r, span, &jumped, true)
					col++
				}
			} else if !spaceBefore && col+wordRunes <= width {
				for _, r := range word {
					write(r, span, &jumped, true)
					col++
				}
			} else {
				flush()
				if wordRunes <= width {
					for _, r := range word {
						write(r, span, &jumped, true)
						col++
					}
				} else {
					for _, r := range word {
						if col >= width {
							flush()
						}
						write(r, span, &jumped, true)
						col++
					}
				}
			}
		}
	}
	return row + 1
}

func wrapSpansChar(spans []Span, buf *Buffer, x, y, width, maxLines int, jump spanJumpFunc) int {
	const tabWidth = 4
	row := 0
	col := 0
	var rw RowWriter
	style := Style{}
	if buf != nil {
		rw = buf.Row(y, style)
	}

	write := func(r rune, span Span, jumped *bool, allowJump bool) {
		if buf == nil || (maxLines > 0 && row >= maxLines) {
			return
		}
		if allowJump && !*jumped && jump != nil && (span.OnSelect != nil || span.OnSelectRef != nil) {
			jump(x+col, y+row, span)
			*jumped = true
		}
		if !span.Style.Equal(style) {
			style = span.Style
			rw = buf.Row(y+row, style)
		}
		rw.Put(x+col, r)
	}
	flush := func() {
		row++
		col = 0
		if buf != nil {
			rw = buf.Row(y+row, style)
		}
	}

	for _, span := range spans {
		jumped := false
		for i := 0; i < len(span.Text); {
			r, size := utf8.DecodeRuneInString(span.Text[i:])
			i += size

			if r == '\n' {
				flush()
				continue
			}
			if r == '\t' {
				spaces := tabWidth - (col % tabWidth)
				for j := 0; j < spaces; j++ {
					if col >= width {
						flush()
					}
					write(' ', span, &jumped, false)
					col++
				}
				continue
			}
			if col >= width {
				flush()
			}
			write(r, span, &jumped, true)
			col++
		}
	}
	return row + 1
}

// wrapDrawWord does the actual word-wrap draw. If buf is nil, only counts lines.
func wrapDrawWord(s string, buf *Buffer, x, y, width, maxLines int, style Style) int {
	row := 0
	col := 0
	var rw RowWriter
	if buf != nil {
		rw = buf.Row(y, style)
	}

	write := func(r rune) {
		if buf == nil || (maxLines > 0 && row >= maxLines) {
			return
		}
		rw.Put(x+col, r)
	}
	flush := func() {
		row++
		col = 0
		if buf != nil {
			rw = buf.Row(y+row, style)
		}
	}

	i := 0
	for i < len(s) {
		if s[i] == '\n' {
			flush()
			i++
			continue
		}
		if col == 0 && (s[i] == ' ' || s[i] == '\t') {
			i++
			continue
		}
		wordStart := i
		wordRunes := 0
		for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\n' {
			_, size := utf8.DecodeRuneInString(s[i:])
			i += size
			wordRunes++
		}
		word := s[wordStart:i]

		if col == 0 {
			if wordRunes <= width {
				for _, r := range word {
					write(r)
					col++
				}
			} else {
				for _, r := range word {
					if col >= width {
						flush()
					}
					write(r)
					col++
				}
			}
		} else if col+1+wordRunes <= width {
			write(' ')
			col++
			for _, r := range word {
				write(r)
				col++
			}
		} else {
			flush()
			if wordRunes <= width {
				for _, r := range word {
					write(r)
					col++
				}
			} else {
				for _, r := range word {
					if col >= width {
						flush()
					}
					write(r)
					col++
				}
			}
		}
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
	}
	return row + 1
}

// wrapDrawChar does the actual character-wrap draw. If buf is nil, only counts lines.
func wrapDrawChar(s string, buf *Buffer, x, y, width, maxLines int, style Style) int {
	const tabWidth = 4
	row := 0
	col := 0
	var rw RowWriter
	if buf != nil {
		rw = buf.Row(y, style)
	}

	write := func(r rune) {
		if buf == nil || (maxLines > 0 && row >= maxLines) {
			return
		}
		rw.Put(x+col, r)
	}
	flush := func() {
		row++
		col = 0
		if buf != nil {
			rw = buf.Row(y+row, style)
		}
	}

	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size

		if r == '\n' {
			flush()
			continue
		}
		if r == '\t' {
			spaces := tabWidth - (col % tabWidth)
			for j := 0; j < spaces; j++ {
				if col >= width {
					flush()
				}
				write(' ')
				col++
			}
			continue
		}
		if col >= width {
			flush()
		}
		write(r)
		col++
	}
	return row + 1
}

// wrapText returns character-wrapped lines as strings (test compat).
func wrapText(s string, width int) []string {
	if width <= 0 {
		return nil
	}
	var out []string
	line := make([]byte, 0, width*4)
	const tabWidth = 4
	col := 0
	flush := func() {
		out = append(out, string(line))
		line = line[:0]
		col = 0
	}
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if r == '\n' {
			flush()
			continue
		}
		if r == '\t' {
			spaces := tabWidth - (col % tabWidth)
			for j := 0; j < spaces; j++ {
				if col >= width {
					flush()
				}
				line = append(line, ' ')
				col++
			}
			continue
		}
		if col >= width {
			flush()
		}
		line = utf8.AppendRune(line, r)
		col++
	}
	out = append(out, string(line))
	return out
}

// wrapTextWord returns word-wrapped lines as strings (test compat).
func wrapTextWord(s string, width int) []string {
	if width <= 0 {
		return nil
	}
	var out []string
	line := make([]byte, 0, width*4)
	col := 0
	flush := func() {
		out = append(out, string(line))
		line = line[:0]
		col = 0
	}
	i := 0
	for i < len(s) {
		if s[i] == '\n' {
			flush()
			i++
			continue
		}
		if col == 0 && (s[i] == ' ' || s[i] == '\t') {
			i++
			continue
		}
		wordStart := i
		wordRunes := 0
		for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\n' {
			_, size := utf8.DecodeRuneInString(s[i:])
			i += size
			wordRunes++
		}
		word := s[wordStart:i]
		if col == 0 {
			if wordRunes <= width {
				line = append(line, word...)
				col = wordRunes
			} else {
				for _, r := range word {
					if col >= width {
						flush()
					}
					line = utf8.AppendRune(line, r)
					col++
				}
			}
		} else if col+1+wordRunes <= width {
			line = append(line, ' ')
			line = append(line, word...)
			col += 1 + wordRunes
		} else {
			flush()
			if wordRunes <= width {
				line = append(line, word...)
				col = wordRunes
			} else {
				for _, r := range word {
					if col >= width {
						flush()
					}
					line = utf8.AppendRune(line, r)
					col++
				}
			}
		}
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
	}
	out = append(out, string(line))
	return out
}
