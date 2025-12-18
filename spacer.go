package tui

import "sync"

// SpacerComponent is a flexible empty space that expands to fill available room.
type SpacerComponent struct {
	Base
}

// Pool for SpacerComponent reuse
var spacerPool = sync.Pool{
	New: func() any { return &SpacerComponent{} },
}

// Spacer creates a new spacer that expands to fill available space.
func Spacer() *SpacerComponent {
	s := spacerPool.Get().(*SpacerComponent)
	s.Reset()
	s.flexGrow = 1 // Spacers grow by default
	return s
}

// FixedSpacer creates a spacer with a fixed size.
func FixedSpacer(size int) *SpacerComponent {
	s := spacerPool.Get().(*SpacerComponent)
	s.Reset()
	s.minW = size
	s.minH = size
	s.flexGrow = 0 // Fixed spacers don't grow
	return s
}

// Reset clears the component for reuse.
func (s *SpacerComponent) Reset() {
	*s = SpacerComponent{}
}

// SetConstraints implements Component.
func (s *SpacerComponent) SetConstraints(width, height int) {
	s.Base.SetConstraints(width, height)

	// Spacer takes whatever space it's given, respecting minimums
	s.width = width
	if s.width < s.minW {
		s.width = s.minW
	}

	s.height = height
	if s.height < s.minH {
		s.height = s.minH
	}
}

// MinSize implements Component.
func (s *SpacerComponent) MinSize() (int, int) {
	return s.minW, s.minH
}

// Render implements Component.
// Spacer is invisible - nothing to render.
func (s *SpacerComponent) Render(buf *Buffer, x, y int) {
	// Spacers are invisible
}

// --- Fluent API ---

// Min sets the minimum size in both dimensions.
func (s *SpacerComponent) Min(size int) *SpacerComponent {
	s.minW = size
	s.minH = size
	return s
}

// MinWidth sets the minimum width.
func (s *SpacerComponent) MinWidth(w int) *SpacerComponent {
	s.minW = w
	return s
}

// MinHeight sets the minimum height.
func (s *SpacerComponent) MinHeight(h int) *SpacerComponent {
	s.minH = h
	return s
}

// Grow sets the flex grow factor.
func (s *SpacerComponent) Grow(factor float64) *SpacerComponent {
	s.flexGrow = factor
	return s
}
