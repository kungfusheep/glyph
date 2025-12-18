package tui

import "sync"

// GridComponent arranges children in a grid of rows and columns.
type GridComponent struct {
	Base
	children []Component
	cols     int // number of columns

	gap    int
	border *BorderStyle
}

// Pool for GridComponent reuse
var gridPool = sync.Pool{
	New: func() any { return &GridComponent{} },
}

// Grid creates a grid layout with the specified number of columns.
// Children fill left-to-right, top-to-bottom.
func Grid(cols int, children ...ChildItem) *GridComponent {
	g := gridPool.Get().(*GridComponent)
	g.Reset()
	g.cols = cols
	g.style = DefaultStyle()

	// Pre-allocate if needed
	if cap(g.children) < len(children) {
		g.children = make([]Component, 0, len(children))
	}

	// Flatten children (handle iterators if needed)
	for _, item := range children {
		if c, ok := item.(Component); ok {
			g.children = append(g.children, c)
		}
	}

	return g
}

// Reset clears the component for reuse.
func (g *GridComponent) Reset() {
	g.children = g.children[:0] // Keep capacity
	g.Base = Base{}
	g.cols = 0
	g.gap = 0
	g.border = nil
}

// Cols2 creates a 2-column grid.
func Cols2(children ...ChildItem) *GridComponent {
	return Grid(2, children...)
}

// Cols3 creates a 3-column grid.
func Cols3(children ...ChildItem) *GridComponent {
	return Grid(3, children...)
}

// SetConstraints implements Component.
func (g *GridComponent) SetConstraints(width, height int) {
	g.Base.SetConstraints(width, height)
	g.width = width
	g.height = height

	if len(g.children) == 0 || g.cols == 0 {
		return
	}

	// Calculate rows needed
	rows := (len(g.children) + g.cols - 1) / g.cols

	// Calculate cell dimensions
	totalGapW := g.gap * (g.cols - 1)
	totalGapH := g.gap * (rows - 1)

	cellW := (width - totalGapW) / g.cols
	cellH := (height - totalGapH) / rows

	if cellW < 1 {
		cellW = 1
	}
	if cellH < 1 {
		cellH = 1
	}

	// Set constraints on all children
	for _, child := range g.children {
		child.SetConstraints(cellW, cellH)
	}
}

// Render implements Component.
func (g *GridComponent) Render(buf *Buffer, x, y int) {
	if len(g.children) == 0 || g.cols == 0 {
		return
	}

	rows := (len(g.children) + g.cols - 1) / g.cols

	totalGapW := g.gap * (g.cols - 1)
	totalGapH := g.gap * (rows - 1)

	cellW := (g.width - totalGapW) / g.cols
	cellH := (g.height - totalGapH) / rows

	for i, child := range g.children {
		col := i % g.cols
		row := i / g.cols

		childX := x + col*(cellW+g.gap)
		childY := y + row*(cellH+g.gap)

		child.Render(buf, childX, childY)
	}
}

// MinSize implements Component.
func (g *GridComponent) MinSize() (int, int) {
	if len(g.children) == 0 {
		return 0, 0
	}

	// Find max min size among children
	var maxW, maxH int
	for _, child := range g.children {
		w, h := child.MinSize()
		if w > maxW {
			maxW = w
		}
		if h > maxH {
			maxH = h
		}
	}

	rows := (len(g.children) + g.cols - 1) / g.cols

	totalW := maxW*g.cols + g.gap*(g.cols-1)
	totalH := maxH*rows + g.gap*(rows-1)

	return totalW, totalH
}

// Size implements Component.
func (g *GridComponent) Size() (int, int) {
	return g.width, g.height
}

// FlexGrow implements flex interface.
func (g *GridComponent) FlexGrow() float64 {
	return g.flexGrow
}

// --- Fluent API ---

// Gap sets the gap between cells.
func (g *GridComponent) Gap(gap int) *GridComponent {
	g.gap = gap
	return g
}

// Grow sets flex grow factor.
func (g *GridComponent) Grow(f float64) *GridComponent {
	g.flexGrow = f
	return g
}

// Border sets the border style.
func (g *GridComponent) Border(b BorderStyle) *GridComponent {
	g.border = &b
	return g
}
