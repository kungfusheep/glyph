package tui

import (
	"fmt"
	"iter"
	"unsafe"
)

// Arena-based UI tree with function-only API
// Goal: Near-zero allocations per frame

// NodeKind identifies the type of node
type NodeKind uint8

const (
	NodeText NodeKind = iota
	NodeVStack
	NodeHStack
	NodeSpacer
	NodeProgress
	NodeGrid
)

// Node is a compact representation of a UI element
// Target: ~48 bytes per node (vs 128+ for typed components)
type Node struct {
	Kind       NodeKind
	Parent     int16 // index of parent (-1 for root)
	FirstChild int16 // index of first child (-1 for none)
	LastChild  int16 // index of last child (for O(1) append)
	NextSib    int16 // index of next sibling (-1 for none)

	// Layout results
	X, Y int16
	W, H int16

	// Props - meaning depends on Kind
	Prop1 int32 // TextNode: offset into text arena, ProgressNode: width, GridNode: cols
	Prop2 int32 // TextNode: length, ProgressNode: current
	Prop3 int32 // ProgressNode: total

	// Container layout props
	Gap     int8
	Padding int8
	Border  uint8 // 0=none, 1=single, 2=rounded, 3=double

	// Explicit min dimensions (0 means use content size, -1 means 0)
	MinW int16
	MinH int16

	// Progress bar chars (when set)
	FilledChar rune
	EmptyChar  rune

	// Style
	Style Style

	// Flex
	FlexGrow float32
}

// Frame holds all nodes for a single frame
// Reused across frames - just reset lengths
type Frame struct {
	nodes []Node
	text  []byte // all strings concatenated

	// Build-time state
	stack []int16 // parent stack during build
}

// Global frame - set during Build()
var currentFrame *Frame

// NewFrame creates a new frame with pre-allocated capacity
func NewFrame(nodeCapacity, textCapacity int) *Frame {
	return &Frame{
		nodes: make([]Node, 0, nodeCapacity),
		text:  make([]byte, 0, textCapacity),
		stack: make([]int16, 0, 32),
	}
}

// Reset clears the frame for reuse
func (f *Frame) Reset() {
	f.nodes = f.nodes[:0]
	f.text = f.text[:0]
	f.stack = f.stack[:0]
}

// Build executes the build function with this frame as context
func (f *Frame) Build(fn func()) {
	f.Reset()
	f.stack = append(f.stack, -1) // root parent
	currentFrame = f
	fn()
	currentFrame = nil
}

// Nodes returns the built nodes
func (f *Frame) Nodes() []Node {
	return f.nodes
}

// Children returns an iterator over a node's children
func (f *Frame) Children(idx int16) iter.Seq[*Node] {
	return func(yield func(*Node) bool) {
		for child := f.nodes[idx].FirstChild; child >= 0; child = f.nodes[child].NextSib {
			if !yield(&f.nodes[child]) {
				return
			}
		}
	}
}

// GetText returns the string for a text node
func (f *Frame) GetText(n *Node) string {
	start := n.Prop1
	length := n.Prop2
	return string(f.text[start : start+length])
}

// --- Internal helpers ---

func (f *Frame) addNode(kind NodeKind) int16 {
	idx := int16(len(f.nodes))
	parent := f.stack[len(f.stack)-1]

	f.nodes = append(f.nodes, Node{
		Kind:       kind,
		Parent:     parent,
		FirstChild: -1,
		LastChild:  -1,
		NextSib:    -1,
		Style:      DefaultStyle(),
	})

	// Link to parent (O(1) with LastChild tracking)
	if parent >= 0 {
		f.linkChild(parent, idx)
	}

	return idx
}

func (f *Frame) linkChild(parent, child int16) {
	p := &f.nodes[parent]
	if p.FirstChild < 0 {
		p.FirstChild = child
		p.LastChild = child
	} else {
		// Append to last child (O(1))
		f.nodes[p.LastChild].NextSib = child
		p.LastChild = child
	}
}

func (f *Frame) addText(s string) (offset, length int32) {
	offset = int32(len(f.text))
	f.text = append(f.text, s...)
	length = int32(len(s))
	return
}

