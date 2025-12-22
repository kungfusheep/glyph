package tui

import (
	"reflect"
)

// --- Event Handlers ---

type EventType uint8

const (
	EventClick EventType = iota
	EventKey
	EventFocus
	EventBlur
	EventChange
	EventSubmit
)

type Event struct {
	Type    EventType
	Key     rune
	KeyName string // "Enter", "Tab", "Escape", etc.
	Target  int16  // node index
}

// --- Custom Component Interface ---

// DComponent is an interface that custom components can implement
// to participate in the declarative UI system.
type DComponent interface {
	// DeclRender returns the component's UI definition
	// It can return any declarative type (DCol, DRow, DText, etc.)
	DeclRender() any
}

// --- Styling ---

// DStyle represents styling options for declarative components
type DStyle struct {
	FG        *Color // Foreground color
	BG        *Color // Background color
	Bold      bool
	Dim       bool
	Italic    bool
	Underline bool
	Inverse   bool
}

// ToStyle converts DStyle to the rendering Style
func (ds DStyle) ToStyle() Style {
	s := Style{}
	if ds.FG != nil {
		s.FG = *ds.FG
	}
	if ds.BG != nil {
		s.BG = *ds.BG
	}
	if ds.Bold {
		s.Attr |= AttrBold
	}
	if ds.Dim {
		s.Attr |= AttrDim
	}
	if ds.Italic {
		s.Attr |= AttrItalic
	}
	if ds.Underline {
		s.Attr |= AttrUnderline
	}
	if ds.Inverse {
		s.Attr |= AttrInverse
	}
	return s
}

// ColorPtr returns a pointer to a color for use in DStyle
// Usage: DStyle{FG: ColorPtr(Red), BG: ColorPtr(Blue)}
func ColorPtr(c Color) *Color { return &c }

// --- Declarative UI Components ---

type DText struct {
	Content   any    // string, *string, or func() string
	Bold      bool   // shorthand for Style.Bold
	Style     DStyle // full styling options
	OnClick   func()
	Focusable bool
}

type DProgress struct {
	Value any // int, *int, or func() int (0-100)
	Width int16
}

// --- Interactive Components ---

type DButton struct {
	Label     any // string, *string, or func() string
	OnClick   func()
	OnKey     func(key string)
	Disabled  any // bool, *bool, or func() bool
}

type DInput struct {
	Value      *string
	Placeholder string
	Width      int16
	OnChange   func(value string)
	OnSubmit   func()
	OnKey      func(key string) bool // return true to prevent default
}

type DCheckbox struct {
	Checked  *bool
	Label    any
	OnChange func(checked bool)
}

type DSelect struct {
	Value    *string
	Options  []string
	OnChange func(value string)
}

type DRow struct {
	Children []any
	Gap      int8
}

type DCol struct {
	Children []any
	Gap      int8
}

// DScroll creates a scrollable viewport with viewport culling
// Only visible items are rendered - off-screen items are skipped entirely
type DScroll struct {
	Children   []any
	Height     int16 // viewport height in rows (required)
	ItemHeight int16 // height per item (default: 1)
	Offset     *int  // pointer to scroll offset for external binding
	OnScroll   func(offset int)
}

// DScrollState holds scroll state for external control
type DScrollState struct {
	Offset      int
	ItemCount   int
	ViewportRows int
}

// ScrollBy adjusts offset by delta and clamps to valid range
func (s *DScrollState) ScrollBy(delta int) {
	s.Offset += delta
	s.clamp()
}

// ScrollTo sets offset to a specific value and clamps
func (s *DScrollState) ScrollTo(offset int) {
	s.Offset = offset
	s.clamp()
}

func (s *DScrollState) clamp() {
	maxOffset := s.ItemCount - s.ViewportRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if s.Offset > maxOffset {
		s.Offset = maxOffset
	}
	if s.Offset < 0 {
		s.Offset = 0
	}
}

// VisibleRange returns start and end indices of visible items
func (s *DScrollState) VisibleRange() (start, end int) {
	start = s.Offset
	end = s.Offset + s.ViewportRows
	if end > s.ItemCount {
		end = s.ItemCount
	}
	return
}

// DScrollView creates a scrollable viewport by rendering content to an off-screen
// buffer, then displaying a slice based on scroll offset. Unlike DScroll which
// does item-based viewport culling, DScrollView supports variable-height content
// and row-based scrolling - ideal for long-form content like web pages.
type DScrollView struct {
	Content any    // Child component(s) to render
	Height  int16  // Viewport height in rows
	Width   int16  // Viewport width (0 = use available width)
	Offset  *int   // Row offset into content (pointer for external binding)
}

// DScrollViewState provides external control over a scroll view
type DScrollViewState struct {
	Offset        int
	ContentHeight int
	ViewportHeight int
}

// ScrollBy adjusts offset by delta and clamps to valid range
func (s *DScrollViewState) ScrollBy(delta int) {
	s.Offset += delta
	s.clamp()
}

// ScrollTo sets offset to a specific row and clamps
func (s *DScrollViewState) ScrollTo(offset int) {
	s.Offset = offset
	s.clamp()
}

// ScrollToBottom scrolls to the end of content
func (s *DScrollViewState) ScrollToBottom() {
	s.Offset = s.ContentHeight - s.ViewportHeight
	s.clamp()
}

func (s *DScrollViewState) clamp() {
	maxOffset := s.ContentHeight - s.ViewportHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if s.Offset > maxOffset {
		s.Offset = maxOffset
	}
	if s.Offset < 0 {
		s.Offset = 0
	}
}

// MaxScroll returns the maximum valid scroll offset
func (s *DScrollViewState) MaxScroll() int {
	max := s.ContentHeight - s.ViewportHeight
	if max < 0 {
		return 0
	}
	return max
}

// --- Control Flow ---

type ifState struct {
	satisfied bool
}

type DIfNode struct {
	Cond any // bool, *bool, or func() bool
	Then any
}

type DElseIfNode struct {
	Cond any
	Then any
}

type DElseNode struct {
	Then any
}

