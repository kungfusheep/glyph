package tui

// List is a generic container for a list of components of the same type.
// It provides type-safe add/remove/access operations.
type List[T Component] struct {
	BaseContainer
	items     []T
	direction Direction

	// Optional decoration
	border     *BorderStyle
	background *Color
}

// NewList creates a new vertical list.
func NewList[T Component]() *List[T] {
	l := &List[T]{direction: Vertical}
	l.style = DefaultStyle()
	return l
}

// NewHList creates a new horizontal list.
func NewHList[T Component]() *List[T] {
	l := &List[T]{direction: Horizontal}
	l.style = DefaultStyle()
	return l
}

// Items returns all items in the list.
func (l *List[T]) Items() []T {
	return l.items
}

// Len returns the number of items.
func (l *List[T]) Len() int {
	return len(l.items)
}

// At returns the item at index i, or zero value if out of bounds.
func (l *List[T]) At(i int) T {
	if i < 0 || i >= len(l.items) {
		var zero T
		return zero
	}
	return l.items[i]
}

// Add appends items to the list.
func (l *List[T]) Add(items ...T) *List[T] {
	for _, item := range items {
		l.items = append(l.items, item)
		l.AddChild(Component(item))
	}
	return l
}

// Insert inserts an item at index i.
func (l *List[T]) Insert(i int, item T) *List[T] {
	if i < 0 {
		i = 0
	}
	if i > len(l.items) {
		i = len(l.items)
	}
	l.items = append(l.items[:i], append([]T{item}, l.items[i:]...)...)
	l.rebuildChildren()
	return l
}

// RemoveAt removes the item at index i.
func (l *List[T]) RemoveAt(i int) *List[T] {
	if i < 0 || i >= len(l.items) {
		return l
	}
	l.items = append(l.items[:i], l.items[i+1:]...)
	l.rebuildChildren()
	return l
}

// Remove removes a specific item from the list.
func (l *List[T]) Remove(item T) *List[T] {
	for i, it := range l.items {
		if Component(it) == Component(item) {
			return l.RemoveAt(i)
		}
	}
	return l
}

// Clear removes all items.
func (l *List[T]) Clear() *List[T] {
	l.items = l.items[:0]
	l.children = l.children[:0]
	return l
}

// rebuildChildren syncs the children slice with items.
func (l *List[T]) rebuildChildren() {
	l.children = l.children[:0]
	for _, item := range l.items {
		l.children = append(l.children, Component(item))
	}
}

// SetConstraints implements Component.
func (l *List[T]) SetConstraints(width, height int) {
	l.Base.SetConstraints(width, height)

	innerW := width - l.padding*2
	innerH := height - l.padding*2
	if l.border != nil {
		innerW -= 2
		innerH -= 2
	}

	// Layout children
	for _, child := range l.children {
		if l.direction == Vertical {
			child.SetConstraints(innerW, 0)
		} else {
			child.SetConstraints(0, innerH)
		}
	}

	l.calculateSize(innerW, innerH)
}

// calculateSize determines our size based on children.
func (l *List[T]) calculateSize(maxW, maxH int) {
	var totalMain, maxCross int

	for _, child := range l.children {
		w, h := child.Size()
		if l.direction == Vertical {
			totalMain += h
			if w > maxCross {
				maxCross = w
			}
		} else {
			totalMain += w
			if h > maxCross {
				maxCross = h
			}
		}
	}

	if len(l.children) > 1 {
		totalMain += l.gap * (len(l.children) - 1)
	}

	extra := l.padding * 2
	if l.border != nil {
		extra += 2
	}

	if l.direction == Vertical {
		l.width = maxCross + extra
		l.height = totalMain + extra
	} else {
		l.width = totalMain + extra
		l.height = maxCross + extra
	}

	// Expand to fill
	if maxW > 0 && l.width < maxW+extra {
		l.width = maxW + extra
	}
	if maxH > 0 && l.height < maxH+extra {
		l.height = maxH + extra
	}
}

// MinSize implements Component.
func (l *List[T]) MinSize() (int, int) {
	var totalMain, maxCross int

	for _, child := range l.children {
		w, h := child.MinSize()
		if l.direction == Vertical {
			totalMain += h
			if w > maxCross {
				maxCross = w
			}
		} else {
			totalMain += w
			if h > maxCross {
				maxCross = h
			}
		}
	}

	if len(l.children) > 1 {
		totalMain += l.gap * (len(l.children) - 1)
	}

	extra := l.padding * 2
	if l.border != nil {
		extra += 2
	}

	if l.direction == Vertical {
		return maxCross + extra, totalMain + extra
	}
	return totalMain + extra, maxCross + extra
}

// Render implements Component.
func (l *List[T]) Render(buf *Buffer, x, y int) {
	if l.background != nil {
		cell := NewCell(' ', DefaultStyle().Background(*l.background))
		buf.FillRect(x, y, l.width, l.height, cell)
	}

	if l.border != nil {
		buf.DrawBorder(x, y, l.width, l.height, *l.border, l.style)
	}

	contentX := x + l.padding
	contentY := y + l.padding
	if l.border != nil {
		contentX++
		contentY++
	}

	pos := 0
	for _, child := range l.children {
		childW, childH := child.Size()

		var childX, childY int
		if l.direction == Vertical {
			childX = contentX
			childY = contentY + pos
			pos += childH + l.gap
		} else {
			childX = contentX + pos
			childY = contentY
			pos += childW + l.gap
		}

		child.Render(buf, childX, childY)
	}
}

// --- Fluent API ---

func (l *List[T]) Gap(g int) *List[T] {
	l.gap = g
	return l
}

func (l *List[T]) Padding(p int) *List[T] {
	l.padding = p
	return l
}

func (l *List[T]) Border(b BorderStyle) *List[T] {
	l.border = &b
	return l
}

func (l *List[T]) Background(c Color) *List[T] {
	l.background = &c
	return l
}
