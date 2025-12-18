package tui

// Component is the interface all UI components implement.
type Component interface {
	// Layout
	SetConstraints(width, height int) // Parent tells us available space
	MinSize() (width, height int)     // Minimum size we need
	Size() (width, height int)        // Our actual size after layout

	// Hierarchy
	Parent() Container
	SetParent(Container)

	// Rendering
	Render(buf *Buffer, x, y int)

	// Styling
	GetStyle() Style
	SetStyle(Style)
}

// Container is a component that can hold children.
type Container interface {
	Component
	Children() []Component
	Add(children ...Component) Container
	Remove(child Component)
	Clear()
}

// Base provides common functionality for all components.
// Embed this in your component structs.
type Base struct {
	parent          Container
	style           Style
	width, height   int // Actual size
	minW, minH      int // Minimum size
	maxW, maxH      int // Maximum size (0 = unconstrained)
	constraintW     int // Available width from parent
	constraintH     int // Available height from parent

	// Flexible sizing
	flexGrow   float64 // How much to grow (0 = don't grow)
	flexShrink float64 // How much to shrink (1 = can shrink)
}

// Parent returns the parent container.
func (b *Base) Parent() Container {
	return b.parent
}

// SetParent sets the parent container.
func (b *Base) SetParent(p Container) {
	b.parent = p
}

// GetStyle returns the component's style.
func (b *Base) GetStyle() Style {
	return b.style
}

// SetStyle sets the component's style.
func (b *Base) SetStyle(s Style) {
	b.style = s
}

// SetConstraints is called by parent to tell us available space.
func (b *Base) SetConstraints(width, height int) {
	b.constraintW = width
	b.constraintH = height
}

// Constraints returns the current constraints.
func (b *Base) Constraints() (width, height int) {
	return b.constraintW, b.constraintH
}

// MinSize returns the minimum size needed.
func (b *Base) MinSize() (int, int) {
	return b.minW, b.minH
}

// Size returns the actual size.
func (b *Base) Size() (int, int) {
	return b.width, b.height
}

// SetSize sets the actual size.
func (b *Base) SetSize(w, h int) {
	b.width = w
	b.height = h
}

// SetMinSize sets the minimum size.
func (b *Base) SetMinSize(w, h int) {
	b.minW = w
	b.minH = h
}

// SetMaxSize sets the maximum size (0 = unconstrained).
func (b *Base) SetMaxSize(w, h int) {
	b.maxW = w
	b.maxH = h
}

// FlexGrow returns the flex grow factor.
func (b *Base) FlexGrow() float64 {
	return b.flexGrow
}

// SetFlexGrow sets the flex grow factor.
func (b *Base) SetFlexGrow(f float64) {
	b.flexGrow = f
}

// FlexShrink returns the flex shrink factor.
func (b *Base) FlexShrink() float64 {
	return b.flexShrink
}

// SetFlexShrink sets the flex shrink factor.
func (b *Base) SetFlexShrink(f float64) {
	b.flexShrink = f
}

// BaseContainer provides common functionality for containers.
// Embed this in container structs.
type BaseContainer struct {
	Base
	children []Component

	// Layout properties
	gap     int // Space between children
	padding int // Padding inside container
}

// Children returns the child components.
func (c *BaseContainer) Children() []Component {
	return c.children
}

// AddChild adds a single child to the container.
// Concrete container types should wrap this with their own Add method.
func (c *BaseContainer) AddChild(child Component) {
	c.children = append(c.children, child)
}

// AddChildren adds multiple children to the container.
func (c *BaseContainer) AddChildren(children ...Component) {
	c.children = append(c.children, children...)
}

// Remove removes a child from the container.
func (c *BaseContainer) Remove(child Component) {
	for i, ch := range c.children {
		if ch == child {
			child.SetParent(nil)
			c.children = append(c.children[:i], c.children[i+1:]...)
			return
		}
	}
}

// Clear removes all children.
func (c *BaseContainer) Clear() {
	for _, child := range c.children {
		child.SetParent(nil)
	}
	c.children = c.children[:0]
}

// Gap returns the gap between children.
func (c *BaseContainer) Gap() int {
	return c.gap
}

// SetGap sets the gap between children.
func (c *BaseContainer) SetGap(g int) {
	c.gap = g
}

// Padding returns the padding.
func (c *BaseContainer) Padding() int {
	return c.padding
}

// SetPadding sets the padding.
func (c *BaseContainer) SetPadding(p int) {
	c.padding = p
}