func DIf(cond any, then any) DIfNode {
	return DIfNode{Cond: cond, Then: then}
}

func DElseIf(cond any, then any) DElseIfNode {
	return DElseIfNode{Cond: cond, Then: then}
}

func DElse(then any) DElseNode {
	return DElseNode{Then: then}
}

// --- Switch/Case ---

type DSwitchNode struct {
	Value any // the value to switch on
	Cases []any
}

type DCaseNode struct {
	Match any
	Then  any
}

type DDefaultNode struct {
	Then any
}

func DSwitch(value any, cases ...any) DSwitchNode {
	return DSwitchNode{Value: value, Cases: cases}
}

func DCase(match any, then any) DCaseNode {
	return DCaseNode{Match: match, Then: then}
}

func DDefault(then any) DDefaultNode {
	return DDefaultNode{Then: then}
}

// --- ForEach ---

type DForEachNode struct {
	Items  any // *[]T or func() []T
	Render any // func(*T) any or func(T) any
}

func DForEach(items any, render any) DForEachNode {
	return DForEachNode{Items: items, Render: render}
}

// --- Executor ---

type DFrame struct {
	nodes      []DLayoutNode
	focusables []int16 // indices of focusable nodes in tab order
	focusIndex int     // current focus position in focusables

	// Jump label support
	jumpMode    bool
	jumpTargets []JumpTarget

	// Nested focus context support (for modals/dialogs)
	focusContexts []FocusContext
}

// FocusContext represents a saved focus state for nested contexts
type FocusContext struct {
	FocusIndex int     // saved focus index
	StartIdx   int16   // first focusable index in this context
	EndIdx     int16   // last focusable index in this context (exclusive)
}

// JumpTarget represents an interactive element that can be jumped to
type JumpTarget struct {
	NodeIdx int16
	X, Y    int16  // position for label overlay
	Label   string // assigned jump label
}

type DLayoutNode struct {
	X, Y, W, H int16
	Type       string
	Text       string
	Bold       bool
	Style      Style // computed style for rendering
	Ratio      float32
	IsRow      bool
	Gap        int8
	FirstChild int16
	NextSib    int16
	Parent     int16

	// Interactivity
	Focusable bool
	Disabled  bool
	OnClick   func()
	OnKey     func(key string) bool
	OnChange  func(value string)
	OnSubmit  func()

	// Input state
	InputValue *string
	CursorPos  int

	// Scroll state (for scroll containers)
	ScrollOffset   int16 // first visible item index
	ScrollTotal    int16 // total item count
	ScrollViewport int16 // visible item count

	// ScrollView buffer (for render-to-buffer scrolling)
	ScrollBuffer       *Buffer // off-screen rendered content
	ScrollViewParent   int16   // if >= 0, this node is inside a scrollview (skip in main render)
}

func NewDFrame() *DFrame {
	return &DFrame{
		nodes:      make([]DLayoutNode, 0, 256),
		focusables: make([]int16, 0, 32),
	}
}

func (f *DFrame) Reset() {
	f.nodes = f.nodes[:0]
	f.focusables = f.focusables[:0]
	// Note: focusIndex preserved across resets to maintain focus
}

// Focus returns the currently focused node index (-1 if none)
func (f *DFrame) Focus() int16 {
	if len(f.focusables) == 0 || f.focusIndex >= len(f.focusables) {
		return -1
	}
	return f.focusables[f.focusIndex]
}

// FocusNext moves focus to next focusable within current context
func (f *DFrame) FocusNext() {
	if len(f.focusables) == 0 {
		return
	}
	start, end := f.focusBounds()
	count := end - start
	if count <= 0 {
		return
	}
	// Calculate position within context and wrap
	pos := f.focusIndex - start
	pos = (pos + 1) % count
	f.focusIndex = start + pos
}

// FocusPrev moves focus to previous focusable within current context
func (f *DFrame) FocusPrev() {
	if len(f.focusables) == 0 {
		return
	}
	start, end := f.focusBounds()
	count := end - start
	if count <= 0 {
		return
	}
	// Calculate position within context and wrap
	pos := f.focusIndex - start
	pos = (pos - 1 + count) % count
	f.focusIndex = start + pos
}

// Activate triggers the focused element's OnClick
func (f *DFrame) Activate() {
	if node := f.FocusedNode(); node != nil && node.OnClick != nil && !node.Disabled {
		node.OnClick()
	}
}

// ActivateNode triggers OnClick on a specific node by index
func (f *DFrame) ActivateNode(idx int16) {
	if idx >= 0 && int(idx) < len(f.nodes) {
		node := &f.nodes[idx]
		if node.OnClick != nil && !node.Disabled {
			node.OnClick()
		}
	}
}

// FocusedNode returns the currently focused node
func (f *DFrame) FocusedNode() *DLayoutNode {
	idx := f.Focus()
	if idx < 0 {
		return nil
	}
	return &f.nodes[idx]
}

// IsFocused checks if a node is focused
func (f *DFrame) IsFocused(idx int16) bool {
	return f.Focus() == idx
}

// --- Nested Focus Context Methods ---

// PushFocusContext creates a new focus context starting from the current focus position
// This is used for modals/dialogs to trap focus within a subset of focusables
func (f *DFrame) PushFocusContext() {
	if len(f.focusables) == 0 {
		return
	}

	// Context starts from current focus position to end
	startIdx := int16(f.focusIndex)

	ctx := FocusContext{
		FocusIndex: f.focusIndex,
		StartIdx:   startIdx,
		EndIdx:     int16(len(f.focusables)),
	}
	f.focusContexts = append(f.focusContexts, ctx)
}

// PopFocusContext restores the previous focus context
func (f *DFrame) PopFocusContext() {
	if len(f.focusContexts) == 0 {
		return
	}

	ctx := f.focusContexts[len(f.focusContexts)-1]
	f.focusContexts = f.focusContexts[:len(f.focusContexts)-1]
	f.focusIndex = ctx.FocusIndex
}

// FocusContextDepth returns the number of nested focus contexts
func (f *DFrame) FocusContextDepth() int {
	return len(f.focusContexts)
}

