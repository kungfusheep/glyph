package tui

import "sync"

// Pool for ProgressComponent reuse
var progressPool = sync.Pool{
	New: func() any { return &ProgressComponent{} },
}

// Progress creates a progress bar component.
func Progress(current, total int) *ProgressComponent {
	p := progressPool.Get().(*ProgressComponent)
	p.Reset()
	p.current = current
	p.total = total
	p.width = 30
	p.filled = '█'
	p.empty = '░'
	return p
}

// ProgressComponent displays a progress bar.
type ProgressComponent struct {
	Base
	current int
	total   int
	width   int
	filled  rune
	empty   rune
}

// Reset clears the component for reuse.
func (p *ProgressComponent) Reset() {
	*p = ProgressComponent{}
}

// Width sets the bar width in characters.
func (p *ProgressComponent) Width(w int) *ProgressComponent {
	p.width = w
	return p
}

// Chars sets the filled and empty characters.
func (p *ProgressComponent) Chars(filled, empty rune) *ProgressComponent {
	p.filled = filled
	p.empty = empty
	return p
}

// SetConstraints implements Component.
func (p *ProgressComponent) SetConstraints(width, height int) {
	p.Base.SetConstraints(width, height)
	p.minW = p.width + 2 // brackets
	p.minH = 1
	p.height = 1
	p.Base.width = p.minW
	if p.Base.width > width {
		p.Base.width = width
	}
}

// Render implements Component.
func (p *ProgressComponent) Render(buf *Buffer, x, y int) {
	total := p.total
	if total == 0 {
		total = 1
	}
	filledCount := (p.current * p.width) / total
	if filledCount > p.width {
		filledCount = p.width
	}
	if filledCount < 0 {
		filledCount = 0
	}

	style := p.style
	buf.Set(x, y, NewCell('[', style))
	for i := 0; i < p.width; i++ {
		r := p.empty
		if i < filledCount {
			r = p.filled
		}
		buf.Set(x+1+i, y, NewCell(r, style))
	}
	buf.Set(x+1+p.width, y, NewCell(']', style))
}

// FlexGrow implements the flex interface.
func (p *ProgressComponent) FlexGrow() float64 {
	return p.flexGrow
}

// Grow sets flex grow factor.
func (p *ProgressComponent) Grow(f float64) *ProgressComponent {
	p.flexGrow = f
	return p
}

// ---

// Title creates a title bar with optional right-side content.
func Title(title string, right ...Component) *StackComponent {
	children := []ChildItem{
		Text(title).Bold(),
		Spacer(),
	}
	for _, r := range right {
		children = append(children, r)
	}
	return HStack(children...)
}

// ---

// Fragment creates a bordered pane that groups components.
// Fragments are the building blocks for tiled/dashboard layouts.
func Fragment(children ...ChildItem) *StackComponent {
	return VStack(children...).Border(BorderRounded).Padding(1)
}

// ---

// Window creates a top-level container with optional border and padding.
func Window(children ...ChildItem) *WindowComponent {
	return &WindowComponent{
		content: VStack(children...),
		border:  &BorderRounded,
		padding: 1,
	}
}

// WindowComponent is a top-level container.
type WindowComponent struct {
	Base
	content *StackComponent
	border  *BorderStyle
	padding int
	title   string
}

// Border sets the border style.
func (w *WindowComponent) Border(b BorderStyle) *WindowComponent {
	w.border = &b
	return w
}

// NoBorder removes the border.
func (w *WindowComponent) NoBorder() *WindowComponent {
	w.border = nil
	return w
}

// Padding sets internal padding.
func (w *WindowComponent) Padding(p int) *WindowComponent {
	w.padding = p
	return w
}

// SetConstraints implements Component.
func (w *WindowComponent) SetConstraints(width, height int) {
	w.Base.SetConstraints(width, height)
	w.width = width
	w.height = height

	// Calculate inner dimensions
	innerW, innerH := width, height
	if w.border != nil {
		innerW -= 2
		innerH -= 2
	}
	innerW -= w.padding * 2
	innerH -= w.padding * 2

	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	w.content.SetConstraints(innerW, innerH)
}

// Render implements Component.
func (w *WindowComponent) Render(buf *Buffer, x, y int) {
	// Border
	if w.border != nil {
		buf.DrawBorder(x, y, w.width, w.height, *w.border, w.style)
	}

	// Content position
	contentX := x + w.padding
	contentY := y + w.padding
	if w.border != nil {
		contentX++
		contentY++
	}

	w.content.Render(buf, contentX, contentY)
}

// FlexGrow implements the flex interface.
func (w *WindowComponent) FlexGrow() float64 {
	return w.flexGrow
}

// Grow sets flex grow factor.
func (w *WindowComponent) Grow(f float64) *WindowComponent {
	w.flexGrow = f
	return w
}

// ---

// DataList creates a list from data with a render function.
// This wraps VirtualList with a simpler API.
// By default it grows to fill available space.
func DataList[T any](items []T, render func(T, int) Component) *VirtualList[T] {
	return NewVirtualList(items, 1, render).Grow(1)
}
