package tui

import (
	"fmt"
	"sync"
	"unicode/utf8"
)

// TextComponent displays text content.
type TextComponent struct {
	Base
	text string
}

// Pool for TextComponent reuse
var textPool = sync.Pool{
	New: func() any { return &TextComponent{} },
}

// Text creates a new text component with the given string.
func Text(s string) *TextComponent {
	t := textPool.Get().(*TextComponent)
	t.Reset()
	t.text = s
	t.style = DefaultStyle()
	t.updateSize()
	return t
}

// Reset clears the component for reuse.
func (t *TextComponent) Reset() {
	*t = TextComponent{}
}

// Textf creates a new text component with printf-style formatting.
func Textf(format string, args ...any) *TextComponent {
	return Text(fmt.Sprintf(format, args...))
}

// updateSize recalculates the minimum size based on text content.
func (t *TextComponent) updateSize() {
	t.minW = utf8.RuneCountInString(t.text)
	t.minH = 1
	if t.minW == 0 {
		t.minH = 0
	}
}

// SetText updates the text content.
func (t *TextComponent) SetText(text string) *TextComponent {
	t.text = text
	t.updateSize()
	return t
}

// GetText returns the text content.
func (t *TextComponent) GetText() string {
	return t.text
}

// SetConstraints implements Component.
func (t *TextComponent) SetConstraints(width, height int) {
	t.Base.SetConstraints(width, height)
	// Text takes its natural size, up to constraints
	t.width = t.minW
	if t.width > width && width > 0 {
		t.width = width
	}
	t.height = t.minH
	if t.height > height && height > 0 {
		t.height = height
	}
}

// Render implements Component.
func (t *TextComponent) Render(buf *Buffer, x, y int) {
	if t.height == 0 || t.width == 0 {
		return
	}
	buf.WriteStringClipped(x, y, t.text, t.style, t.width)
}

// --- Fluent API for styling ---

// Bold makes the text bold.
func (t *TextComponent) Bold() *TextComponent {
	t.style.Attr = t.style.Attr.With(AttrBold)
	return t
}

// Dim makes the text dim.
func (t *TextComponent) Dim() *TextComponent {
	t.style.Attr = t.style.Attr.With(AttrDim)
	return t
}

// Italic makes the text italic.
func (t *TextComponent) Italic() *TextComponent {
	t.style.Attr = t.style.Attr.With(AttrItalic)
	return t
}

// Underline makes the text underlined.
func (t *TextComponent) Underline() *TextComponent {
	t.style.Attr = t.style.Attr.With(AttrUnderline)
	return t
}

// Strikethrough makes the text struck through.
func (t *TextComponent) Strikethrough() *TextComponent {
	t.style.Attr = t.style.Attr.With(AttrStrikethrough)
	return t
}

// Inverse makes the text inverse (swap fg/bg).
func (t *TextComponent) Inverse() *TextComponent {
	t.style.Attr = t.style.Attr.With(AttrInverse)
	return t
}

// Color sets the foreground color.
func (t *TextComponent) Color(c Color) *TextComponent {
	t.style.FG = c
	return t
}

// Fg is an alias for Color.
func (t *TextComponent) Fg(c Color) *TextComponent {
	return t.Color(c)
}

// Bg sets the background color.
func (t *TextComponent) Bg(c Color) *TextComponent {
	t.style.BG = c
	return t
}

// Style sets the complete style.
func (t *TextComponent) Style(s Style) *TextComponent {
	t.style = s
	return t
}

// Grow sets the flex grow factor.
func (t *TextComponent) Grow(factor float64) *TextComponent {
	t.flexGrow = factor
	return t
}

// Ref stores a reference to this component in the provided pointer.
// Useful for later updates.
func (t *TextComponent) Ref(ref **TextComponent) *TextComponent {
	*ref = t
	return t
}