// focusBounds returns the current context's valid focus range
func (f *DFrame) focusBounds() (start, end int) {
	if len(f.focusContexts) == 0 {
		return 0, len(f.focusables)
	}
	ctx := f.focusContexts[len(f.focusContexts)-1]
	return int(ctx.StartIdx), int(ctx.EndIdx)
}

// --- Text Input Methods ---

// InputInsert inserts a character at the cursor position in the focused input
func (f *DFrame) InputInsert(ch rune) bool {
	node := f.FocusedNode()
	if node == nil || node.Type != "input" || node.InputValue == nil {
		return false
	}

	val := *node.InputValue
	runes := []rune(val)

	// Clamp cursor to valid range
	if node.CursorPos < 0 {
		node.CursorPos = 0
	}
	if node.CursorPos > len(runes) {
		node.CursorPos = len(runes)
	}

	// Insert character at cursor
	newRunes := make([]rune, 0, len(runes)+1)
	newRunes = append(newRunes, runes[:node.CursorPos]...)
	newRunes = append(newRunes, ch)
	newRunes = append(newRunes, runes[node.CursorPos:]...)

	*node.InputValue = string(newRunes)
	node.CursorPos++

	// Trigger OnChange
	if node.OnChange != nil {
		node.OnChange(*node.InputValue)
	}

	return true
}

// InputBackspace deletes the character before the cursor
func (f *DFrame) InputBackspace() bool {
	node := f.FocusedNode()
	if node == nil || node.Type != "input" || node.InputValue == nil {
		return false
	}

	val := *node.InputValue
	runes := []rune(val)

	if node.CursorPos <= 0 || len(runes) == 0 {
		return false
	}

	// Clamp cursor
	if node.CursorPos > len(runes) {
		node.CursorPos = len(runes)
	}

	// Delete character before cursor
	newRunes := make([]rune, 0, len(runes)-1)
	newRunes = append(newRunes, runes[:node.CursorPos-1]...)
	newRunes = append(newRunes, runes[node.CursorPos:]...)

	*node.InputValue = string(newRunes)
	node.CursorPos--

	// Trigger OnChange
	if node.OnChange != nil {
		node.OnChange(*node.InputValue)
	}

	return true
}

// InputDelete deletes the character at the cursor position
func (f *DFrame) InputDelete() bool {
	node := f.FocusedNode()
	if node == nil || node.Type != "input" || node.InputValue == nil {
		return false
	}

	val := *node.InputValue
	runes := []rune(val)

	if node.CursorPos < 0 || node.CursorPos >= len(runes) {
		return false
	}

	// Delete character at cursor
	newRunes := make([]rune, 0, len(runes)-1)
	newRunes = append(newRunes, runes[:node.CursorPos]...)
	newRunes = append(newRunes, runes[node.CursorPos+1:]...)

	*node.InputValue = string(newRunes)

	// Trigger OnChange
	if node.OnChange != nil {
		node.OnChange(*node.InputValue)
	}

	return true
}

// InputMoveCursor moves the cursor by delta positions
func (f *DFrame) InputMoveCursor(delta int) bool {
	node := f.FocusedNode()
	if node == nil || node.Type != "input" || node.InputValue == nil {
		return false
	}

	val := *node.InputValue
	runes := []rune(val)

	newPos := node.CursorPos + delta
	if newPos < 0 {
		newPos = 0
	}
	if newPos > len(runes) {
		newPos = len(runes)
	}

	node.CursorPos = newPos
	return true
}

// InputHome moves cursor to the beginning
func (f *DFrame) InputHome() bool {
	node := f.FocusedNode()
	if node == nil || node.Type != "input" {
		return false
	}
	node.CursorPos = 0
	return true
}

// InputEnd moves cursor to the end
func (f *DFrame) InputEnd() bool {
	node := f.FocusedNode()
	if node == nil || node.Type != "input" || node.InputValue == nil {
		return false
	}
	node.CursorPos = len([]rune(*node.InputValue))
	return true
}

// InputSubmit triggers the OnSubmit callback for the focused input
func (f *DFrame) InputSubmit() bool {
	node := f.FocusedNode()
	if node == nil || node.Type != "input" {
		return false
	}
	if node.OnSubmit != nil {
		node.OnSubmit()
	}
	return true
}

// IsFocusedInput returns true if the focused element is an input
func (f *DFrame) IsFocusedInput() bool {
	node := f.FocusedNode()
	return node != nil && node.Type == "input"
}

// --- Jump Label System ---

// jumpLabelChars defines the characters used for jump labels (home row prioritized)
const jumpLabelChars = "asdfghjklqwertyuiopzxcvbnm"

// GenerateJumpLabels creates labels that scale: 1-char, 2-char, 3-char as needed
func GenerateJumpLabels(count int) []string {
	if count == 0 {
		return nil
	}

	chars := jumpLabelChars
	base := len(chars)

	// Calculate required label length
	length := 1
	for capacity := base; capacity < count; capacity *= base {
		length++
	}

	// Generate labels
	labels := make([]string, 0, count)
	var generate func(prefix string, depth int)
	generate = func(prefix string, depth int) {
		if len(labels) >= count {
			return
		}
		if depth == length {
			labels = append(labels, prefix)
			return
		}
		for _, c := range chars {
			generate(prefix+string(c), depth+1)
		}
	}
	generate("", 0)

	return labels[:count]
}

// EnterJumpMode activates jump label mode
func (f *DFrame) EnterJumpMode() {
	if len(f.focusables) == 0 {
		return
	}

	// Build jump targets sorted by position
	f.jumpTargets = make([]JumpTarget, 0, len(f.focusables))
	for _, idx := range f.focusables {
		node := &f.nodes[idx]
		f.jumpTargets = append(f.jumpTargets, JumpTarget{
			NodeIdx: idx,
			X:       node.X,
			Y:       node.Y,
		})
	}

	// Sort by Y, then X (top-to-bottom, left-to-right)
	for i := 0; i < len(f.jumpTargets)-1; i++ {
		for j := i + 1; j < len(f.jumpTargets); j++ {
			ti, tj := f.jumpTargets[i], f.jumpTargets[j]
			if tj.Y < ti.Y || (tj.Y == ti.Y && tj.X < ti.X) {
				f.jumpTargets[i], f.jumpTargets[j] = tj, ti
			}
		}
	}

	// Generate and assign labels
	labels := GenerateJumpLabels(len(f.jumpTargets))
	for i := range f.jumpTargets {
		f.jumpTargets[i].Label = labels[i]
	}

	f.jumpMode = true
}