// --- User-facing API ---

// AText creates a text node (A prefix to avoid conflict with existing Text)
func AText(s string) NodeRef {
	f := currentFrame
	idx := f.addNode(NodeText)
	offset, length := f.addText(s)
	f.nodes[idx].Prop1 = offset
	f.nodes[idx].Prop2 = length
	// Calculate min size
	f.nodes[idx].W = int16(length)
	f.nodes[idx].H = 1
	return NodeRef{idx}
}

// ATextf creates a formatted text node
// NOTE: This allocates due to fmt.Sprintf - use AText with pre-formatted strings for zero-alloc
func ATextf(format string, args ...any) {
	AText(fmt.Sprintf(format, args...))
}

// ATextInt is a zero-alloc way to render an integer
func ATextInt(n int) NodeRef {
	f := currentFrame
	idx := f.addNode(NodeText)

	// Write int directly to text arena (no allocation)
	offset := int32(len(f.text))
	f.text = appendInt(f.text, n)
	length := int32(len(f.text)) - offset

	f.nodes[idx].Prop1 = offset
	f.nodes[idx].Prop2 = length
	f.nodes[idx].W = int16(length)
	f.nodes[idx].H = 1
	return NodeRef{idx}
}

// ATextIntW renders an integer right-aligned to a fixed width (zero-alloc)
func ATextIntW(n int, width int) NodeRef {
	f := currentFrame
	idx := f.addNode(NodeText)

	offset := int32(len(f.text))
	f.text = appendIntPadded(f.text, n, width)
	length := int32(len(f.text)) - offset

	f.nodes[idx].Prop1 = offset
	f.nodes[idx].Prop2 = length
	f.nodes[idx].W = int16(length)
	f.nodes[idx].H = 1
	return NodeRef{idx}
}

// appendInt appends an integer to a byte slice without allocating
func appendInt(buf []byte, n int) []byte {
	if n == 0 {
		return append(buf, '0')
	}
	if n < 0 {
		buf = append(buf, '-')
		n = -n
	}

	// Find number of digits
	tmp := n
	digits := 0
	for tmp > 0 {
		digits++
		tmp /= 10
	}

	// Write digits in reverse
	start := len(buf)
	for i := 0; i < digits; i++ {
		buf = append(buf, 0)
	}
	for i := digits - 1; i >= 0; i-- {
		buf[start+i] = byte('0' + n%10)
		n /= 10
	}
	return buf
}

// appendIntPadded appends an integer right-aligned to a fixed width
func appendIntPadded(buf []byte, n int, width int) []byte {
	// Count digits needed
	digits := 1
	if n < 0 {
		digits++ // for minus sign
		tmp := -n
		for tmp >= 10 {
			digits++
			tmp /= 10
		}
	} else if n > 0 {
		tmp := n
		digits = 0
		for tmp > 0 {
			digits++
			tmp /= 10
		}
	}

	// Add padding spaces
	padding := width - digits
	for i := 0; i < padding; i++ {
		buf = append(buf, ' ')
	}

	// Append the number
	return appendInt(buf, n)
}

// NodeRef is a reference to a node for fluent API chaining
type NodeRef struct {
	idx int16
}

// LayoutFunc positions children within a container's inner area
// Children already have their sizes computed - layout just sets relative x,y positions
// The positions are relative to the container's inner area (0,0 is top-left of inner area)
// After the layout function runs, layoutNode applies absolute positioning
type LayoutFunc func(f *Frame, idx int16, innerW, innerH int16)

// Layout registry - maps node kinds to layout functions
var layoutFuncs = make(map[NodeKind]LayoutFunc)

// RegisterLayout registers a custom layout function
func RegisterLayout(kind NodeKind, fn LayoutFunc) {
	layoutFuncs[kind] = fn
}

// nextCustomKind for user-defined layouts
var nextCustomKind NodeKind = 100

// NewLayout creates and registers a new layout, returning its kind
func NewLayout(fn LayoutFunc) NodeKind {
	kind := nextCustomKind
	nextCustomKind++
	layoutFuncs[kind] = fn
	return kind
}

