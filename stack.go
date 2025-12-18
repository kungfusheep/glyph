package tui

import (
	"iter"
	"sync"
)

// Direction specifies the layout direction.
type Direction int

const (
	Vertical Direction = iota
	Horizontal
)

// StackComponent arranges children in a line (vertical or horizontal).
type StackComponent struct {
	BaseContainer
	direction Direction

	// Optional decoration
	border     *BorderStyle
	background *Color
}

// Pool for StackComponent reuse
var stackPool = sync.Pool{
	New: func() any { return &StackComponent{} },
}

// ChildItem is something that can be added to a stack.
// Can be a Component or an iterator of Components.
type ChildItem interface{}

// VStack creates a vertical stack from children.
// Children can be Components or iter.Seq[Component] for dynamic lists.
func VStack(items ...ChildItem) *StackComponent {
	s := stackPool.Get().(*StackComponent)
	s.Reset()
	s.direction = Vertical
	s.style = DefaultStyle()
	if cap(s.children) < len(items) {
		s.children = make([]Component, 0, len(items))
	}
	s.addItems(items)
	return s
}

// HStack creates a horizontal stack from children.
// Children can be Components or iter.Seq[Component] for dynamic lists.
func HStack(items ...ChildItem) *StackComponent {
	s := stackPool.Get().(*StackComponent)
	s.Reset()
	s.direction = Horizontal
	s.style = DefaultStyle()
	if cap(s.children) < len(items) {
		s.children = make([]Component, 0, len(items))
	}
	s.addItems(items)
	return s
}

// Reset clears the component for reuse.
func (s *StackComponent) Reset() {
	s.children = s.children[:0] // Keep capacity
	s.Base = Base{}
	s.gap = 0
	s.padding = 0
	s.direction = Vertical
	s.border = nil
	s.background = nil
}

// addItems processes a mix of Components and iterators.
func (s *StackComponent) addItems(items []ChildItem) {
	for _, item := range items {
		switch v := item.(type) {
		case Component:
			v.SetParent(s)
			s.AddChild(v)
		case iter.Seq[Component]:
			for child := range v {
				child.SetParent(s)
				s.AddChild(child)
			}
		case func(yield func(Component) bool):
			// Also accept raw iterator functions
			for child := range v {
				child.SetParent(s)
				s.AddChild(child)
			}
		}
	}
}

// Add adds children to the stack. Returns self for chaining.
func (s *StackComponent) Add(children ...Component) Container {
	for _, child := range children {
		child.SetParent(s)
		s.AddChild(child)
	}
	return s
}

// AddFrom adds children from an iterator.
func (s *StackComponent) AddFrom(seq iter.Seq[Component]) *StackComponent {
	for child := range seq {
		child.SetParent(s)
		s.AddChild(child)
	}
	return s
}

// SetConstraints implements Component.
func (s *StackComponent) SetConstraints(width, height int) {
	s.Base.SetConstraints(width, height)

	// Account for padding and border
	innerW := width - s.padding*2
	innerH := height - s.padding*2
	if s.border != nil {
		innerW -= 2
		innerH -= 2
	}

	// First pass: get children's minimum sizes (no SetConstraints yet!)
	totalFixed := 0
	totalFlex := 0.0
	childSizes := make([]struct{ w, h int }, len(s.children))

	for i, child := range s.children {
		// Just get min size - don't trigger full layout
		w, h := child.MinSize()
		childSizes[i].w = w
		childSizes[i].h = h

		if s.direction == Vertical {
			totalFixed += h
		} else {
			totalFixed += w
		}

		// Check for flex grow
		if base, ok := child.(*SpacerComponent); ok {
			totalFlex += base.FlexGrow()
		} else if grower, ok := child.(interface{ FlexGrow() float64 }); ok {
			totalFlex += grower.FlexGrow()
		}
	}

	// Add gaps
	if len(s.children) > 1 {
		totalFixed += s.gap * (len(s.children) - 1)
	}

	// Calculate remaining space for flex items
	var remaining int
	if s.direction == Vertical {
		remaining = innerH - totalFixed
	} else {
		remaining = innerW - totalFixed
	}
	if remaining < 0 {
		remaining = 0
	}

	// Second pass: distribute remaining space to flex items
	if totalFlex > 0 && remaining > 0 {
		for i, child := range s.children {
			var flex float64
			if base, ok := child.(*SpacerComponent); ok {
				flex = base.FlexGrow()
			} else if grower, ok := child.(interface{ FlexGrow() float64 }); ok {
				flex = grower.FlexGrow()
			}

			if flex > 0 {
				extra := int(float64(remaining) * (flex / totalFlex))
				if s.direction == Vertical {
					childSizes[i].h += extra
				} else {
					childSizes[i].w += extra
				}
			}
		}
	}

	// Third pass: apply final sizes to children
	for i, child := range s.children {
		if s.direction == Vertical {
			child.SetConstraints(innerW, childSizes[i].h)
		} else {
			child.SetConstraints(childSizes[i].w, innerH)
		}
	}

	// Calculate our own size
	s.calculateSize(innerW, innerH)
}