// ExitJumpMode deactivates jump label mode
func (f *DFrame) ExitJumpMode() {
	f.jumpMode = false
	f.jumpTargets = nil
}

// InJumpMode returns whether jump mode is active
func (f *DFrame) InJumpMode() bool {
	return f.jumpMode
}

// JumpTargets returns the current jump targets (for rendering)
func (f *DFrame) JumpTargets() []JumpTarget {
	return f.jumpTargets
}

// Execute walks the declarative UI tree and builds layout nodes
func Execute(ui any) *DFrame {
	frame := NewDFrame()
	var ifs ifState
	frame.walkNode(ui, -1, &ifs)
	return frame
}

// ExecuteInto reuses an existing frame
func ExecuteInto(frame *DFrame, ui any) {
	frame.Reset()
	var ifs ifState
	frame.walkNode(ui, -1, &ifs)
}

func (f *DFrame) walkNode(node any, parent int16, ifs *ifState) int16 {
	if node == nil {
		return -1
	}

	// Type switch with direct field access - no reflection for known types
	switch v := node.(type) {
	case DText:
		return f.handleText(v, parent)
	case DProgress:
		return f.handleProgress(v, parent)
	case DButton:
		return f.handleButton(v, parent)
	case DInput:
		return f.handleInput(v, parent)
	case DCheckbox:
		return f.handleCheckbox(v, parent)
	case DRow:
		return f.handleContainer(v.Gap, v.Children, parent, true)
	case DCol:
		return f.handleContainer(v.Gap, v.Children, parent, false)
	case DScroll:
		return f.handleScroll(v, parent)
	case DScrollView:
		return f.handleScrollView(v, parent)
	case DIfNode:
		return f.handleIf(v, parent, ifs)
	case DElseIfNode:
		return f.handleElseIf(v, parent, ifs)
	case DElseNode:
		return f.handleElse(v, parent, ifs)
	case DSwitchNode:
		return f.handleSwitch(v, parent, ifs)
	case DForEachNode:
		return f.handleForEach(v, parent, ifs)
	default:
		// Check if it implements DComponent interface
		if comp, ok := node.(DComponent); ok {
			return f.walkNode(comp.DeclRender(), parent, ifs)
		}
	}

	return -1
}

func (f *DFrame) addNode(nodeType string, parent int16) int16 {
	idx := int16(len(f.nodes))
	f.nodes = append(f.nodes, DLayoutNode{
		Type:             nodeType,
		Parent:           parent,
		FirstChild:       -1,
		NextSib:          -1,
		ScrollViewParent: -1,
	})

	if parent >= 0 {
		p := &f.nodes[parent]
		if p.FirstChild < 0 {
			p.FirstChild = idx
		} else {
			// Find last sibling
			last := p.FirstChild
			for f.nodes[last].NextSib >= 0 {
				last = f.nodes[last].NextSib
			}
			f.nodes[last].NextSib = idx
		}
	}

	return idx
}

func (f *DFrame) handleText(v DText, parent int16) int16 {
	idx := f.addNode("text", parent)
	node := &f.nodes[idx]

	node.Text = resolveString(v.Content)
	node.Bold = v.Bold || v.Style.Bold
	node.W = int16(len(node.Text))
	node.H = 1

	// Apply style
	node.Style = v.Style.ToStyle()

	return idx
}

func (f *DFrame) handleProgress(v DProgress, parent int16) int16 {
	idx := f.addNode("progress", parent)
	node := &f.nodes[idx]

	intVal := resolveInt(v.Value)
	node.Ratio = float32(intVal) / 100.0

	width := v.Width
	if width == 0 {
		width = 20
	}
	node.W = int16(width)
	node.H = 1

	return idx
}

func (f *DFrame) handleButton(v DButton, parent int16) int16 {
	idx := f.addNode("button", parent)
	node := &f.nodes[idx]

	node.Text = resolveString(v.Label)
	node.W = int16(len(node.Text)) + 4 // [ label ]
	node.H = 1
	node.Focusable = true
	node.Disabled = resolveBool(v.Disabled)
	node.OnClick = v.OnClick

	f.focusables = append(f.focusables, idx)
	return idx
}

func (f *DFrame) handleInput(v DInput, parent int16) int16 {
	idx := f.addNode("input", parent)
	node := &f.nodes[idx]

	width := v.Width
	if width == 0 {
		width = 20
	}
	node.W = int16(width)
	node.H = 1
	node.Focusable = true

	// Capture value pointer
	if v.Value != nil {
		node.InputValue = v.Value
		node.Text = *v.Value
	}

	node.OnChange = v.OnChange
	node.OnSubmit = v.OnSubmit

	f.focusables = append(f.focusables, idx)
	return idx
}

func (f *DFrame) handleCheckbox(v DCheckbox, parent int16) int16 {
	idx := f.addNode("checkbox", parent)
	node := &f.nodes[idx]

	labelText := resolveString(v.Label)

	// Read checked state
	checked := false
	if v.Checked != nil {
		ptr := v.Checked
		checked = *ptr
		onChange := v.OnChange

		// OnClick toggles the checkbox
		node.OnClick = func() {
			*ptr = !*ptr
			if onChange != nil {
				onChange(*ptr)
			}
		}
	}

	if checked {
		node.Text = "[x] " + labelText
	} else {
		node.Text = "[ ] " + labelText
	}
	node.W = int16(len(node.Text))
	node.H = 1
	node.Focusable = true

	f.focusables = append(f.focusables, idx)
	return idx
}