// Built-in layout functions - ONLY position children.
// Sizes are already set by measure + distribute phases.

// VerticalLayout positions children vertically
func VerticalLayout(f *Frame, idx int16, innerW, innerH int16) {
	gap, y := int16(f.nodes[idx].Gap), int16(0)
	for c := range f.Children(idx) {
		c.X, c.Y, c.W = 0, y, innerW
		y += c.H + gap
	}
}

// HorizontalLayout positions children horizontally
func HorizontalLayout(f *Frame, idx int16, innerW, innerH int16) {
	gap, x := int16(f.nodes[idx].Gap), int16(0)
	for c := range f.Children(idx) {
		c.X, c.Y, c.H = x, 0, innerH
		x += c.W + gap
	}
}

// Pre-registered layout kinds for built-in layouts
const (
	layoutKindVertical   NodeKind = 101
	layoutKindHorizontal NodeKind = 102
)

func init() {
	layoutFuncs[layoutKindVertical] = VerticalLayout
	layoutFuncs[layoutKindHorizontal] = HorizontalLayout
}

func GridLayout(cols int) LayoutFunc {
	return func(f *Frame, idx int16, innerW, innerH int16) {
		gap := int16(f.nodes[idx].Gap)

		// Count children
		count := f.childCount(idx)
		if count == 0 {
			return
		}
		rows := (count + cols - 1) / cols
		cellW := (innerW - gap*int16(cols-1)) / int16(cols)
		cellH := (innerH - gap*int16(rows-1)) / int16(rows)

		i := 0
		for c := range f.Children(idx) {
			c.X = int16(i%cols) * (cellW + gap)
			c.Y = int16(i/cols) * (cellH + gap)
			c.W, c.H = cellW, cellH
			i++
		}
	}
}

func (f *Frame) childCount(idx int16) int {
	count := 0
	for range f.Children(idx) {
		count++
	}
	return count
}

// NodeBox is a generic container kind
const NodeBox NodeKind = 10

// ABox creates a container (defaults to vertical layout)
func ABox(children ...NodeRef) NodeRef {
	f := currentFrame
	idx := f.addNode(NodeBox)
	f.nodes[idx].Prop3 = int32(NodeVStack) // default layout stored in Prop3
	for _, child := range children {
		if child.idx >= 0 {
			f.relinkChild(idx, child.idx)
		}
	}
	return NodeRef{idx}
}

// Layout sets the layout function for a Box container
func (r NodeRef) Layout(fn LayoutFunc) NodeRef {
	if r.idx < 0 || currentFrame == nil {
		return r
	}
	n := &currentFrame.nodes[r.idx]

	// Check if it's a built-in layout (avoid re-registering)
	switch {
	case funcAddr(fn) == funcAddr(VerticalLayout):
		n.Prop3 = int32(layoutKindVertical)
		n.Prop2 = 0 // vertical flex
	case funcAddr(fn) == funcAddr(HorizontalLayout):
		n.Prop3 = int32(layoutKindHorizontal)
		n.Prop2 = 1 // horizontal flex
	default:
		// Custom layout - register it, default to vertical flex
		kind := NewLayout(fn)
		n.Prop3 = int32(kind)
		n.Prop2 = 0
	}
	return r
}

// Horizontal marks this container as using horizontal flex distribution
// Use for custom horizontal layouts
func (r NodeRef) Horizontal() NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		currentFrame.nodes[r.idx].Prop2 = 1
	}
	return r
}

// funcAddr gets function address for comparison
func funcAddr(fn LayoutFunc) uintptr {
	return *(*uintptr)(unsafe.Pointer(&fn))
}

// Space sets the gap between children (alias for Gap)
func (r NodeRef) Space(s int) NodeRef {
	return r.Gap(s)
}

// Grow sets flex grow and returns self for chaining
func (r NodeRef) Grow(factor float32) NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		currentFrame.nodes[r.idx].FlexGrow = factor
	}
	return r
}

