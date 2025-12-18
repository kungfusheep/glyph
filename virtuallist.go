package tui

// VirtualList renders only visible items from a large dataset.
// It maintains a viewport window and only creates components for visible rows.
type VirtualList[T any] struct {
	Base

	items      []T
	render     func(item T, index int) Component
	itemHeight int // fixed height per item (for now)

	// Viewport state
	scrollOffset  int // first visible item index
	viewportRows  int // how many rows fit in viewport
	viewportHeight int

	// Cached visible components (reused on scroll)
	visibleComponents []Component

	// Decoration
	border     *BorderStyle
	background *Color
	padding    int
}

// NewVirtualList creates a virtual list with the given render function.
// itemHeight is the fixed height of each row (required for virtual scrolling).
func NewVirtualList[T any](items []T, itemHeight int, render func(T, int) Component) *VirtualList[T] {
	v := &VirtualList[T]{
		items:      items,
		render:     render,
		itemHeight: itemHeight,
	}
	v.style = DefaultStyle()
	return v
}

// SetItems replaces the item list.
func (v *VirtualList[T]) SetItems(items []T) *VirtualList[T] {
	v.items = items
	// Clamp scroll offset
	if v.scrollOffset > len(items)-v.viewportRows {
		v.scrollOffset = max(0, len(items)-v.viewportRows)
	}
	v.rebuildVisible()
	return v
}

// Items returns the current items.
func (v *VirtualList[T]) Items() []T {
	return v.items
}

// Len returns total item count.
func (v *VirtualList[T]) Len() int {
	return len(v.items)
}

// ScrollTo scrolls to show the item at index.
func (v *VirtualList[T]) ScrollTo(index int) *VirtualList[T] {
	if index < 0 {
		index = 0
	}
	maxOffset := max(0, len(v.items)-v.viewportRows)
	if index > maxOffset {
		index = maxOffset
	}
	if v.scrollOffset != index {
		v.scrollOffset = index
		v.rebuildVisible()
	}
	return v
}

// ScrollBy scrolls by delta items (positive = down, negative = up).
func (v *VirtualList[T]) ScrollBy(delta int) *VirtualList[T] {
	return v.ScrollTo(v.scrollOffset + delta)
}

// ScrollOffset returns the current scroll position.
func (v *VirtualList[T]) ScrollOffset() int {
	return v.scrollOffset
}

// VisibleRange returns the range of currently visible item indices.
func (v *VirtualList[T]) VisibleRange() (start, end int) {
	start = v.scrollOffset
	end = min(v.scrollOffset+v.viewportRows, len(v.items))
	return
}

// rebuildVisible recreates only the visible components.
func (v *VirtualList[T]) rebuildVisible() {
	start, end := v.VisibleRange()
	needed := end - start

	// Resize slice if needed
	if cap(v.visibleComponents) < needed {
		v.visibleComponents = make([]Component, needed)
	} else {
		v.visibleComponents = v.visibleComponents[:needed]
	}

	// Create components for visible items only
	for i := 0; i < needed; i++ {
		itemIdx := start + i
		v.visibleComponents[i] = v.render(v.items[itemIdx], itemIdx)
	}
}

// SetConstraints implements Component.
func (v *VirtualList[T]) SetConstraints(width, height int) {
	v.Base.SetConstraints(width, height)

	// Calculate inner dimensions
	innerH := height
	innerW := width
	if v.border != nil {
		innerH -= 2
		innerW -= 2
	}
	innerH -= v.padding * 2
	innerW -= v.padding * 2

	// Clamp to reasonable values
	if innerH < 1 {
		innerH = 1
	}
	if innerW < 1 {
		innerW = 1
	}

	v.viewportHeight = innerH
	v.viewportRows = innerH / v.itemHeight
	if v.viewportRows < 1 {
		v.viewportRows = 1
	}

	// Rebuild visible components with new viewport size
	v.rebuildVisible()

	// Set constraints on visible components
	for _, comp := range v.visibleComponents {
		comp.SetConstraints(innerW, v.itemHeight)
	}

	// Store actual size
	v.width = width
	v.height = height
}

// MinSize implements Component.
func (v *VirtualList[T]) MinSize() (int, int) {
	w := 10 // minimum width
	h := v.itemHeight
	if v.border != nil {
		w += 2
		h += 2
	}
	h += v.padding * 2
	w += v.padding * 2
	return w, h
}

// Size implements Component.
func (v *VirtualList[T]) Size() (int, int) {
	return v.width, v.height
}

// Render implements Component.
func (v *VirtualList[T]) Render(buf *Buffer, x, y int) {
	// Background
	if v.background != nil {
		cell := NewCell(' ', DefaultStyle().Background(*v.background))
		buf.FillRect(x, y, v.width, v.height, cell)
	}

	// Border
	if v.border != nil {
		buf.DrawBorder(x, y, v.width, v.height, *v.border, v.style)
	}

	// Content area
	contentX := x + v.padding
	contentY := y + v.padding
	if v.border != nil {
		contentX++
		contentY++
	}

	// Render only visible components
	rowY := contentY
	for _, comp := range v.visibleComponents {
		// Bounds check - don't render outside viewport
		if rowY >= y+v.height-v.padding {
			break
		}
		if v.border != nil && rowY >= y+v.height-1-v.padding {
			break
		}

		comp.Render(buf, contentX, rowY)
		rowY += v.itemHeight
	}

	// Draw scrollbar if needed
	if len(v.items) > v.viewportRows {
		v.renderScrollbar(buf, x, y)
	}
}

// renderScrollbar draws a simple scrollbar indicator.
func (v *VirtualList[T]) renderScrollbar(buf *Buffer, x, y int) {
	sbX := x + v.width - 1
	if v.border != nil {
		sbX--
	}

	sbTop := y + v.padding
	if v.border != nil {
		sbTop++
	}
	sbHeight := v.viewportHeight

	if sbHeight < 1 || len(v.items) == 0 {
		return
	}

	// Calculate thumb position and size
	thumbSize := max(1, sbHeight*v.viewportRows/len(v.items))
	maxScroll := len(v.items) - v.viewportRows
	thumbPos := 0
	if maxScroll > 0 {
		thumbPos = (sbHeight - thumbSize) * v.scrollOffset / maxScroll
	}

	// Draw track
	trackStyle := DefaultStyle().Foreground(BrightBlack)
	for i := 0; i < sbHeight; i++ {
		buf.Set(sbX, sbTop+i, NewCell('│', trackStyle))
	}

	// Draw thumb
	thumbStyle := DefaultStyle().Foreground(White)
	for i := 0; i < thumbSize; i++ {
		buf.Set(sbX, sbTop+thumbPos+i, NewCell('┃', thumbStyle))
	}
}

// FlexGrow returns the flex grow factor (needed for stack layout).
func (v *VirtualList[T]) FlexGrow() float64 {
	return v.flexGrow
}

// --- Fluent API ---

func (v *VirtualList[T]) Border(b BorderStyle) *VirtualList[T] {
	v.border = &b
	return v
}

func (v *VirtualList[T]) Background(c Color) *VirtualList[T] {
	v.background = &c
	return v
}

func (v *VirtualList[T]) Padding(p int) *VirtualList[T] {
	v.padding = p
	return v
}

// Grow sets the flex grow factor (1 = take available space).
func (v *VirtualList[T]) Grow(factor float64) *VirtualList[T] {
	v.flexGrow = factor
	return v
}