func (f *DFrame) handleContainer(gap int8, children []any, parent int16, isRow bool) int16 {
	idx := f.addNode("container", parent)
	node := &f.nodes[idx]
	node.IsRow = isRow
	node.Gap = gap

	var ifs ifState
	for _, child := range children {
		f.walkNode(child, idx, &ifs)
	}

	// Measure
	f.measureNode(idx)

	return idx
}

func (f *DFrame) handleScroll(v DScroll, parent int16) int16 {
	idx := f.addNode("scroll", parent)
	node := &f.nodes[idx]
	node.IsRow = false // scrolls vertically

	// Get scroll offset
	offset := 0
	if v.Offset != nil {
		offset = *v.Offset
	}

	// Calculate viewport
	itemHeight := int(v.ItemHeight)
	if itemHeight <= 0 {
		itemHeight = 1
	}
	viewportHeight := int(v.Height)
	if viewportHeight <= 0 {
		viewportHeight = 10 // default
	}
	viewportRows := viewportHeight / itemHeight

	// Calculate visible range
	totalItems := len(v.Children)
	startIdx := offset
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= totalItems {
		startIdx = totalItems - 1
		if startIdx < 0 {
			startIdx = 0
		}
	}
	endIdx := startIdx + viewportRows
	if endIdx > totalItems {
		endIdx = totalItems
	}

	// Store scroll metadata on node
	node.ScrollOffset = int16(offset)
	node.ScrollTotal = int16(totalItems)
	node.ScrollViewport = int16(viewportRows)
	node.H = int16(viewportHeight)

	// VIEWPORT CULLING: Only walk children in visible range
	var ifs ifState
	for i := startIdx; i < endIdx; i++ {
		f.walkNode(v.Children[i], idx, &ifs)
	}

	// Measure visible content
	f.measureNode(idx)

	// Override height to viewport height (content may be smaller)
	node.H = int16(viewportHeight)

	return idx
}

func (f *DFrame) handleScrollView(v DScrollView, parent int16) int16 {
	idx := f.addNode("scrollview", parent)
	node := &f.nodes[idx]
	node.IsRow = false

	// Store viewport height
	viewportHeight := v.Height
	if viewportHeight <= 0 {
		viewportHeight = 10
	}
	node.H = viewportHeight
	node.ScrollViewport = viewportHeight

	// Get scroll offset
	offset := 0
	if v.Offset != nil {
		offset = *v.Offset
	}
	node.ScrollOffset = int16(offset)

	// Track first child index so we can mark descendants
	firstChildIdx := int16(len(f.nodes))

	// Walk content as children (they'll be measured normally)
	if v.Content != nil {
		var ifs ifState
		f.walkNode(v.Content, idx, &ifs)
	}

	// Mark direct descendants as belonging to this scrollview
	// Don't overwrite nodes that already belong to a nested scrollview
	for i := firstChildIdx; i < int16(len(f.nodes)); i++ {
		if f.nodes[i].ScrollViewParent < 0 {
			f.nodes[i].ScrollViewParent = idx
		}
	}

	// Measure to get content height
	f.measureNode(idx)

	// Store total content height
	node.ScrollTotal = node.H

	// Clamp offset
	maxOffset := int(node.ScrollTotal - node.ScrollViewport)
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	node.ScrollOffset = int16(offset)

	// Update external offset if clamped
	if v.Offset != nil && *v.Offset != offset {
		*v.Offset = offset
	}

	// Override height to viewport (not content height)
	node.H = viewportHeight

	return idx
}

func (f *DFrame) handleIf(v DIfNode, parent int16, ifs *ifState) int16 {
	if resolveBool(v.Cond) {
		ifs.satisfied = true
		return f.walkNode(v.Then, parent, ifs)
	}
	ifs.satisfied = false
	return -1
}

func (f *DFrame) handleElseIf(v DElseIfNode, parent int16, ifs *ifState) int16 {
	if ifs.satisfied {
		return -1
	}

	if resolveBool(v.Cond) {
		ifs.satisfied = true
		return f.walkNode(v.Then, parent, ifs)
	}
	return -1
}

func (f *DFrame) handleElse(v DElseNode, parent int16, ifs *ifState) int16 {
	if ifs.satisfied {
		return -1
	}

	return f.walkNode(v.Then, parent, ifs)
}

func (f *DFrame) handleSwitch(v DSwitchNode, parent int16, ifs *ifState) int16 {
	resolved := resolveAny(v.Value)

	for _, c := range v.Cases {
		switch cv := c.(type) {
		case DCaseNode:
			if reflect.DeepEqual(resolveAny(cv.Match), resolved) {
				return f.walkNode(cv.Then, parent, ifs)
			}
		case DDefaultNode:
			return f.walkNode(cv.Then, parent, ifs)
		}
	}

	return -1
}

func (f *DFrame) handleForEach(v DForEachNode, parent int16, ifs *ifState) int16 {
	slice := resolveSlice(v.Items)
	renderFn := reflect.ValueOf(v.Render)

	var firstIdx int16 = -1

	for i := 0; i < slice.Len(); i++ {
		item := slice.Index(i)

		// Call render function with item
		var result reflect.Value
		if renderFn.Type().In(0).Kind() == reflect.Ptr {
			result = renderFn.Call([]reflect.Value{item.Addr()})[0]
		} else {
			result = renderFn.Call([]reflect.Value{item})[0]
		}

		idx := f.walkNode(result.Interface(), parent, ifs)
		if firstIdx < 0 {
			firstIdx = idx
		}
	}

	return firstIdx
}