// Bold makes text bold
func (r NodeRef) Bold() NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		n := &currentFrame.nodes[r.idx]
		n.Style.Attr = n.Style.Attr.With(AttrBold)
	}
	return r
}

// Dim makes text dim
func (r NodeRef) Dim() NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		n := &currentFrame.nodes[r.idx]
		n.Style.Attr = n.Style.Attr.With(AttrDim)
	}
	return r
}

// Italic makes text italic
func (r NodeRef) Italic() NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		n := &currentFrame.nodes[r.idx]
		n.Style.Attr = n.Style.Attr.With(AttrItalic)
	}
	return r
}

// Underline makes text underlined
func (r NodeRef) Underline() NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		n := &currentFrame.nodes[r.idx]
		n.Style.Attr = n.Style.Attr.With(AttrUnderline)
	}
	return r
}

// Strikethrough makes text struck through
func (r NodeRef) Strikethrough() NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		n := &currentFrame.nodes[r.idx]
		n.Style.Attr = n.Style.Attr.With(AttrStrikethrough)
	}
	return r
}

// Inverse swaps foreground and background
func (r NodeRef) Inverse() NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		n := &currentFrame.nodes[r.idx]
		n.Style.Attr = n.Style.Attr.With(AttrInverse)
	}
	return r
}

// Color sets foreground color
func (r NodeRef) Color(c Color) NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		currentFrame.nodes[r.idx].Style.FG = c
	}
	return r
}

// Background sets background color
func (r NodeRef) Background(c Color) NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		currentFrame.nodes[r.idx].Style.BG = c
	}
	return r
}

// Width sets explicit width (for progress bars)
func (r NodeRef) Width(w int) NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		currentFrame.nodes[r.idx].Prop1 = int32(w)
		currentFrame.nodes[r.idx].W = int16(w) + 2
	}
	return r
}

// Chars sets the filled and empty characters for progress bars
func (r NodeRef) Chars(filled, empty rune) NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		n := &currentFrame.nodes[r.idx]
		n.FilledChar = filled
		n.EmptyChar = empty
	}
	return r
}

// Gap sets spacing between children in stacks
func (r NodeRef) Gap(g int) NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		currentFrame.nodes[r.idx].Gap = int8(g)
	}
	return r
}

// Padding sets internal padding for containers
func (r NodeRef) Padding(p int) NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		currentFrame.nodes[r.idx].Padding = int8(p)
	}
	return r
}

// Border sets the border style (0=none, 1=single, 2=rounded, 3=double)
func (r NodeRef) Border(style uint8) NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		currentFrame.nodes[r.idx].Border = style
	}
	return r
}

// BorderSingle sets a single-line border
func (r NodeRef) BorderSingle() NodeRef {
	return r.Border(1)
}

// BorderRounded sets a rounded border
func (r NodeRef) BorderRounded() NodeRef {
	return r.Border(2)
}

// BorderDouble sets a double-line border
func (r NodeRef) BorderDouble() NodeRef {
	return r.Border(3)
}

// MinWidth sets explicit minimum width (use 0 to ignore content width for flex)
func (r NodeRef) MinWidth(w int) NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		if w == 0 {
			currentFrame.nodes[r.idx].MinW = -1 // -1 means explicit 0
		} else {
			currentFrame.nodes[r.idx].MinW = int16(w)
		}
	}
	return r
}

// MinHeight sets explicit minimum height
func (r NodeRef) MinHeight(h int) NodeRef {
	if r.idx >= 0 && currentFrame != nil {
		if h == 0 {
			currentFrame.nodes[r.idx].MinH = -1 // -1 means explicit 0
		} else {
			currentFrame.nodes[r.idx].MinH = int16(h)
		}
	}
	return r
}

// AVStack creates a vertical stack
func AVStack(children ...NodeRef) NodeRef {
	f := currentFrame
	idx := f.addNode(NodeVStack)
	// Children are already added and linked via their own addNode calls
	// We need to re-parent them to this stack
	for _, child := range children {
		if child.idx >= 0 {
			f.relinkChild(idx, child.idx)
		}
	}
	return NodeRef{idx}
}