// calculateSize determines our size based on children.
func (s *StackComponent) calculateSize(maxW, maxH int) {
	var totalMain, maxCross int

	for _, child := range s.children {
		w, h := child.Size()
		if s.direction == Vertical {
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

	// Add gaps
	if len(s.children) > 1 {
		totalMain += s.gap * (len(s.children) - 1)
	}

	// Apply padding and border
	extra := s.padding * 2
	if s.border != nil {
		extra += 2
	}

	if s.direction == Vertical {
		s.width = maxCross + extra
		s.height = totalMain + extra
	} else {
		s.width = totalMain + extra
		s.height = maxCross + extra
	}

	// Expand to fill available space if we have flex children
	if s.direction == Vertical {
		if maxH > 0 && s.height < maxH+extra {
			s.height = maxH + extra
		}
		if maxW > 0 && s.width < maxW+extra {
			s.width = maxW + extra
		}
	} else {
		if maxW > 0 && s.width < maxW+extra {
			s.width = maxW + extra
		}
		if maxH > 0 && s.height < maxH+extra {
			s.height = maxH + extra
		}
	}

	// Update minimum size
	s.minW = s.width
	s.minH = s.height
}

// MinSize implements Component.
func (s *StackComponent) MinSize() (int, int) {
	// Quick calculation of minimum size
	var totalMain, maxCross int

	for _, child := range s.children {
		w, h := child.MinSize()
		if s.direction == Vertical {
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

	if len(s.children) > 1 {
		totalMain += s.gap * (len(s.children) - 1)
	}

	extra := s.padding * 2
	if s.border != nil {
		extra += 2
	}

	if s.direction == Vertical {
		return maxCross + extra, totalMain + extra
	}
	return totalMain + extra, maxCross + extra
}

// Render implements Component.
func (s *StackComponent) Render(buf *Buffer, x, y int) {
	// Draw background
	if s.background != nil {
		cell := NewCell(' ', DefaultStyle().Background(*s.background))
		buf.FillRect(x, y, s.width, s.height, cell)
	}

	// Draw border
	if s.border != nil {
		buf.DrawBorder(x, y, s.width, s.height, *s.border, s.style)
	}

	// Calculate content area
	contentX := x + s.padding
	contentY := y + s.padding
	if s.border != nil {
		contentX++
		contentY++
	}

	// Render children
	pos := 0
	for _, child := range s.children {
		childW, childH := child.Size()

		var childX, childY int
		if s.direction == Vertical {
			childX = contentX
			childY = contentY + pos
			pos += childH + s.gap
		} else {
			childX = contentX + pos
			childY = contentY
			pos += childW + s.gap
		}

		child.Render(buf, childX, childY)
	}
}

// --- Fluent API ---

// Gap sets the gap between children.
func (s *StackComponent) Gap(g int) *StackComponent {
	s.gap = g
	return s
}

// Padding sets the padding inside the stack.
func (s *StackComponent) Padding(p int) *StackComponent {
	s.padding = p
	return s
}

// Border adds a border around the stack.
func (s *StackComponent) Border(b BorderStyle) *StackComponent {
	s.border = &b
	return s
}

// Background sets the background color.
func (s *StackComponent) Background(c Color) *StackComponent {
	s.background = &c
	return s
}

// Color sets the foreground color (for borders, etc).
func (s *StackComponent) Color(c Color) *StackComponent {
	s.style.FG = c
	return s
}

// Grow sets the flex grow factor.
func (s *StackComponent) Grow(factor float64) *StackComponent {
	s.flexGrow = factor
	return s
}

// Ref stores a reference to this component.
func (s *StackComponent) Ref(ref **StackComponent) *StackComponent {
	*ref = s
	return s
}

// --- Iterator helpers ---

// Map transforms a slice into a component iterator.
func Map[T any](items []T, fn func(T) Component) iter.Seq[Component] {
	return func(yield func(Component) bool) {
		for _, item := range items {
			if !yield(fn(item)) {
				return
			}
		}
	}
}

// MapIndex transforms a slice into a component iterator with index.
func MapIndex[T any](items []T, fn func(int, T) Component) iter.Seq[Component] {
	return func(yield func(Component) bool) {
		for i, item := range items {
			if !yield(fn(i, item)) {
				return
			}
		}
	}
}

// Filter filters an iterator.
func Filter[T any](items []T, pred func(T) bool, fn func(T) Component) iter.Seq[Component] {
	return func(yield func(Component) bool) {
		for _, item := range items {
			if pred(item) {
				if !yield(fn(item)) {
					return
				}
			}
		}
	}
}