func (f *DFrame) measureNode(idx int16) (w, h int16) {
	node := &f.nodes[idx]

	isContainer := node.Type == "container" || node.Type == "scroll" || node.Type == "scrollview"
	if !isContainer {
		return node.W, node.H
	}

	gap := int16(node.Gap)
	var totalW, totalH, maxW, maxH int16
	count := int16(0)

	for child := node.FirstChild; child >= 0; child = f.nodes[child].NextSib {
		cw, ch := f.measureNode(child)
		totalW += cw
		totalH += ch
		if cw > maxW {
			maxW = cw
		}
		if ch > maxH {
			maxH = ch
		}
		count++
	}

	if count > 1 {
		if node.IsRow {
			totalW += gap * (count - 1)
		} else {
			totalH += gap * (count - 1)
		}
	}

	if node.IsRow {
		node.W = totalW
		node.H = maxH
	} else {
		node.W = maxW
		// For scroll containers, preserve the fixed height
		if node.Type != "scroll" {
			node.H = totalH
		}
	}

	return node.W, node.H
}

// Layout positions all nodes
func (f *DFrame) Layout(width, height int16) {
	if len(f.nodes) == 0 {
		return
	}
	f.positionNode(0, 0, 0, width, height)
}

func (f *DFrame) positionNode(idx int16, x, y, w, h int16) {
	node := &f.nodes[idx]
	node.X = x
	node.Y = y

	isContainer := node.Type == "container" || node.Type == "scroll" || node.Type == "scrollview"
	if !isContainer {
		if node.W > w {
			node.W = w
		}
		if node.H > h {
			node.H = h
		}
		return
	}

	node.W = w
	if node.Type != "scrollview" {
		node.H = h
	}
	// scrollview keeps its viewport height from handleScrollView

	gap := int16(node.Gap)
	offset := int16(0)

	// For scrollview, position children in buffer space (0,0 based)
	childX, childY := x, y
	childW := w
	if node.Type == "scrollview" {
		childX, childY = 0, 0
		// Don't include scrollbar in content width
		if node.ScrollTotal > node.ScrollViewport {
			childW = w - 1
		}
	}

	for child := node.FirstChild; child >= 0; child = f.nodes[child].NextSib {
		cn := &f.nodes[child]
		if node.IsRow {
			f.positionNode(child, childX+offset, childY, cn.W, h)
			offset += cn.W + gap
		} else {
			f.positionNode(child, childX, childY+offset, childW, cn.H)
			offset += cn.H + gap
		}
	}
}

// Render draws to buffer
func (f *DFrame) Render(buf *Buffer) {
	for i := range f.nodes {
		node := &f.nodes[i]
		idx := int16(i)

		// Skip nodes that belong to a scrollview - they're rendered separately
		if node.ScrollViewParent >= 0 {
			continue
		}

		focused := f.IsFocused(idx)

		switch node.Type {
		case "text":
			style := node.Style
			if node.Bold {
				style.Attr |= AttrBold
			}
			buf.WriteStringClipped(int(node.X), int(node.Y), node.Text, style, int(node.W))

		case "progress":
			f.renderProgress(buf, node)

		case "button":
			f.renderButton(buf, node, focused)

		case "input":
			f.renderInput(buf, node, focused)

		case "checkbox":
			f.renderCheckbox(buf, node, focused)

		case "scroll":
			f.renderScroll(buf, node)

		case "scrollview":
			f.renderScrollView(buf, node, idx)
		}
	}

	// Render jump labels overlay
	if f.jumpMode {
		f.renderJumpLabels(buf)
	}
}

func (f *DFrame) renderJumpLabels(buf *Buffer) {
	// Yellow background with black text for labels
	labelStyle := Style{FG: BasicColor(0), BG: BasicColor(3), Attr: AttrBold}

	for _, target := range f.jumpTargets {
		x, y := int(target.X), int(target.Y)
		for i, ch := range target.Label {
			buf.Set(x+i, y, NewCell(ch, labelStyle))
		}
	}
}

func (f *DFrame) renderButton(buf *Buffer, node *DLayoutNode, focused bool) {
	style := Style{}
	if focused {
		style.Attr = AttrInverse
	}
	if node.Disabled {
		style.Attr = AttrDim
	}

	text := "[ " + node.Text + " ]"
	buf.WriteStringClipped(int(node.X), int(node.Y), text, style, int(node.W))
}

func (f *DFrame) renderInput(buf *Buffer, node *DLayoutNode, focused bool) {
	// Get current value from pointer
	text := ""
	if node.InputValue != nil {
		text = *node.InputValue
	}
	runes := []rune(text)

	w := int(node.W)
	cursorPos := node.CursorPos
	if cursorPos < 0 {
		cursorPos = 0
	}
	if cursorPos > len(runes) {
		cursorPos = len(runes)
	}

	// Calculate visible window (scroll if cursor would be outside)
	startOffset := 0
	if cursorPos >= w {
		startOffset = cursorPos - w + 1
	}

	// Build visible portion
	endIdx := startOffset + w
	if endIdx > len(runes) {
		endIdx = len(runes)
	}
	visibleRunes := runes[startOffset:endIdx]

	// Pad to width
	for len(visibleRunes) < w {
		visibleRunes = append(visibleRunes, '_')
	}

	x, y := int(node.X), int(node.Y)

	// Render each character
	for i, ch := range visibleRunes {
		style := Style{}
		if focused {
			style.Attr = AttrUnderline
		}

		// Highlight cursor position with inverse
		localCursorPos := cursorPos - startOffset
		if focused && i == localCursorPos {
			style.Attr = AttrInverse
		}

		buf.Set(x+i, y, NewCell(ch, style))
	}
}

func (f *DFrame) renderCheckbox(buf *Buffer, node *DLayoutNode, focused bool) {
	style := Style{}
	if focused {
		style.Attr = AttrInverse
	}
	buf.WriteStringClipped(int(node.X), int(node.Y), node.Text, style, int(node.W))
}