// AHStack creates a horizontal stack
func AHStack(children ...NodeRef) NodeRef {
	f := currentFrame
	idx := f.addNode(NodeHStack)
	for _, child := range children {
		if child.idx >= 0 {
			f.relinkChild(idx, child.idx)
		}
	}
	return NodeRef{idx}
}

// relinkChild moves a child from root to a new parent
func (f *Frame) relinkChild(newParent, child int16) {
	c := &f.nodes[child]
	oldParent := c.Parent

	// Remove from old parent's child list
	if oldParent >= 0 {
		f.unlinkChild(oldParent, child)
	}

	// Add to new parent
	c.Parent = newParent
	f.linkChild(newParent, child)
}

// unlinkChild removes a child from parent's linked list
func (f *Frame) unlinkChild(parent, child int16) {
	p := &f.nodes[parent]
	if p.FirstChild == child {
		p.FirstChild = f.nodes[child].NextSib
		if p.FirstChild < 0 {
			p.LastChild = -1
		}
	} else {
		// Find previous sibling
		prev := p.FirstChild
		for prev >= 0 && f.nodes[prev].NextSib != child {
			prev = f.nodes[prev].NextSib
		}
		if prev >= 0 {
			f.nodes[prev].NextSib = f.nodes[child].NextSib
			if p.LastChild == child {
				p.LastChild = prev
			}
		}
	}
	f.nodes[child].NextSib = -1
}

// ASpacer creates a spacer
func ASpacer() NodeRef {
	f := currentFrame
	idx := f.addNode(NodeSpacer)
	f.nodes[idx].FlexGrow = 1
	return NodeRef{idx}
}

// AProgress creates a progress bar
func AProgress(current, total int) NodeRef {
	f := currentFrame
	idx := f.addNode(NodeProgress)
	f.nodes[idx].Prop1 = 30 // width
	f.nodes[idx].Prop2 = int32(current)
	f.nodes[idx].Prop3 = int32(total)
	f.nodes[idx].W = 32 // width + brackets
	f.nodes[idx].H = 1
	f.nodes[idx].FilledChar = '█'
	f.nodes[idx].EmptyChar = '░'
	return NodeRef{idx}
}

// AGrid creates a grid layout with the specified number of columns
func AGrid(cols int, children ...NodeRef) NodeRef {
	f := currentFrame
	idx := f.addNode(NodeGrid)
	f.nodes[idx].Prop1 = int32(cols)
	for _, child := range children {
		if child.idx >= 0 {
			f.relinkChild(idx, child.idx)
		}
	}
	return NodeRef{idx}
}

// ACols2 creates a 2-column grid
func ACols2(children ...NodeRef) NodeRef {
	return AGrid(2, children...)
}

// ACols3 creates a 3-column grid
func ACols3(children ...NodeRef) NodeRef {
	return AGrid(3, children...)
}

// AFragment creates a bordered pane - building blocks for dashboards
func AFragment(children ...NodeRef) NodeRef {
	return AVStack(children...).BorderRounded().Padding(1)
}

// AWindow creates a top-level container with border and padding
func AWindow(children ...NodeRef) NodeRef {
	return AVStack(children...).BorderRounded().Padding(1).Grow(1)
}

// ATitle creates a title bar with optional right-side content
func ATitle(title string, right ...NodeRef) NodeRef {
	items := []NodeRef{AText(title).Bold(), ASpacer()}
	items = append(items, right...)
	return AHStack(items...)
}

// AMap transforms a slice into NodeRefs - the arena equivalent of Map
func AMap[T any](items []T, fn func(T, int) NodeRef) []NodeRef {
	result := make([]NodeRef, len(items))
	for i, item := range items {
		result[i] = fn(item, i)
	}
	return result
}

// --- Layout ---