func (f *DFrame) renderScrollView(buf *Buffer, node *DLayoutNode, svIdx int16) {
	// Create off-screen buffer for content
	contentHeight := int(node.ScrollTotal)
	contentWidth := int(node.W)
	if contentHeight <= 0 || contentWidth <= 0 {
		return
	}

	// Create temporary buffer for content
	contentBuf := NewBuffer(contentWidth, contentHeight)

	// Render all descendant nodes into the content buffer
	for i := range f.nodes {
		child := &f.nodes[i]
		if child.ScrollViewParent != svIdx {
			continue
		}

		childIdx := int16(i)
		focused := f.IsFocused(childIdx)

		switch child.Type {
		case "text":
			style := child.Style
			if child.Bold {
				style.Attr |= AttrBold
			}
			contentBuf.WriteStringClipped(int(child.X), int(child.Y), child.Text, style, int(child.W))

		case "progress":
			f.renderProgress(contentBuf, child)

		case "button":
			f.renderButton(contentBuf, child, focused)

		case "input":
			f.renderInput(contentBuf, child, focused)

		case "checkbox":
			f.renderCheckbox(contentBuf, child, focused)

		case "scrollview":
			// Nested scrollview - render it recursively
			f.renderScrollView(contentBuf, child, childIdx)

		case "container":
			// Containers don't render anything themselves
		}
	}

	// Blit visible portion to screen buffer
	scrollOffset := int(node.ScrollOffset)
	viewportHeight := int(node.ScrollViewport)
	screenX := int(node.X)
	screenY := int(node.Y)

	for row := 0; row < viewportHeight; row++ {
		srcY := scrollOffset + row
		if srcY < 0 || srcY >= contentHeight {
			continue
		}
		for col := 0; col < contentWidth; col++ {
			cell := contentBuf.Get(col, srcY)
			if cell.Rune != 0 {
				buf.Set(screenX+col, screenY+row, cell)
			}
		}
	}

	// Draw scrollbar if needed
	if node.ScrollTotal > node.ScrollViewport {
		f.renderScrollbar(buf, node)
	}
}

// renderScrollbar draws a scrollbar for a scroll container
func (f *DFrame) renderScrollbar(buf *Buffer, node *DLayoutNode) {
	sbX := int(node.X + node.W - 1)
	sbTop := int(node.Y)
	sbHeight := int(node.ScrollViewport)

	if sbHeight < 1 || node.ScrollTotal == 0 {
		return
	}

	thumbSize := sbHeight * int(node.ScrollViewport) / int(node.ScrollTotal)
	if thumbSize < 1 {
		thumbSize = 1
	}

	maxScroll := int(node.ScrollTotal - node.ScrollViewport)
	thumbPos := 0
	if maxScroll > 0 {
		thumbPos = (sbHeight - thumbSize) * int(node.ScrollOffset) / maxScroll
	}

	trackStyle := Style{FG: BasicColor(8)}
	for i := 0; i < sbHeight; i++ {
		buf.Set(sbX, sbTop+i, NewCell('│', trackStyle))
	}

	thumbStyle := Style{FG: BasicColor(7)}
	for i := 0; i < thumbSize; i++ {
		buf.Set(sbX, sbTop+thumbPos+i, NewCell('┃', thumbStyle))
	}
}


func (f *DFrame) renderScroll(buf *Buffer, node *DLayoutNode) {
	// Only draw scrollbar if content exceeds viewport
	if node.ScrollTotal <= node.ScrollViewport {
		return
	}

	// Scrollbar position (right edge of scroll container)
	sbX := int(node.X + node.W - 1)
	sbTop := int(node.Y)
	sbHeight := int(node.H)

	if sbHeight < 1 || node.ScrollTotal == 0 {
		return
	}

	// Calculate thumb position and size
	thumbSize := sbHeight * int(node.ScrollViewport) / int(node.ScrollTotal)
	if thumbSize < 1 {
		thumbSize = 1
	}

	maxScroll := int(node.ScrollTotal - node.ScrollViewport)
	thumbPos := 0
	if maxScroll > 0 {
		thumbPos = (sbHeight - thumbSize) * int(node.ScrollOffset) / maxScroll
	}

	// Draw track
	trackStyle := Style{FG: BasicColor(8)} // dim gray
	for i := 0; i < sbHeight; i++ {
		buf.Set(sbX, sbTop+i, NewCell('│', trackStyle))
	}

	// Draw thumb
	thumbStyle := Style{FG: BasicColor(7)} // white
	for i := 0; i < thumbSize; i++ {
		buf.Set(sbX, sbTop+thumbPos+i, NewCell('┃', thumbStyle))
	}
}

func repeatRune(r rune, n int) string {
	if n <= 0 {
		return ""
	}
	result := make([]rune, n)
	for i := range result {
		result[i] = r
	}
	return string(result)
}

func (f *DFrame) renderProgress(buf *Buffer, node *DLayoutNode) {
	barW := int(node.W) - 2
	if barW < 0 {
		barW = 0
	}
	filled := int(node.Ratio * float32(barW))

	style := Style{}
	buf.Set(int(node.X), int(node.Y), NewCell('[', style))
	for i := 0; i < barW; i++ {
		r := '░'
		if i < filled {
			r = '█'
		}
		buf.Set(int(node.X)+1+i, int(node.Y), NewCell(r, style))
	}
	buf.Set(int(node.X)+1+barW, int(node.Y), NewCell(']', style))
}

// --- Value Resolution ---

func resolveString(v any) string {
	if v == nil {
		return ""
	}

	rv := reflect.ValueOf(v)

	// Pointer - dereference
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return ""
		}
		return rv.Elem().String()
	}

	// Function - call it
	if rv.Kind() == reflect.Func {
		results := rv.Call(nil)
		if len(results) > 0 {
			return results[0].String()
		}
		return ""
	}

	// Direct string
	if s, ok := v.(string); ok {
		return s
	}

	return ""
}

func resolveInt(v any) int {
	if v == nil {
		return 0
	}

	rv := reflect.ValueOf(v)

	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return 0
		}
		return int(rv.Elem().Int())
	}

	if rv.Kind() == reflect.Func {
		results := rv.Call(nil)
		if len(results) > 0 {
			return int(results[0].Int())
		}
		return 0
	}

	if i, ok := v.(int); ok {
		return i
	}

	return 0
}

func resolveBool(v any) bool {
	if v == nil {
		return false
	}

	rv := reflect.ValueOf(v)

	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return false
		}
		return rv.Elem().Bool()
	}

	if rv.Kind() == reflect.Func {
		results := rv.Call(nil)
		if len(results) > 0 {
			return results[0].Bool()
		}
		return false
	}

	if b, ok := v.(bool); ok {
		return b
	}

	return false
}