// Layout calculates positions and sizes for all nodes
// Layout runs the three-phase layout cycle (like TypeScript ValidationManager):
//   Phase 1 - Measure (bottom-up): compute minimum sizes
//   Phase 2 - Distribute (top-down): distribute flex space
//   Phase 3 - Position (top-down): position children
func (f *Frame) Layout(width, height int) {
	if len(f.nodes) == 0 {
		return
	}

	w, h := int16(width), int16(height)

	// Find root nodes
	for i := range f.nodes {
		if f.nodes[i].Parent < 0 {
			idx := int16(i)
			f.measure(idx)            // Phase 1: bottom-up sizing
			f.distribute(idx, w, h)   // Phase 2: top-down flex distribution
			f.position(idx, 0, 0, w, h) // Phase 3: top-down positioning
		}
	}
}

// measure computes minimum sizes bottom-up
func (f *Frame) measure(idx int16) {
	n := &f.nodes[idx]

	// Measure children first (bottom-up)
	for c := range f.Children(idx) {
		f.measure(f.indexOf(c))
	}

	// Compute this node's size from children
	if n.MinW == 0 {
		n.W = f.measureWidth(idx)
	}
	if n.MinH == 0 {
		n.H = f.measureHeight(idx)
	}
}

// distribute assigns final sizes top-down (flex distribution)
func (f *Frame) distribute(idx int16, w, h int16) {
	n := &f.nodes[idx]
	n.W, n.H = w, h

	if n.Kind == NodeText || n.Kind == NodeProgress || n.Kind == NodeSpacer {
		return
	}

	// Calculate inner dimensions
	innerW, innerH := f.innerSize(idx, w, h)

	// Distribute flex to children
	horizontal := n.Kind == NodeHStack || (n.Kind == NodeBox && n.Prop2 == 1)
	if n.Kind != NodeGrid {
		f.distributeFlex(idx, innerW, innerH, horizontal)
	}

	// Recurse - children get their distributed sizes
	for c := range f.Children(idx) {
		if n.Kind == NodeGrid {
			// Grid cells get equal sizes (calculated in position phase)
			continue
		}
		if horizontal {
			f.distribute(f.indexOf(c), c.W, innerH)
		} else {
			f.distribute(f.indexOf(c), innerW, c.H)
		}
	}
}

// position assigns x,y coordinates top-down
func (f *Frame) position(idx int16, x, y, w, h int16) {
	n := &f.nodes[idx]
	n.X, n.Y, n.W, n.H = x, y, w, h

	if n.Kind == NodeText || n.Kind == NodeProgress || n.Kind == NodeSpacer {
		return
	}

	innerX, innerY := f.innerOrigin(idx, x, y)
	innerW, innerH := f.innerSize(idx, w, h)

	// Get layout function and position children
	layoutFn := f.getLayoutFunc(idx)
	layoutFn(f, idx, innerW, innerH)

	// Recurse with absolute positions
	for c := range f.Children(idx) {
		f.position(f.indexOf(c), innerX+c.X, innerY+c.Y, c.W, c.H)
	}
}

// Helper: get inner origin accounting for border/padding
func (f *Frame) innerOrigin(idx int16, x, y int16) (int16, int16) {
	n := &f.nodes[idx]
	if n.Border > 0 {
		x, y = x+1, y+1
	}
	p := int16(n.Padding)
	return x + p, y + p
}

// Helper: get inner size accounting for border/padding
func (f *Frame) innerSize(idx int16, w, h int16) (int16, int16) {
	n := &f.nodes[idx]
	if n.Border > 0 {
		w, h = w-2, h-2
	}
	p := int16(n.Padding) * 2
	w, h = w-p, h-p
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return w, h
}

// Helper: get layout function for a node
func (f *Frame) getLayoutFunc(idx int16) LayoutFunc {
	n := &f.nodes[idx]
	switch n.Kind {
	case NodeVStack:
		return VerticalLayout
	case NodeHStack:
		return HorizontalLayout
	case NodeGrid:
		cols := int(n.Prop1)
		if cols <= 0 {
			cols = 1
		}
		return GridLayout(cols)
	case NodeBox:
		if fn, ok := layoutFuncs[NodeKind(n.Prop3)]; ok {
			return fn
		}
	default:
		if fn, ok := layoutFuncs[n.Kind]; ok {
			return fn
		}
	}
	return VerticalLayout
}

// Helper: get node index from pointer
func (f *Frame) indexOf(n *Node) int16 {
	return int16((uintptr(unsafe.Pointer(n)) - uintptr(unsafe.Pointer(&f.nodes[0]))) / unsafe.Sizeof(Node{}))
}

// distributeFlex distributes remaining space among flex children
func (f *Frame) distributeFlex(idx int16, innerW, innerH int16, horizontal bool) {
	n := &f.nodes[idx]
	gap := int16(n.Gap)

	var totalFixed int16
	var totalFlex float32
	var count int16

	for c := range f.Children(idx) {
		if horizontal {
			totalFixed += c.W
		} else {
			totalFixed += c.H
		}
		totalFlex += c.FlexGrow
		count++
	}

	if count > 1 {
		totalFixed += gap * (count - 1)
	}

	available := innerW
	if !horizontal {
		available = innerH
	}

	remaining := available - totalFixed
	if remaining <= 0 || totalFlex == 0 {
		return
	}

	for c := range f.Children(idx) {
		if c.FlexGrow > 0 {
			extra := int16(float32(remaining) * (c.FlexGrow / totalFlex))
			if horizontal {
				c.W += extra
			} else {
				c.H += extra
			}
		}
	}
}


// measureHeight returns the minimum height needed for a node
func (f *Frame) measureHeight(idx int16) int16 {
	n := &f.nodes[idx]

	// Check for explicit minimum height
	if n.MinH != 0 {
		if n.MinH == -1 {
			return 0 // Explicit 0
		}
		return n.MinH
	}

	// Account for border and padding
	extra := int16(0)
	if n.Border > 0 {
		extra += 2
	}
	extra += int16(n.Padding) * 2

	switch n.Kind {
	case NodeText, NodeProgress:
		return 1

	case NodeVStack:
		var total int16
		gap := int16(n.Gap)
		first := true
		for child := n.FirstChild; child >= 0; child = f.nodes[child].NextSib {
			if !first {
				total += gap
			}
			first = false
			total += f.measureHeight(child)
		}
		return total + extra

	case NodeHStack:
		var maxH int16
		for child := n.FirstChild; child >= 0; child = f.nodes[child].NextSib {
			h := f.measureHeight(child)
			if h > maxH {
				maxH = h
			}
		}
		return maxH + extra

	case NodeGrid:
		cols := int(n.Prop1)
		if cols <= 0 {
			cols = 1
		}
		gap := int16(n.Gap)

		// Count children and find max cell height
		var childCount int
		var maxCellH int16
		for child := n.FirstChild; child >= 0; child = f.nodes[child].NextSib {
			childCount++
			h := f.measureHeight(child)
			if h > maxCellH {
				maxCellH = h
			}
		}

		rows := (childCount + cols - 1) / cols
		if rows == 0 {
			return extra
		}

		totalH := maxCellH*int16(rows) + gap*int16(rows-1)
		return totalH + extra

	case NodeBox:
		// Box measures based on its layout strategy (stored in Prop3)
		layoutKind := NodeKind(n.Prop3)
		switch layoutKind {
		case NodeHStack:
			var maxH int16
			for child := n.FirstChild; child >= 0; child = f.nodes[child].NextSib {
				h := f.measureHeight(child)
				if h > maxH {
					maxH = h
				}
			}
			return maxH + extra
		default: // VStack is default
			var total int16
			gap := int16(n.Gap)
			first := true
			for child := n.FirstChild; child >= 0; child = f.nodes[child].NextSib {
				if !first {
					total += gap
				}
				first = false
				total += f.measureHeight(child)
			}
			return total + extra
		}

	case NodeSpacer:
		return 0
	}
	return 0
}