func resolveAny(v any) any {
	if v == nil {
		return nil
	}

	rv := reflect.ValueOf(v)

	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		return rv.Elem().Interface()
	}

	if rv.Kind() == reflect.Func {
		results := rv.Call(nil)
		if len(results) > 0 {
			return results[0].Interface()
		}
		return nil
	}

	return v
}

func resolveSlice(v any) reflect.Value {
	rv := reflect.ValueOf(v)

	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() == reflect.Func {
		results := rv.Call(nil)
		if len(results) > 0 {
			return results[0]
		}
	}

	return rv
}

// --- Demo ---

type DemoProcess struct {
	Name string
	CPU  int
}

var demoData = struct {
	Title     string
	CPULoad   int
	MemLoad   int
	ShowCPU   bool
	Mode      string
	Processes []DemoProcess
}{
	Title:   "Dashboard",
	CPULoad: 75,
	MemLoad: 40,
	ShowCPU: true,
	Mode:    "normal",
	Processes: []DemoProcess{
		{Name: "nginx", CPU: 25},
		{Name: "postgres", CPU: 45},
		{Name: "redis", CPU: 10},
	},
}

var declarativeUI = DCol{
	Children: []any{
		// Static text
		DText{Content: "=== Dashboard ===", Bold: true},

		// Pointer binding - reads current value
		DText{Content: &demoData.Title},

		// Conditional
		DIf(&demoData.ShowCPU,
			DRow{Children: []any{
				DText{Content: "CPU: "},
				DProgress{Value: &demoData.CPULoad, Width: 20},
			}},
		),
		DElse(DText{Content: "CPU hidden"}),

		// Always show mem
		DRow{Children: []any{
			DText{Content: "Mem: "},
			DProgress{Value: &demoData.MemLoad, Width: 20},
		}},

		// Switch
		DSwitch(&demoData.Mode,
			DCase("normal", DText{Content: "Mode: Normal"}),
			DCase("debug", DText{Content: "Mode: DEBUG", Bold: true}),
			DDefault(DText{Content: "Mode: Unknown"}),
		),

		// ForEach
		DText{Content: "Processes:", Bold: true},
		DForEach(&demoData.Processes, func(p *DemoProcess) any {
			return DRow{Children: []any{
				DText{Content: &p.Name},
				DText{Content: ": "},
				DProgress{Value: &p.CPU, Width: 15},
			}}
		}),
	},
}

func DeclarativeDemo() {
	frame := Execute(declarativeUI)
	frame.Layout(50, 20)

	buf := NewBuffer(50, 20)
	frame.Render(buf)

	println("=== Declarative PoC ===")
	println(buf.String())

	// Modify data and re-render
	demoData.CPULoad = 95
	demoData.ShowCPU = false
	demoData.Mode = "debug"

	frame = Execute(declarativeUI)
	frame.Layout(50, 20)
	buf.Clear()
	frame.Render(buf)

	println("=== After data change ===")
	println(buf.String())
}

// --- Interactive Demo ---

var interactiveData = struct {
	Count       int
	Name        string
	AutoRefresh bool
	Message     string
}{
	Count:       0,
	Name:        "",
	AutoRefresh: false,
	Message:     "",
}

var interactiveUI = DCol{
	Gap: 1,
	Children: []any{
		DText{Content: "=== Interactive Demo ===", Bold: true},

		// Buttons
		DRow{Gap: 1, Children: []any{
			DButton{
				Label: "Increment",
				OnClick: func() {
					interactiveData.Count++
					interactiveData.Message = "Count incremented!"
				},
			},
			DButton{
				Label: "Reset",
				OnClick: func() {
					interactiveData.Count = 0
					interactiveData.Message = "Count reset!"
				},
			},
		}},

		// Counter display
		DRow{Children: []any{
			DText{Content: "Count: "},
			DText{Content: func() string {
				return string(rune('0' + interactiveData.Count%10))
			}},
		}},

		// Input
		DRow{Children: []any{
			DText{Content: "Name: "},
			DInput{
				Value: &interactiveData.Name,
				Width: 20,
				OnChange: func(v string) {
					interactiveData.Message = "Name changed to: " + v
				},
			},
		}},

		// Checkbox
		DCheckbox{
			Checked: &interactiveData.AutoRefresh,
			Label:   "Auto Refresh",
			OnChange: func(checked bool) {
				if checked {
					interactiveData.Message = "Auto refresh enabled"
				} else {
					interactiveData.Message = "Auto refresh disabled"
				}
			},
		},

		// Status message
		DText{Content: &interactiveData.Message},
	},
}

func InteractiveDemo() {
	frame := NewDFrame()
	buf := NewBuffer(60, 15)

	// Initial render
	ExecuteInto(frame, interactiveUI)
	frame.Layout(60, 15)
	frame.Render(buf)

	println("=== Interactive Demo ===")
	println(buf.String())
	println("Focusables:", len(frame.focusables))

	// Simulate Tab navigation
	println("\n--- After Tab (focus next) ---")
	frame.FocusNext()
	ExecuteInto(frame, interactiveUI)
	frame.Layout(60, 15)
	buf.Clear()
	frame.Render(buf)
	println(buf.String())

	// Simulate button click
	println("\n--- After Activate (click focused) ---")
	frame.Activate()
	ExecuteInto(frame, interactiveUI)
	frame.Layout(60, 15)
	buf.Clear()
	frame.Render(buf)
	println(buf.String())

	// Tab to checkbox and toggle
	println("\n--- Tab to checkbox and toggle ---")
	frame.FocusNext() // Skip reset button
	frame.FocusNext() // Skip input
	frame.FocusNext() // Checkbox
	frame.Activate()  // Toggle it
	ExecuteInto(frame, interactiveUI)
	frame.Layout(60, 15)
	buf.Clear()
	frame.Render(buf)
	println(buf.String())
}