// measureWidth returns the minimum width needed for a node
func (f *Frame) measureWidth(idx int16) int16 {
	n := &f.nodes[idx]

	// Check for explicit minimum width
	if n.MinW != 0 {
		if n.MinW == -1 {
			return 0 // Explicit 0
		}
		return n.MinW
	}

	// Account for border and padding
	extra := int16(0)
	if n.Border > 0 {
		extra += 2
	}
	extra += int16(n.Padding) * 2

	switch n.Kind {
	case NodeText:
		return int16(n.Prop2) // length stored in Prop2

	case NodeProgress:
		return int16(n.Prop1) + 2 // width + brackets

	case NodeHStack:
		var total int16
		gap := int16(n.Gap)
		first := true
		for child := n.FirstChild; child >= 0; child = f.nodes[child].NextSib {
			if !first {
				total += gap
			}
			first = false
			total += f.measureWidth(child)
		}
		return total + extra

	case NodeVStack:
		var maxW int16
		for child := n.FirstChild; child >= 0; child = f.nodes[child].NextSib {
			w := f.measureWidth(child)
			if w > maxW {
				maxW = w
			}
		}
		return maxW + extra

	case NodeGrid:
		cols := int(n.Prop1)
		if cols <= 0 {
			cols = 1
		}
		gap := int16(n.Gap)

		// Find max cell width
		var maxCellW int16
		for child := n.FirstChild; child >= 0; child = f.nodes[child].NextSib {
			w := f.measureWidth(child)
			if w > maxCellW {
				maxCellW = w
			}
		}

		totalW := maxCellW*int16(cols) + gap*int16(cols-1)
		return totalW + extra

	case NodeBox:
		// Box measures based on its layout strategy (stored in Prop3)
		layoutKind := NodeKind(n.Prop3)
		switch layoutKind {
		case NodeHStack:
			var total int16
			gap := int16(n.Gap)
			first := true
			for child := n.FirstChild; child >= 0; child = f.nodes[child].NextSib {
				if !first {
					total += gap
				}
				first = false
				total += f.measureWidth(child)
			}
			return total + extra
		default: // VStack is default
			var maxW int16
			for child := n.FirstChild; child >= 0; child = f.nodes[child].NextSib {
				w := f.measureWidth(child)
				if w > maxW {
					maxW = w
				}
			}
			return maxW + extra
		}

	case NodeSpacer:
		return 0
	}
	return 0
}

// --- Render ---

// Render draws all nodes to the buffer
func (f *Frame) Render(buf *Buffer) {
	for i := range f.nodes {
		f.renderNode(buf, int16(i))
	}
}

func (f *Frame) renderNode(buf *Buffer, idx int16) {
	n := &f.nodes[idx]
	x, y := int(n.X), int(n.Y)
	w, h := int(n.W), int(n.H)

	// Render background if set
	if n.Style.BG.Mode != ColorDefault {
		for dy := 0; dy < h; dy++ {
			for dx := 0; dx < w; dx++ {
				cell := buf.Get(x+dx, y+dy)
				cell.Style.BG = n.Style.BG
				buf.Set(x+dx, y+dy, cell)
			}
		}
	}

	// Render border if set
	if n.Border > 0 {
		var border BorderStyle
		switch n.Border {
		case 1:
			border = BorderSingle
		case 2:
			border = BorderRounded
		case 3:
			border = BorderDouble
		default:
			border = BorderSingle
		}
		buf.DrawBorder(x, y, w, h, border, n.Style)
	}

	switch n.Kind {
	case NodeText:
		text := f.GetText(n)
		buf.WriteStringClipped(x, y, text, n.Style, int(n.W))

	case NodeProgress:
		width := int(n.Prop1)
		current := int(n.Prop2)
		total := int(n.Prop3)
		if total == 0 {
			total = 1
		}
		filled := (current * width) / total
		if filled > width {
			filled = width
		}

		filledChar := n.FilledChar
		emptyChar := n.EmptyChar
		if filledChar == 0 {
			filledChar = '█'
		}
		if emptyChar == 0 {
			emptyChar = '░'
		}

		buf.Set(x, y, NewCell('[', n.Style))
		for i := 0; i < width; i++ {
			r := emptyChar
			if i < filled {
				r = filledChar
			}
			buf.Set(x+1+i, y, NewCell(r, n.Style))
		}
		buf.Set(x+1+width, y, NewCell(']', n.Style))

	// VStack, HStack, Grid, Spacer don't render content themselves (just containers)
	}
}
