package glyph

import (
	"reflect"
	"unsafe"
)

// binding represents a declared key binding on a component.
// stored as data during construction, wired to a router during setup.
type binding struct {
	pattern string
	handler any
}

type onDecl interface {
	routeBindings() []binding
}

// KeyC declares a key pattern and handler for an On scope.
type KeyC struct {
	declaredBindings []binding
}

// Key declares a key handler for an On scope.
func Key(pattern string, handler any) KeyC {
	return KeyC{declaredBindings: []binding{{pattern: pattern, handler: handler}}}
}

func (k KeyC) routeBindings() []binding { return k.declaredBindings }

// OnC declares a zero-size event routing scope inside the view tree.
type OnC struct {
	declaredBindings []binding
	modal            bool
}

type OnFn func(decls ...onDecl) OnC

// On declares event handlers as part of the view tree. It renders no cells.
var On OnFn = func(decls ...onDecl) OnC {
	var bindings []binding
	for _, decl := range decls {
		if decl == nil {
			continue
		}
		bindings = append(bindings, decl.routeBindings()...)
	}
	return OnC{declaredBindings: bindings}
}

// Modal declares event handlers that push a modal routing scope while active.
func (f OnFn) Modal(decls ...onDecl) OnC {
	on := f(decls...)
	on.modal = true
	return on
}

func (o OnC) routeBindings() []binding { return o.declaredBindings }

// textInputBinding represents an InputC that wants unmatched keys routed to it.
type textInputBinding struct {
	value    *string
	cursor   *int
	onChange func(string) // optional callback when value changes
}

// ============================================================================
// Functional Component API
// ============================================================================
//
// Container components (VBox, HBox) use a function-type-with-methods pattern:
//   VBox(children...)                    - simple usage
//   VBox.Fill(c).Gap(2)(children...)     - with fill color
//   VBox.CascadeStyle(&s)(children...)   - with style inheritance
//
// Leaf components (Text, Spacer, etc.) use simple functions with method chaining:
//   Text("hello")                        - simple usage
//   Text("hello").Bold().FG(Red)         - with styling
//
// ============================================================================

// Define creates a scoped block for local component helpers and styles.
// The function runs once at compile time (when SetView is called).
// Pointers inside still provide dynamic values at render time.
//
//	app.SetView(
//	    Define(func() Component {
//	        dot := func(ok *bool) Component {
//	            return If(ok).Then(Text("●")).Else(Text("○"))
//	        }
//	        return VBox(dot(&a), dot(&b), dot(&c))
//	    }),
//	)
func Define(fn func() Component) Component {
	return fn()
}

// ============================================================================
// VBox - Vertical container
// ============================================================================

type VBoxC struct {
	fill             Color
	inheritStyle     *Style
	gap              int8
	border           BorderStyle
	borderFG         *Color
	borderFGDyn      any
	borderBG         *Color
	title            string
	width            int16
	height           int16
	percentWidth     float32
	flexGrow         float32
	fitContent       bool
	margin           [4]int16 // top, right, bottom, left
	padding          [4]int16 // top, right, bottom, left
	nodeRef          *NodeRef
	widthPtr         *int16
	heightPtr        *int16
	gapPtr           *int8
	percentWidthPtr  *float32
	flexGrowPtr      *float32
	heightCond       any
	widthCond        any
	gapCond          any
	percentWidthCond any
	flexGrowCond     any
	fillPtr          *Color
	fillCond         any
	localStyle       *Style
	localStylePtr    *Style
	localStyleCond   any
	opacity          dynFloat64
	opacityMode      OpacityMode
	children         []Component
}

type VBoxFn func(children ...Component) VBoxC

// Fill sets the background fill color. Accepts Color, *Color, conditionNode, valueBranchNode, or tweenNode.
func (f VBoxFn) Fill(c any) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		switch val := c.(type) {
		case Color:
			v.fill = val
		case *Color:
			v.fillPtr = val
		case conditionNode:
			v.fillCond = val
		case valueBranchNode:
			v.fillCond = val
		case tweenNode:
			v.fillCond = val
		}
		return v
	}
}

// Style sets a local style for this container (does not cascade to children).
// Accepts Style, *Style, conditionNode, or tweenNode.
func (f VBoxFn) Style(s any) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		switch val := s.(type) {
		case Style:
			v.localStyle = &val
		case *Style:
			v.localStylePtr = val
		case conditionNode:
			v.localStyleCond = val
		case tweenNode:
			v.localStyleCond = val
		}
		return v
	}
}

// Opacity sets the container's opacity (0.0 = invisible, 1.0 = fully visible).
// Accepts float64, *float64, conditionNode, or tweenNode.
func (f VBoxFn) Opacity(o any) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		v.opacity.set(o)
		return v
	}
}

// OpacityMode sets how rune ownership hands off during opacity fades.
func (f VBoxFn) OpacityMode(mode OpacityMode) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		v.opacityMode = mode
		return v
	}
}

// CascadeStyle sets a style pointer that children inherit.
func (f VBoxFn) CascadeStyle(s *Style) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		v.inheritStyle = s
		return v
	}
}

// Gap sets the spacing between children. Accepts int8, int, or *int8 for dynamic values.
func (f VBoxFn) Gap(g any) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		switch val := g.(type) {
		case int8:
			v.gap = val
		case int:
			v.gap = int8(val)
		case *int8:
			v.gapPtr = val
		case conditionNode:
			v.gapCond = val
		case valueBranchNode:
			v.gapCond = val
		case tweenNode:
			v.gapCond = val
		}
		return v
	}
}

// Border sets the border style.
func (f VBoxFn) Border(b BorderStyle) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		v.border = b
		return v
	}
}

// BorderFG sets the border foreground color.
func (f VBoxFn) BorderFG(c any) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		switch val := c.(type) {
		case Color:
			v.borderFG = &val
		case *Color:
			v.borderFG = val
		case conditionNode:
			v.borderFGDyn = val
		case valueBranchNode:
			v.borderFGDyn = val
		case tweenNode:
			v.borderFGDyn = val
		}
		return v
	}
}

// BorderBG sets the border background color.
func (f VBoxFn) BorderBG(c Color) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		v.borderBG = &c
		return v
	}
}

// Title sets the border title text.
func (f VBoxFn) Title(t string) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		v.title = t
		return v
	}
}

// Width sets a fixed width. Accepts int, int16, or *int16 for dynamic values.
func (f VBoxFn) Width(w any) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		switch val := w.(type) {
		case int16:
			v.width = val
		case int:
			v.width = int16(val)
		case *int16:
			v.widthPtr = val
		case conditionNode:
			v.widthCond = val
		case valueBranchNode:
			v.widthCond = val
		case tweenNode:
			v.widthCond = val
		}
		return v
	}
}

// Height sets a fixed height. Accepts int, int16, or *int16 for dynamic values.
func (f VBoxFn) Height(h any) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		switch val := h.(type) {
		case int16:
			v.height = val
		case int:
			v.height = int16(val)
		case *int16:
			v.heightPtr = val
		case conditionNode:
			v.heightCond = val
		case valueBranchNode:
			v.heightCond = val
		case tweenNode:
			v.heightCond = val
		}
		return v
	}
}

// Size sets a fixed width and height.
func (f VBoxFn) Size(w, h int16) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		v.width = w
		v.height = h
		return v
	}
}

// WidthPct sets width as a percentage of the parent (0.0-1.0). Accepts float32, float64, or *float32 for dynamic values.
func (f VBoxFn) WidthPct(pct any) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		switch val := pct.(type) {
		case float32:
			v.percentWidth = val
		case float64:
			v.percentWidth = float32(val)
		case *float32:
			v.percentWidthPtr = val
		case conditionNode:
			v.percentWidthCond = val
		case tweenNode:
			v.percentWidthCond = val
		}
		return v
	}
}

// Grow sets the flex grow factor. Accepts float32, float64, int, or *float32 for dynamic values.
func (f VBoxFn) Grow(g any) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		switch val := g.(type) {
		case float32:
			v.flexGrow = val
		case float64:
			v.flexGrow = float32(val)
		case int:
			v.flexGrow = float32(val)
		case *float32:
			v.flexGrowPtr = val
		case conditionNode:
			v.flexGrowCond = val
		case valueBranchNode:
			v.flexGrowCond = val
		case tweenNode:
			v.flexGrowCond = val
		}
		return v
	}
}

// FitContent sizes the container to fit its content.
func (f VBoxFn) FitContent() VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		v.fitContent = true
		return v
	}
}

// Margin sets uniform margin on all sides.
func (f VBoxFn) Margin(all int16) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		v.margin = [4]int16{all, all, all, all}
		return v
	}
}

// MarginVH sets vertical and horizontal margin.
func (f VBoxFn) MarginVH(vertical, horizontal int16) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		v.margin = [4]int16{vertical, horizontal, vertical, horizontal}
		return v
	}
}

// MarginTRBL sets individual margins for top, right, bottom, left.
func (f VBoxFn) MarginTRBL(top, right, bottom, left int16) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		v.margin = [4]int16{top, right, bottom, left}
		return v
	}
}

// Padding sets uniform padding on all sides.
func (f VBoxFn) Padding(all int16) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		v.padding = [4]int16{all, all, all, all}
		return v
	}
}

// PaddingVH sets vertical and horizontal padding.
func (f VBoxFn) PaddingVH(vertical, horizontal int16) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		v.padding = [4]int16{vertical, horizontal, vertical, horizontal}
		return v
	}
}

// PaddingTRBL sets individual padding for top, right, bottom, left.
func (f VBoxFn) PaddingTRBL(top, right, bottom, left int16) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		v.padding = [4]int16{top, right, bottom, left}
		return v
	}
}

// NodeRef attaches a reference that is populated with this node's rendered
// screen bounds each frame. Use it in effects or anywhere that needs to know
// where a node actually rendered.
func (f VBoxFn) NodeRef(ref *NodeRef) VBoxFn {
	return func(children ...Component) VBoxC {
		v := f(children...)
		v.nodeRef = ref
		return v
	}
}

// VBox arranges children in a vertical stack.
// Use method chaining to configure before calling with children:
//
//	VBox.Gap(1).Border(BorderRounded)("Title")(child1, child2)
var VBox VBoxFn = func(children ...Component) VBoxC {
	return VBoxC{children: children}
}

// ============================================================================
// HBox - Horizontal container
// ============================================================================

type HBoxC struct {
	fill             Color
	inheritStyle     *Style
	gap              int8
	border           BorderStyle
	borderFG         *Color
	borderFGDyn      any
	borderBG         *Color
	title            string
	width            int16
	height           int16
	percentWidth     float32
	flexGrow         float32
	fitContent       bool
	margin           [4]int16 // top, right, bottom, left
	padding          [4]int16 // top, right, bottom, left
	nodeRef          *NodeRef
	widthPtr         *int16
	heightPtr        *int16
	gapPtr           *int8
	percentWidthPtr  *float32
	flexGrowPtr      *float32
	heightCond       any
	widthCond        any
	gapCond          any
	percentWidthCond any
	flexGrowCond     any
	fillPtr          *Color
	fillCond         any
	localStyle       *Style
	localStylePtr    *Style
	localStyleCond   any
	opacity          dynFloat64
	opacityMode      OpacityMode
	children         []Component
}

type HBoxFn func(children ...Component) HBoxC

// Fill sets the background fill color. Accepts Color, *Color, conditionNode, valueBranchNode, or tweenNode.
func (f HBoxFn) Fill(c any) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		switch val := c.(type) {
		case Color:
			h.fill = val
		case *Color:
			h.fillPtr = val
		case conditionNode:
			h.fillCond = val
		case valueBranchNode:
			h.fillCond = val
		case tweenNode:
			h.fillCond = val
		}
		return h
	}
}

// Style sets a local style for this container (does not cascade to children).
// Accepts Style, *Style, conditionNode, or tweenNode.
func (f HBoxFn) Style(s any) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		switch val := s.(type) {
		case Style:
			h.localStyle = &val
		case *Style:
			h.localStylePtr = val
		case conditionNode:
			h.localStyleCond = val
		case tweenNode:
			h.localStyleCond = val
		}
		return h
	}
}

// Opacity sets the container's opacity (0.0 = invisible, 1.0 = fully visible).
// Accepts float64, *float64, conditionNode, or tweenNode.
func (f HBoxFn) Opacity(o any) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		h.opacity.set(o)
		return h
	}
}

// OpacityMode sets how rune ownership hands off during opacity fades.
func (f HBoxFn) OpacityMode(mode OpacityMode) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		h.opacityMode = mode
		return h
	}
}

// CascadeStyle sets a style pointer that children inherit.
func (f HBoxFn) CascadeStyle(s *Style) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		h.inheritStyle = s
		return h
	}
}

// Gap sets the spacing between children. Accepts int8, int, or *int8 for dynamic values.
func (f HBoxFn) Gap(g any) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		switch val := g.(type) {
		case int8:
			h.gap = val
		case int:
			h.gap = int8(val)
		case *int8:
			h.gapPtr = val
		case conditionNode:
			h.gapCond = val
		case valueBranchNode:
			h.gapCond = val
		case tweenNode:
			h.gapCond = val
		}
		return h
	}
}

// Border sets the border style.
func (f HBoxFn) Border(b BorderStyle) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		h.border = b
		return h
	}
}

// BorderFG sets the border foreground color.
func (f HBoxFn) BorderFG(c any) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		switch val := c.(type) {
		case Color:
			h.borderFG = &val
		case *Color:
			h.borderFG = val
		case conditionNode:
			h.borderFGDyn = val
		case valueBranchNode:
			h.borderFGDyn = val
		case tweenNode:
			h.borderFGDyn = val
		}
		return h
	}
}

// BorderBG sets the border background color.
func (f HBoxFn) BorderBG(c Color) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		h.borderBG = &c
		return h
	}
}

// Title sets the border title text.
func (f HBoxFn) Title(t string) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		h.title = t
		return h
	}
}

// Width sets a fixed width. Accepts int, int16, or *int16 for dynamic values.
func (f HBoxFn) Width(w any) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		switch val := w.(type) {
		case int16:
			h.width = val
		case int:
			h.width = int16(val)
		case *int16:
			h.widthPtr = val
		case conditionNode:
			h.widthCond = val
		case valueBranchNode:
			h.widthCond = val
		case tweenNode:
			h.widthCond = val
		}
		return h
	}
}

// Height sets a fixed height. Accepts int, int16, or *int16 for dynamic values.
func (f HBoxFn) Height(h any) HBoxFn {
	return func(children ...Component) HBoxC {
		c := f(children...)
		switch val := h.(type) {
		case int16:
			c.height = val
		case int:
			c.height = int16(val)
		case *int16:
			c.heightPtr = val
		case conditionNode:
			c.heightCond = val
		case valueBranchNode:
			c.heightCond = val
		case tweenNode:
			c.heightCond = val
		}
		return c
	}
}

// Size sets a fixed width and height.
func (f HBoxFn) Size(w, h int16) HBoxFn {
	return func(children ...Component) HBoxC {
		c := f(children...)
		c.width = w
		c.height = h
		return c
	}
}

// WidthPct sets width as a percentage of the parent (0.0-1.0). Accepts float32, float64, or *float32 for dynamic values.
func (f HBoxFn) WidthPct(pct any) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		switch val := pct.(type) {
		case float32:
			h.percentWidth = val
		case float64:
			h.percentWidth = float32(val)
		case *float32:
			h.percentWidthPtr = val
		case conditionNode:
			h.percentWidthCond = val
		case tweenNode:
			h.percentWidthCond = val
		}
		return h
	}
}

// Grow sets the flex grow factor. Accepts float32, float64, int, or *float32 for dynamic values.
func (f HBoxFn) Grow(g any) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		switch val := g.(type) {
		case float32:
			h.flexGrow = val
		case float64:
			h.flexGrow = float32(val)
		case int:
			h.flexGrow = float32(val)
		case *float32:
			h.flexGrowPtr = val
		case conditionNode:
			h.flexGrowCond = val
		case valueBranchNode:
			h.flexGrowCond = val
		case tweenNode:
			h.flexGrowCond = val
		}
		return h
	}
}

// FitContent sizes the container to fit its content.
func (f HBoxFn) FitContent() HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		h.fitContent = true
		return h
	}
}

// Margin sets uniform margin on all sides.
func (f HBoxFn) Margin(all int16) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		h.margin = [4]int16{all, all, all, all}
		return h
	}
}

// MarginVH sets vertical and horizontal margin.
func (f HBoxFn) MarginVH(vertical, horizontal int16) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		h.margin = [4]int16{vertical, horizontal, vertical, horizontal}
		return h
	}
}

// MarginTRBL sets individual margins for top, right, bottom, left.
func (f HBoxFn) MarginTRBL(top, right, bottom, left int16) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		h.margin = [4]int16{top, right, bottom, left}
		return h
	}
}

// Padding sets uniform padding on all sides.
func (f HBoxFn) Padding(all int16) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		h.padding = [4]int16{all, all, all, all}
		return h
	}
}

// PaddingVH sets vertical and horizontal padding.
func (f HBoxFn) PaddingVH(vertical, horizontal int16) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		h.padding = [4]int16{vertical, horizontal, vertical, horizontal}
		return h
	}
}

// PaddingTRBL sets individual padding for top, right, bottom, left.
func (f HBoxFn) PaddingTRBL(top, right, bottom, left int16) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		h.padding = [4]int16{top, right, bottom, left}
		return h
	}
}

// NodeRef attaches a reference that is populated with this node's rendered
// screen bounds each frame. Use it in effects or anywhere that needs to know
// where a node actually rendered.
func (f HBoxFn) NodeRef(ref *NodeRef) HBoxFn {
	return func(children ...Component) HBoxC {
		h := f(children...)
		h.nodeRef = ref
		return h
	}
}

// HBox arranges children in a horizontal row.
// Use method chaining to configure before calling with children:
//
//	HBox.Gap(2)(Text("left"), Space(), Text("right"))
var HBox HBoxFn = func(children ...Component) HBoxC {
	return HBoxC{children: children}
}

// ============================================================================
// Arrange - Container with custom layout function
// ============================================================================

// Arrange creates a container with a custom layout function.
// The layout function receives child sizes and available space, returns positions.
//
//	Arrange(Grid(3, 20, 5))(
//	    Text("A"), Text("B"), Text("C"),
//	    Text("D"), Text("E"), Text("F"),
//	)
func Arrange(layout LayoutFunc) func(children ...Component) Box {
	return func(children ...Component) Box {
		return Box{Layout: layout, Children: children}
	}
}

// ============================================================================
// Custom - Fully custom component
// ============================================================================

// Custom creates a fully custom component with explicit measure and render functions.
// Use this when you need complete control over sizing and drawing.
//
//	Custom(
//	    func(availW int16) (w, h int16) { return 20, 3 },
//	    func(buf *Buffer, x, y, w, h int16) {
//	        buf.WriteString(int(x), int(y), "Custom!", Style{})
//	    },
//	)
func Custom(
	measure func(availW int16) (w, h int16),
	render func(buf *Buffer, x, y, w, h int16),
) Component {
	return customC{Measure: measure, Render: render}
}

// ============================================================================
// Text - Text display
// ============================================================================

type TextC struct {
	content     any // string or *string
	style       Style
	width       int16 // explicit width (0 = content-sized)
	widthPtr    *int16
	widthCond   any
	styleDyn    any // *Style, conditionNode, or tweenNode for whole style
	fgDyn       any // *Color, conditionNode, or tweenNode for FG
	bgDyn       any // *Color, conditionNode, or tweenNode for BG
	opacity     dynFloat64
	opacityMode OpacityMode
}

// Text creates a text display component.
func Text(content any) TextC {
	return TextC{content: content}
}

// Style sets the component style. Accepts Style, *Style, conditionNode, or tweenNode.
func (t TextC) Style(s any) TextC {
	switch val := s.(type) {
	case Style:
		t.style = val
	case *Style:
		t.styleDyn = val
	case conditionNode:
		t.styleDyn = val
	case valueBranchNode:
		t.styleDyn = val
	case tweenNode:
		t.styleDyn = val
	}
	return t
}

// FG sets the foreground color. Accepts Color, *Color, conditionNode, or tweenNode.
func (t TextC) FG(c any) TextC {
	switch val := c.(type) {
	case Color:
		t.style.FG = val
	case *Color:
		t.fgDyn = val
	case conditionNode:
		t.fgDyn = val
	case valueBranchNode:
		t.fgDyn = val
	case tweenNode:
		t.fgDyn = val
	}
	return t
}

// BG sets the background color. Accepts Color, *Color, conditionNode, or tweenNode.
func (t TextC) BG(c any) TextC {
	switch val := c.(type) {
	case Color:
		t.style.BG = val
	case *Color:
		t.bgDyn = val
	case conditionNode:
		t.bgDyn = val
	case valueBranchNode:
		t.bgDyn = val
	case tweenNode:
		t.bgDyn = val
	}
	return t
}

// Opacity sets the text opacity (0.0 = backing cell, 1.0 = fully visible).
// Accepts float64, *float64, conditionNode, or tweenNode.
func (t TextC) Opacity(o any) TextC {
	t.opacity.set(o)
	return t
}

// OpacityMode sets how rune ownership hands off during opacity fades.
func (t TextC) OpacityMode(mode OpacityMode) TextC {
	t.opacityMode = mode
	return t
}

// Bold enables bold text.
func (t TextC) Bold() TextC {
	t.style.Attr |= AttrBold
	return t
}

// Dim enables dim text.
func (t TextC) Dim() TextC {
	t.style.Attr |= AttrDim
	return t
}

// Italic enables italic text.
func (t TextC) Italic() TextC {
	t.style.Attr |= AttrItalic
	return t
}

// Underline enables underline text.
func (t TextC) Underline() TextC {
	t.style.Attr |= AttrUnderline
	return t
}

// Inverse enables inverse (reverse video) text.
func (t TextC) Inverse() TextC {
	t.style.Attr |= AttrInverse
	return t
}

// Strikethrough enables strikethrough text.
func (t TextC) Strikethrough() TextC {
	t.style.Attr |= AttrStrikethrough
	return t
}

// Align sets the text alignment within its available width.
func (t TextC) Align(a Align) TextC { t.style.Align = a; return t }

// Width sets a fixed width. Accepts int16, int, or *int16 for dynamic values.
func (t TextC) Width(w any) TextC {
	switch val := w.(type) {
	case int16:
		t.width = val
	case int:
		t.width = int16(val)
	case *int16:
		t.widthPtr = val
	case conditionNode:
		t.widthCond = val
	case valueBranchNode:
		t.widthCond = val
	case tweenNode:
		t.widthCond = val
	}
	return t
}

// Margin sets uniform margin on all sides.
func (t TextC) Margin(all int16) TextC { t.style.margin = [4]int16{all, all, all, all}; return t }

// MarginVH sets vertical and horizontal margin.
func (t TextC) MarginVH(v, h int16) TextC { t.style.margin = [4]int16{v, h, v, h}; return t }

// MarginTRBL sets individual margins for top, right, bottom, left.
func (t TextC) MarginTRBL(a, b, c, d int16) TextC { t.style.margin = [4]int16{a, b, c, d}; return t }

// Textf composes inline formatted text from mixed parts.
// Accepts string, *string, Span, TextC (from Bold/Italic with *string), and styled helpers.
// Works with ForEach via per-span pointer offset rewriting.
//
// usage:
//
//	Textf("Hello ", Bold("world"), "!")
//	Textf("Name: ", Bold(&it.Name), " Status: ", &it.Status)  // ForEach compatible
func Textf(parts ...any) Component {
	spans := make([]Span, 0, len(parts))
	ptrs := make([]*string, 0, len(parts))
	hasPtrs := false

	for _, p := range parts {
		switch v := p.(type) {
		case string:
			spans = append(spans, Span{Text: v})
			ptrs = append(ptrs, nil)
		case *string:
			spans = append(spans, Span{Text: *v})
			ptrs = append(ptrs, v)
			hasPtrs = true
		case Span:
			spans = append(spans, v)
			ptrs = append(ptrs, nil)
		case TextC:
			// extract the content and style from a TextC (e.g. from Bold(&ptr))
			var sp Span
			sp.Style = v.style
			switch c := v.content.(type) {
			case string:
				sp.Text = c
				spans = append(spans, sp)
				ptrs = append(ptrs, nil)
			case *string:
				sp.Text = *c
				spans = append(spans, sp)
				ptrs = append(ptrs, c)
				hasPtrs = true
			}
		}
	}

	node := richTextNode{Spans: spans}
	if hasPtrs {
		node.spanPtrs = ptrs
	}
	return node
}

// ============================================================================
// Spacer - Empty space
// ============================================================================

type SpacerC struct {
	width        int16
	height       int16
	char         rune
	style        Style
	flexGrow     float32
	widthPtr     *int16
	heightPtr    *int16
	flexGrowPtr  *float32
	widthCond    any
	heightCond   any
	flexGrowCond any
}

// Space creates a flexible empty spacer.
func Space() SpacerC {
	return SpacerC{}
}

// SpaceH creates a vertical spacer with a fixed height.
func SpaceH(h int16) SpacerC {
	return SpacerC{height: h}
}

// SpaceW creates a horizontal spacer with a fixed width.
func SpaceW(w int16) SpacerC {
	return SpacerC{width: w}
}

// Width sets a fixed width. Accepts int16, int, or *int16 for dynamic values.
func (s SpacerC) Width(w any) SpacerC {
	switch val := w.(type) {
	case int16:
		s.width = val
	case int:
		s.width = int16(val)
	case *int16:
		s.widthPtr = val
	case conditionNode:
		s.widthCond = val
	case tweenNode:
		s.widthCond = val
	}
	return s
}

// Height sets a fixed height. Accepts int16, int, or *int16 for dynamic values.
func (s SpacerC) Height(h any) SpacerC {
	switch val := h.(type) {
	case int16:
		s.height = val
	case int:
		s.height = int16(val)
	case *int16:
		s.heightPtr = val
	case conditionNode:
		s.heightCond = val
	case tweenNode:
		s.heightCond = val
	}
	return s
}

// Char sets the display character.
func (s SpacerC) Char(c rune) SpacerC {
	s.char = c
	return s
}

// Style sets the component style.
func (s SpacerC) Style(st Style) SpacerC {
	s.style = st
	return s
}

// Grow sets the flex grow factor. Accepts float32, float64, int, or *float32 for dynamic values.
func (s SpacerC) Grow(g any) SpacerC {
	switch val := g.(type) {
	case float32:
		s.flexGrow = val
	case float64:
		s.flexGrow = float32(val)
	case int:
		s.flexGrow = float32(val)
	case *float32:
		s.flexGrowPtr = val
	case conditionNode:
		s.flexGrowCond = val
	case tweenNode:
		s.flexGrowCond = val
	}
	return s
}

// Margin sets uniform margin on all sides.
func (s SpacerC) Margin(all int16) SpacerC { s.style.margin = [4]int16{all, all, all, all}; return s }

// MarginVH sets vertical and horizontal margin.
func (s SpacerC) MarginVH(v, h int16) SpacerC { s.style.margin = [4]int16{v, h, v, h}; return s }

// MarginTRBL sets individual margins for top, right, bottom, left.
func (s SpacerC) MarginTRBL(a, b, c, d int16) SpacerC {
	s.style.margin = [4]int16{a, b, c, d}
	return s
}

// ============================================================================
// HRule - Horizontal line
// ============================================================================

type HRuleC struct {
	char     rune
	style    Style
	extend   bool
	styleDyn any
	fgDyn    any
	bgDyn    any
}

// HRule creates a horizontal rule.
func HRule() HRuleC {
	return HRuleC{char: '─'}
}

// Extend marks the rule to meet a sibling VRule, producing ┼ junctions.
func (h HRuleC) Extend() HRuleC { h.extend = true; return h }

// Char sets the display character.
func (h HRuleC) Char(c rune) HRuleC {
	h.char = c
	return h
}

// Style sets the component style. Accepts Style, *Style, conditionNode, or tweenNode.
func (h HRuleC) Style(s any) HRuleC {
	switch val := s.(type) {
	case Style:
		h.style = val
	case *Style:
		h.styleDyn = val
	case conditionNode:
		h.styleDyn = val
	case tweenNode:
		h.styleDyn = val
	}
	return h
}

// FG sets the foreground color. Accepts Color, *Color, conditionNode, or tweenNode.
func (h HRuleC) FG(c any) HRuleC {
	switch val := c.(type) {
	case Color:
		h.style.FG = val
	case *Color:
		h.fgDyn = val
	case conditionNode:
		h.fgDyn = val
	case tweenNode:
		h.fgDyn = val
	}
	return h
}

// BG sets the background color. Accepts Color, *Color, conditionNode, or tweenNode.
func (h HRuleC) BG(c any) HRuleC {
	switch val := c.(type) {
	case Color:
		h.style.BG = val
	case *Color:
		h.bgDyn = val
	case conditionNode:
		h.bgDyn = val
	case tweenNode:
		h.bgDyn = val
	}
	return h
}

// Bold enables bold text.
func (h HRuleC) Bold() HRuleC { h.style.Attr |= AttrBold; return h }

// Margin sets uniform margin on all sides.
func (h HRuleC) Margin(all int16) HRuleC { h.style.margin = [4]int16{all, all, all, all}; return h }

// MarginVH sets vertical and horizontal margin.
func (h HRuleC) MarginVH(v, hz int16) HRuleC { h.style.margin = [4]int16{v, hz, v, hz}; return h }

// MarginTRBL sets individual margins for top, right, bottom, left.
func (h HRuleC) MarginTRBL(a, b, c, d int16) HRuleC { h.style.margin = [4]int16{a, b, c, d}; return h }

// ============================================================================
// VRule - Vertical line
// ============================================================================

type VRuleC struct {
	char       rune
	style      Style
	height     int16
	extend     bool
	heightPtr  *int16
	heightCond any
	styleDyn   any
	fgDyn      any
	bgDyn      any
}

// VRule creates a vertical rule.
func VRule() VRuleC {
	return VRuleC{char: '│'}
}

// Char sets the display character.
func (v VRuleC) Char(c rune) VRuleC {
	v.char = c
	return v
}

// Style sets the component style. Accepts Style, *Style, conditionNode, or tweenNode.
func (v VRuleC) Style(s any) VRuleC {
	switch val := s.(type) {
	case Style:
		v.style = val
	case *Style:
		v.styleDyn = val
	case conditionNode:
		v.styleDyn = val
	case tweenNode:
		v.styleDyn = val
	}
	return v
}

// FG sets the foreground color. Accepts Color, *Color, conditionNode, or tweenNode.
func (v VRuleC) FG(c any) VRuleC {
	switch val := c.(type) {
	case Color:
		v.style.FG = val
	case *Color:
		v.fgDyn = val
	case conditionNode:
		v.fgDyn = val
	case tweenNode:
		v.fgDyn = val
	}
	return v
}

// BG sets the background color. Accepts Color, *Color, conditionNode, or tweenNode.
func (v VRuleC) BG(c any) VRuleC {
	switch val := c.(type) {
	case Color:
		v.style.BG = val
	case *Color:
		v.bgDyn = val
	case conditionNode:
		v.bgDyn = val
	case tweenNode:
		v.bgDyn = val
	}
	return v
}

// Bold enables bold text.
func (v VRuleC) Bold() VRuleC { v.style.Attr |= AttrBold; return v }

// Height sets a fixed height. Accepts int16, int, or *int16 for dynamic values.
func (v VRuleC) Height(h any) VRuleC {
	switch val := h.(type) {
	case int16:
		v.height = val
	case int:
		v.height = int16(val)
	case *int16:
		v.heightPtr = val
	case conditionNode:
		v.heightCond = val
	case valueBranchNode:
		v.heightCond = val
	case tweenNode:
		v.heightCond = val
	}
	return v
}

// Extend makes the VRule overdraw one row at each end to meet adjacent HRules or borders.
func (v VRuleC) Extend() VRuleC { v.extend = true; return v }

// Margin sets uniform margin on all sides.
func (v VRuleC) Margin(all int16) VRuleC { v.style.margin = [4]int16{all, all, all, all}; return v }

// MarginVH sets vertical and horizontal margin.
func (v VRuleC) MarginVH(vt, hz int16) VRuleC { v.style.margin = [4]int16{vt, hz, vt, hz}; return v }

// MarginTRBL sets individual margins for top, right, bottom, left.
func (v VRuleC) MarginTRBL(a, b, c, d int16) VRuleC { v.style.margin = [4]int16{a, b, c, d}; return v }

// ============================================================================
// Progress - Progress bar
// ============================================================================

type ProgressC struct {
	value     any // *int (0-100)
	width     int16
	style     Style
	widthPtr  *int16
	widthCond any
	styleDyn  any
	fgDyn     any
	bgDyn     any
}

// Progress creates a progress bar bound to an int pointer (0-100).
func Progress(value any) ProgressC {
	return ProgressC{value: value}
}

// Width sets a fixed width. Accepts int16, int, or *int16 for dynamic values.
func (p ProgressC) Width(w any) ProgressC {
	switch val := w.(type) {
	case int16:
		p.width = val
	case int:
		p.width = int16(val)
	case *int16:
		p.widthPtr = val
	case conditionNode:
		p.widthCond = val
	case tweenNode:
		p.widthCond = val
	}
	return p
}

// Style sets the component style. Accepts Style, *Style, conditionNode, or tweenNode.
func (p ProgressC) Style(s any) ProgressC {
	switch val := s.(type) {
	case Style:
		p.style = val
	case *Style:
		p.styleDyn = val
	case conditionNode:
		p.styleDyn = val
	case tweenNode:
		p.styleDyn = val
	}
	return p
}

// FG sets the foreground color. Accepts Color, *Color, conditionNode, or tweenNode.
func (p ProgressC) FG(c any) ProgressC {
	switch val := c.(type) {
	case Color:
		p.style.FG = val
	case *Color:
		p.fgDyn = val
	case conditionNode:
		p.fgDyn = val
	case tweenNode:
		p.fgDyn = val
	}
	return p
}

// BG sets the background color. Accepts Color, *Color, conditionNode, or tweenNode.
func (p ProgressC) BG(c any) ProgressC {
	switch val := c.(type) {
	case Color:
		p.style.BG = val
	case *Color:
		p.bgDyn = val
	case conditionNode:
		p.bgDyn = val
	case tweenNode:
		p.bgDyn = val
	}
	return p
}

// Bold enables bold text.
func (p ProgressC) Bold() ProgressC { p.style.Attr |= AttrBold; return p }

// Margin sets uniform margin on all sides.
func (p ProgressC) Margin(all int16) ProgressC {
	p.style.margin = [4]int16{all, all, all, all}
	return p
}

// MarginVH sets vertical and horizontal margin.
func (p ProgressC) MarginVH(v, h int16) ProgressC { p.style.margin = [4]int16{v, h, v, h}; return p }

// MarginTRBL sets individual margins for top, right, bottom, left.
func (p ProgressC) MarginTRBL(a, b, c, d int16) ProgressC {
	p.style.margin = [4]int16{a, b, c, d}
	return p
}

// ============================================================================
// Spinner - Animated spinner
// ============================================================================

type SpinnerC struct {
	frame    *int
	frames   []string
	style    Style
	styleDyn any
	fgDyn    any
	bgDyn    any
}

// Spinner creates an animated spinner bound to a frame counter.
// Increment *frame and re-render to advance the animation.
func Spinner(frame *int) SpinnerC {
	return SpinnerC{frame: frame, frames: SpinnerBraille}
}

// Frames sets the animation frames.
func (s SpinnerC) Frames(f []string) SpinnerC {
	s.frames = f
	return s
}

// Style sets the component style. Accepts Style, *Style, conditionNode, or tweenNode.
func (s SpinnerC) Style(st any) SpinnerC {
	switch val := st.(type) {
	case Style:
		s.style = val
	case *Style:
		s.styleDyn = val
	case conditionNode:
		s.styleDyn = val
	case tweenNode:
		s.styleDyn = val
	}
	return s
}

// FG sets the foreground color. Accepts Color, *Color, conditionNode, or tweenNode.
func (s SpinnerC) FG(c any) SpinnerC {
	switch val := c.(type) {
	case Color:
		s.style.FG = val
	case *Color:
		s.fgDyn = val
	case conditionNode:
		s.fgDyn = val
	case tweenNode:
		s.fgDyn = val
	}
	return s
}

// BG sets the background color. Accepts Color, *Color, conditionNode, or tweenNode.
func (s SpinnerC) BG(c any) SpinnerC {
	switch val := c.(type) {
	case Color:
		s.style.BG = val
	case *Color:
		s.bgDyn = val
	case conditionNode:
		s.bgDyn = val
	case tweenNode:
		s.bgDyn = val
	}
	return s
}

// Bold enables bold text.
func (s SpinnerC) Bold() SpinnerC { s.style.Attr |= AttrBold; return s }

// Margin sets uniform margin on all sides.
func (s SpinnerC) Margin(all int16) SpinnerC { s.style.margin = [4]int16{all, all, all, all}; return s }

// MarginVH sets vertical and horizontal margin.
func (s SpinnerC) MarginVH(v, h int16) SpinnerC { s.style.margin = [4]int16{v, h, v, h}; return s }

// MarginTRBL sets individual margins for top, right, bottom, left.
func (s SpinnerC) MarginTRBL(a, b, c, d int16) SpinnerC {
	s.style.margin = [4]int16{a, b, c, d}
	return s
}

// ============================================================================
// Leader - Label.....Value display
// ============================================================================

type LeaderC struct {
	label     any // string or *string
	value     any // string or *string
	width     int16
	fill      rune
	style     Style
	widthPtr  *int16
	widthCond any
	styleDyn  any
	fgDyn     any
	bgDyn     any
}

// Leader creates a label.....value display with fill characters.
// Accepts string or *string for reactive updates.
func Leader(label, value any) LeaderC {
	return LeaderC{label: label, value: value, fill: '.'}
}

// Width sets a fixed width. Accepts int16, int, or *int16 for dynamic values.
func (l LeaderC) Width(w any) LeaderC {
	switch val := w.(type) {
	case int16:
		l.width = val
	case int:
		l.width = int16(val)
	case *int16:
		l.widthPtr = val
	case conditionNode:
		l.widthCond = val
	case tweenNode:
		l.widthCond = val
	}
	return l
}

// Fill sets the fill character.
func (l LeaderC) Fill(r rune) LeaderC {
	l.fill = r
	return l
}

// Style sets the component style. Accepts Style, *Style, conditionNode, or tweenNode.
func (l LeaderC) Style(s any) LeaderC {
	switch val := s.(type) {
	case Style:
		l.style = val
	case *Style:
		l.styleDyn = val
	case conditionNode:
		l.styleDyn = val
	case tweenNode:
		l.styleDyn = val
	}
	return l
}

// FG sets the foreground color. Accepts Color, *Color, conditionNode, or tweenNode.
func (l LeaderC) FG(c any) LeaderC {
	switch val := c.(type) {
	case Color:
		l.style.FG = val
	case *Color:
		l.fgDyn = val
	case conditionNode:
		l.fgDyn = val
	case tweenNode:
		l.fgDyn = val
	}
	return l
}

// BG sets the background color. Accepts Color, *Color, conditionNode, or tweenNode.
func (l LeaderC) BG(c any) LeaderC {
	switch val := c.(type) {
	case Color:
		l.style.BG = val
	case *Color:
		l.bgDyn = val
	case conditionNode:
		l.bgDyn = val
	case tweenNode:
		l.bgDyn = val
	}
	return l
}

// Bold enables bold text.
func (l LeaderC) Bold() LeaderC { l.style.Attr |= AttrBold; return l }

// Margin sets uniform margin on all sides.
func (l LeaderC) Margin(all int16) LeaderC { l.style.margin = [4]int16{all, all, all, all}; return l }

// MarginVH sets vertical and horizontal margin.
func (l LeaderC) MarginVH(v, h int16) LeaderC { l.style.margin = [4]int16{v, h, v, h}; return l }

// MarginTRBL sets individual margins for top, right, bottom, left.
func (l LeaderC) MarginTRBL(a, b, c, d int16) LeaderC {
	l.style.margin = [4]int16{a, b, c, d}
	return l
}

// ============================================================================
// Counter - "current/total" display (alloc-free)
// ============================================================================

// counterC renders two int pointers as "current/total" with an optional
// prefix. Formatting happens at render time using a stack-allocated scratch
// buffer — zero heap allocations per frame.
type counterC struct {
	current   *int
	total     *int
	prefix    string
	style     Style
	streaming *bool  // when non-nil and true, show spinner
	framePtr  *int32 // spinner frame, accessed atomically
}

func newCounter(current, total *int) counterC {
	return counterC{current: current, total: total}
}

func (c counterC) Prefix(p string) counterC   { c.prefix = p; return c }
func (c counterC) Dim() counterC              { c.style.Attr |= AttrDim; return c }
func (c counterC) Streaming(s *bool) counterC { c.streaming = s; return c }

// ============================================================================
// Sparkline - Mini chart
// ============================================================================

type SparklineC struct {
	values     any // []float64 or *[]float64
	width      int16
	height     int16
	min        float64
	max        float64
	style      Style
	widthPtr   *int16
	heightPtr  *int16
	widthCond  any
	heightCond any
	styleDyn   any
	fgDyn      any
	bgDyn      any
}

// Sparkline creates a mini bar chart using Unicode block characters (▁▂▃▄▅▆▇█).
// Pass []float64 or *[]float64 for reactive updates.
func Sparkline(values any) SparklineC {
	return SparklineC{values: values}
}

// Width sets a fixed width. Accepts int16, int, or *int16 for dynamic values.
func (s SparklineC) Width(w any) SparklineC {
	switch val := w.(type) {
	case int16:
		s.width = val
	case int:
		s.width = int16(val)
	case *int16:
		s.widthPtr = val
	case conditionNode:
		s.widthCond = val
	case tweenNode:
		s.widthCond = val
	}
	return s
}

// Height sets the vertical height in rows. Default is 1.
// Each row adds 8 levels of vertical resolution.
// Accepts int16, int, or *int16 for dynamic values.
func (s SparklineC) Height(h any) SparklineC {
	switch val := h.(type) {
	case int16:
		s.height = val
	case int:
		s.height = int16(val)
	case *int16:
		s.heightPtr = val
	case conditionNode:
		s.heightCond = val
	case tweenNode:
		s.heightCond = val
	}
	return s
}

// Range sets the min and max value range for the chart.
func (s SparklineC) Range(min, max float64) SparklineC {
	s.min = min
	s.max = max
	return s
}

// Style sets the component style. Accepts Style, *Style, conditionNode, or tweenNode.
func (s SparklineC) Style(st any) SparklineC {
	switch val := st.(type) {
	case Style:
		s.style = val
	case *Style:
		s.styleDyn = val
	case conditionNode:
		s.styleDyn = val
	case tweenNode:
		s.styleDyn = val
	}
	return s
}

// FG sets the foreground color. Accepts Color, *Color, conditionNode, or tweenNode.
func (s SparklineC) FG(c any) SparklineC {
	switch val := c.(type) {
	case Color:
		s.style.FG = val
	case *Color:
		s.fgDyn = val
	case conditionNode:
		s.fgDyn = val
	case tweenNode:
		s.fgDyn = val
	}
	return s
}

// BG sets the background color. Accepts Color, *Color, conditionNode, or tweenNode.
func (s SparklineC) BG(c any) SparklineC {
	switch val := c.(type) {
	case Color:
		s.style.BG = val
	case *Color:
		s.bgDyn = val
	case conditionNode:
		s.bgDyn = val
	case tweenNode:
		s.bgDyn = val
	}
	return s
}

// Bold enables bold text.
func (s SparklineC) Bold() SparklineC { s.style.Attr |= AttrBold; return s }

// Margin sets uniform margin on all sides.
func (s SparklineC) Margin(all int16) SparklineC {
	s.style.margin = [4]int16{all, all, all, all}
	return s
}

// MarginVH sets vertical and horizontal margin.
func (s SparklineC) MarginVH(v, h int16) SparklineC { s.style.margin = [4]int16{v, h, v, h}; return s }

// MarginTRBL sets individual margins for top, right, bottom, left.
func (s SparklineC) MarginTRBL(a, b, c, d int16) SparklineC {
	s.style.margin = [4]int16{a, b, c, d}
	return s
}

// ============================================================================
// Jump - Jumpable target wrapper
// ============================================================================

type JumpC struct {
	child    any
	onSelect func()
	style    Style
	margin   [4]int16
	padding  [4]int16
}

// Jump wraps a child component as a jump target.
// When jump mode is active, a label appears at this position; typing it calls onSelect.
func Jump(child Component, onSelect func()) JumpC {
	return JumpC{child: child, onSelect: onSelect}
}

// Style sets the component style.
func (j JumpC) Style(s Style) JumpC {
	j.style = s
	return j
}

// Margin sets uniform margin on all sides.
func (j JumpC) Margin(all int16) JumpC { j.margin = [4]int16{all, all, all, all}; return j }

// MarginVH sets vertical and horizontal margin.
func (j JumpC) MarginVH(v, h int16) JumpC { j.margin = [4]int16{v, h, v, h}; return j }

// MarginTRBL sets individual margins for top, right, bottom, left.
func (j JumpC) MarginTRBL(a, b, c, d int16) JumpC { j.margin = [4]int16{a, b, c, d}; return j }

// Padding sets uniform padding on all sides.
func (j JumpC) Padding(all int16) JumpC { j.padding = [4]int16{all, all, all, all}; return j }

// PaddingVH sets vertical and horizontal padding.
func (j JumpC) PaddingVH(v, h int16) JumpC { j.padding = [4]int16{v, h, v, h}; return j }

// PaddingTRBL sets individual padding for top, right, bottom, left.
func (j JumpC) PaddingTRBL(a, b, c, d int16) JumpC { j.padding = [4]int16{a, b, c, d}; return j }

// ============================================================================
// LayerView - Display a pre-rendered layer
// ============================================================================

type LayerViewC struct {
	layer        *Layer
	viewHeight   int16
	viewWidth    int16
	flexGrow     float32
	margin       [4]int16
	padding      [4]int16
	flexGrowPtr  *float32
	flexGrowCond any
}

// LayerView displays a scrollable, pre-rendered layer within the view tree.
// Use for large content that should scroll independently of the main layout.
func LayerView(layer *Layer) LayerViewC {
	return LayerViewC{layer: layer}
}

// Height sets a fixed height.
func (l LayerViewC) Height(h int16) LayerViewC {
	l.viewHeight = h
	return l
}

// Width sets a fixed width.
func (l LayerViewC) Width(w int16) LayerViewC {
	l.viewWidth = w
	return l
}

// Grow sets the flex grow factor. Accepts float32, float64, int, or *float32 for dynamic values.
func (l LayerViewC) Grow(g any) LayerViewC {
	switch val := g.(type) {
	case float32:
		l.flexGrow = val
	case float64:
		l.flexGrow = float32(val)
	case int:
		l.flexGrow = float32(val)
	case *float32:
		l.flexGrowPtr = val
	case conditionNode:
		l.flexGrowCond = val
	case tweenNode:
		l.flexGrowCond = val
	}
	return l
}

// Margin sets uniform margin on all sides.
func (l LayerViewC) Margin(all int16) LayerViewC { l.margin = [4]int16{all, all, all, all}; return l }

// MarginVH sets vertical and horizontal margin.
func (l LayerViewC) MarginVH(v, h int16) LayerViewC { l.margin = [4]int16{v, h, v, h}; return l }

// MarginTRBL sets individual margins for top, right, bottom, left.
func (l LayerViewC) MarginTRBL(a, b, c, d int16) LayerViewC {
	l.margin = [4]int16{a, b, c, d}
	return l
}

// Padding sets uniform padding on all sides.
func (l LayerViewC) Padding(all int16) LayerViewC {
	l.padding = [4]int16{all, all, all, all}
	return l
}

// PaddingVH sets vertical and horizontal padding.
func (l LayerViewC) PaddingVH(v, h int16) LayerViewC { l.padding = [4]int16{v, h, v, h}; return l }

// PaddingTRBL sets individual padding for top, right, bottom, left.
func (l LayerViewC) PaddingTRBL(a, b, c, d int16) LayerViewC {
	l.padding = [4]int16{a, b, c, d}
	return l
}

// ============================================================================
// Overlay - Modal/popup overlay
// ============================================================================

// OverlayPlacement defines how an overlay is positioned within its parent.
type OverlayPlacement int

const (
	OverlayPlacementAt OverlayPlacement = iota
	OverlayPlacementCentered
	OverlayPlacementTop
	OverlayPlacementBottom
	OverlayPlacementLeft
	OverlayPlacementRight
	OverlayPlacementTopLeft
	OverlayPlacementTopRight
	OverlayPlacementBottomLeft
	OverlayPlacementBottomRight
)

// AnchorPosition defines how an overlay is positioned relative to a NodeRef
type AnchorPosition int

const (
	AnchorNone    AnchorPosition = iota
	AnchorBelow                  // top of overlay at bottom of ref
	AnchorAbove                  // bottom of overlay at top of ref
	AnchorOnTop                  // overlay covers the ref exactly
	AnchorRightOf                // left edge of overlay at right edge of ref
	AnchorLeftOf                 // right edge of overlay at left edge of ref
)

type OverlayC struct {
	placement    OverlayPlacement
	placementSet bool
	backdrop     bool
	x, y         int
	offsetX      any
	offsetY      any
	width        int
	height       int
	backdropFG   Color
	bg           Color
	opacity      dynFloat64
	opacityMode  OpacityMode
	anchor       *NodeRef
	anchorPos    AnchorPosition
	children     []Component
}

type OverlayFn func(children ...Component) OverlayC

// Centered centers the overlay content within the parent bounds.
func (f OverlayFn) Centered() OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.placement = OverlayPlacementCentered
		o.placementSet = true
		return o
	}
}

// Top centers the overlay on the top edge of the parent bounds.
func (f OverlayFn) Top() OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.placement = OverlayPlacementTop
		o.placementSet = true
		return o
	}
}

// Bottom centers the overlay on the bottom edge of the parent bounds.
func (f OverlayFn) Bottom() OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.placement = OverlayPlacementBottom
		o.placementSet = true
		return o
	}
}

// Left centers the overlay on the left edge of the parent bounds.
func (f OverlayFn) Left() OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.placement = OverlayPlacementLeft
		o.placementSet = true
		return o
	}
}

// Right centers the overlay on the right edge of the parent bounds.
func (f OverlayFn) Right() OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.placement = OverlayPlacementRight
		o.placementSet = true
		return o
	}
}

// TopLeft positions the overlay at the top-left corner of the parent bounds.
func (f OverlayFn) TopLeft() OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.placement = OverlayPlacementTopLeft
		o.placementSet = true
		return o
	}
}

// TopRight positions the overlay at the top-right corner of the parent bounds.
func (f OverlayFn) TopRight() OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.placement = OverlayPlacementTopRight
		o.placementSet = true
		return o
	}
}

// BottomLeft positions the overlay at the bottom-left corner of the parent bounds.
func (f OverlayFn) BottomLeft() OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.placement = OverlayPlacementBottomLeft
		o.placementSet = true
		return o
	}
}

// BottomRight positions the overlay at the bottom-right corner of the parent bounds.
func (f OverlayFn) BottomRight() OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.placement = OverlayPlacementBottomRight
		o.placementSet = true
		return o
	}
}

// Backdrop renders a backdrop behind the top-most layer that fills the parent.
func (f OverlayFn) Backdrop() OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.backdrop = true
		return o
	}
}

// At positions the overlay at fixed coordinates.
func (f OverlayFn) At(x, y int) OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.placement = OverlayPlacementAt
		o.placementSet = true
		o.x = x
		o.y = y
		return o
	}
}

// Offset translates the overlay from its resolved position. Accepts int, int16,
// *int16, conditionNode, or tweenNode for each axis.
func (f OverlayFn) Offset(x, y any) OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.offsetX = x
		o.offsetY = y
		return o
	}
}

// Size sets a fixed width and height.
func (f OverlayFn) Size(w, h int) OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.width = w
		o.height = h
		return o
	}
}

// BG sets the background color.
func (f OverlayFn) BG(c Color) OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.bg = c
		return o
	}
}

// Opacity sets the overlay opacity (0.0 = backing cells, 1.0 = fully visible).
// Accepts float64, *float64, conditionNode, or tweenNode.
func (f OverlayFn) Opacity(o any) OverlayFn {
	return func(children ...Component) OverlayC {
		ov := f(children...)
		ov.opacity.set(o)
		return ov
	}
}

// OpacityMode sets how rune ownership hands off during opacity fades.
func (f OverlayFn) OpacityMode(mode OpacityMode) OverlayFn {
	return func(children ...Component) OverlayC {
		ov := f(children...)
		ov.opacityMode = mode
		return ov
	}
}

// Below positions the overlay directly below the referenced node.
func (f OverlayFn) Below(ref *NodeRef) OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.anchor = ref
		o.anchorPos = AnchorBelow
		return o
	}
}

// Above positions the overlay directly above the referenced node.
func (f OverlayFn) Above(ref *NodeRef) OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.anchor = ref
		o.anchorPos = AnchorAbove
		return o
	}
}

// OnTop positions the overlay covering the referenced node exactly.
func (f OverlayFn) OnTop(ref *NodeRef) OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.anchor = ref
		o.anchorPos = AnchorOnTop
		return o
	}
}

// RightOf positions the overlay to the right of the referenced node.
func (f OverlayFn) RightOf(ref *NodeRef) OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.anchor = ref
		o.anchorPos = AnchorRightOf
		return o
	}
}

// LeftOf positions the overlay to the left of the referenced node.
func (f OverlayFn) LeftOf(ref *NodeRef) OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.anchor = ref
		o.anchorPos = AnchorLeftOf
		return o
	}
}

// BackdropFG sets the backdrop foreground color.
func (f OverlayFn) BackdropFG(c Color) OverlayFn {
	return func(children ...Component) OverlayC {
		o := f(children...)
		o.backdropFG = c
		return o
	}
}

// Overlay displays content floating above the main view.
// Use for modals, dialogs, and floating windows.
// Control visibility with If: If(&showModal).Then(Overlay.Backdrop()(content))
var Overlay OverlayFn = func(children ...Component) OverlayC {
	return OverlayC{children: children}
}

// ============================================================================
// ForEach - List rendering
// ============================================================================

type ForEachC[T any] struct {
	items    *[]T
	template func(item *T) Component
}

// ForEach renders a template for each item in a slice.
// template: func(item *T) Component. return a component tree for each item.
// Pointer fields inside *T are reactive; mutate and re-render to update.
//
//	ForEach(&todos, func(t *Todo) Component {
//	    return HBox(Text(&t.Name), Text(&t.Status))
//	})
func ForEach[T any](items *[]T, template func(item *T) Component) ForEachC[T] {
	return ForEachC[T]{items: items, template: template}
}

// compileTo implements forEachCompiler for template compilation.
func (f ForEachC[T]) compileTo(t *Template, parent int16, depth int, elemBase unsafe.Pointer, elemSize uintptr) int16 {
	return t.compileForEach(f.items, f.template, parent, depth, elemBase, elemSize)
}

// ============================================================================
// selectionList - Navigable list with selection
// ============================================================================

type ListC[T any] struct {
	items            *[]T
	selected         *int
	internalSel      int // used when no external selection provided
	render           func(*T) Component
	onSelect         func(*T)
	marker           string
	markerStyle      Style
	maxVisible       int
	style            Style
	selectedStyle    Style
	styleDyn         any
	selectedStyleDyn any
	selectedRef      *NodeRef
	scrollOffsetPtr  *int // ScrollState writeback (see ScrollState)
	scrollVisiblePtr *int
	scrollTotalPtr   *int
	cached           *selectionList // cached instance for consistent reference
	declaredBindings []binding
}

// ScrollState binds three *int that the list writes each render, all in SCREEN ROWS:
// the rows scrolled above the window (offset), the visible row count, and the total
// content height. Rows (not item counts) so the scrollbar stays accurate when items
// have different heights. Pair with ScrollbarDyn(total, visible, offset) to put a live
// scrollbar beside a List — the List manages its own scroll internally, so this is how
// an external scrollbar tracks it (the List analogue of ScrollbarForLayer).
func (l *ListC[T]) ScrollState(offset, visible, total *int) *ListC[T] {
	l.scrollOffsetPtr, l.scrollVisiblePtr, l.scrollTotalPtr = offset, visible, total
	if l.cached != nil {
		l.cached.scrollOffsetPtr = offset
		l.cached.scrollVisiblePtr = visible
		l.cached.scrollTotalPtr = total
	}
	return l
}

// List creates a navigable list from a bound slice.
// Use .Render(func(*T) Component) to customize how items appear,
// .Handle(key, func(*T)) for per-item key actions,
// and .BindNav("j","k") or .BindVimNav() for keyboard navigation.
func List[T any](items *[]T) *ListC[T] {
	l := &ListC[T]{
		items:  items,
		marker: "> ",
	}
	l.selected = &l.internalSel
	return l
}

// Ref provides access to the component for external references.
func (l *ListC[T]) Ref(f func(*ListC[T])) *ListC[T] { f(l); return l }

// Selection binds the selection index to an external pointer.
func (l *ListC[T]) Selection(sel *int) *ListC[T] {
	l.selected = sel
	return l
}

// SelectedRef binds a NodeRef that tracks the selected row's position each frame.
func (l *ListC[T]) SelectedRef(ref *NodeRef) *ListC[T] {
	l.selectedRef = ref
	return l
}

// Selected returns a pointer to the currently selected item, or nil if empty.
func (l *ListC[T]) Selected() *T {
	if l.items == nil || len(*l.items) == 0 {
		return nil
	}
	idx := *l.selected
	if idx < 0 || idx >= len(*l.items) {
		return nil
	}
	return &(*l.items)[idx]
}

// Index returns the current selection index.
func (l *ListC[T]) Index() int {
	return *l.selected
}

// SetIndex sets the selection index directly.
func (l *ListC[T]) SetIndex(i int) {
	*l.selected = i
}

// ClampSelection ensures the selection index is within bounds.
func (l *ListC[T]) ClampSelection() {
	n := len(*l.items)
	if n == 0 {
		*l.selected = 0
		return
	}
	if *l.selected >= n {
		*l.selected = n - 1
	}
	if *l.selected < 0 {
		*l.selected = 0
	}
}

// Delete removes the currently selected item.
func (l *ListC[T]) Delete() {
	if l.items == nil || len(*l.items) == 0 {
		return
	}
	idx := *l.selected
	if idx < 0 || idx >= len(*l.items) {
		return
	}
	*l.items = append((*l.items)[:idx], (*l.items)[idx+1:]...)
	if *l.selected >= len(*l.items) && *l.selected > 0 {
		*l.selected--
	}
}

// Render customises how each item appears in the list.
// fn: func(item *T) Component. return a component tree for the item.
func (l *ListC[T]) Render(fn func(*T) Component) *ListC[T] {
	l.render = fn
	return l
}

// OnSelect fires when the user moves to a different item.
// fn: func(item *T). receives a pointer to the newly selected item.
func (l *ListC[T]) OnSelect(fn func(*T)) *ListC[T] {
	l.onSelect = fn
	return l
}

// Marker sets the selection marker (default "> ").
func (l *ListC[T]) Marker(m string) *ListC[T] {
	l.marker = m
	return l
}

// MarkerStyle sets the style for the marker text.
func (l *ListC[T]) MarkerStyle(s Style) *ListC[T] {
	l.markerStyle = s
	return l
}

// MaxVisible sets the maximum visible items (0 = show all).
func (l *ListC[T]) MaxVisible(n int) *ListC[T] {
	l.maxVisible = n
	return l
}

// Style sets the default style for non-selected rows.
// Accepts Style, *Style, conditionNode, or tweenNode.
func (l *ListC[T]) Style(s any) *ListC[T] {
	switch v := s.(type) {
	case Style:
		l.style = v
		if l.cached != nil {
			l.cached.Style = v
		}
	default:
		l.styleDyn = s
		if l.cached != nil {
			l.cached.StyleDyn = s
		}
	}
	return l
}

// SelectedStyle sets the style for the selected row.
// Accepts Style, *Style, conditionNode, or tweenNode.
func (l *ListC[T]) SelectedStyle(s any) *ListC[T] {
	switch v := s.(type) {
	case Style:
		l.selectedStyle = v
		if l.cached != nil {
			l.cached.SelectedStyle = v
		}
	default:
		l.selectedStyleDyn = s
		if l.cached != nil {
			l.cached.SelectedStyleDyn = s
		}
	}
	return l
}

// Margin sets uniform margin on all sides.
func (l *ListC[T]) Margin(all int16) *ListC[T] {
	l.style.margin = [4]int16{all, all, all, all}
	return l
}

// MarginVH sets vertical and horizontal margin.
func (l *ListC[T]) MarginVH(v, h int16) *ListC[T] {
	l.style.margin = [4]int16{v, h, v, h}
	return l
}

// MarginTRBL sets individual margins for top, right, bottom, left.
func (l *ListC[T]) MarginTRBL(t, r, b, li int16) *ListC[T] {
	l.style.margin = [4]int16{t, r, b, li}
	return l
}

// toSelectionList returns the internal selectionList (creates on first call).
// Same instance is returned for both template compilation and method calls.
func (l *ListC[T]) toSelectionList() *selectionList {
	if l.cached == nil {
		sl := &selectionList{
			Items:         l.items,
			Selected:      l.selected,
			Marker:        l.marker,
			MarkerStyle:   l.markerStyle,
			MaxVisible:    l.maxVisible,
			Style:         l.style,
			SelectedStyle: l.selectedStyle,
			SelectedRef:   l.selectedRef,
		}
		sl.scrollOffsetPtr = l.scrollOffsetPtr
		sl.scrollVisiblePtr = l.scrollVisiblePtr
		sl.scrollTotalPtr = l.scrollTotalPtr
		sl.StyleDyn = l.styleDyn
		sl.SelectedStyleDyn = l.selectedStyleDyn
		if l.render != nil {
			sl.Render = l.render
		} else {
			sl.Render = func(item *T) Component { return Text(item) }
		}
		if l.onSelect != nil {
			fn := l.onSelect
			sl.onMove = func() {
				if item := l.Selected(); item != nil {
					fn(item)
				}
			}
		}
		l.cached = sl
	}
	return l.cached
}

// Up moves selection up by one.
func (l *ListC[T]) Up(m any) { l.toSelectionList().Up(m) }

// Down moves selection down by one.
func (l *ListC[T]) Down(m any) { l.toSelectionList().Down(m) }

// PageUp moves selection up by page size.
func (l *ListC[T]) PageUp(m any) { l.toSelectionList().PageUp(m) }

// PageDown moves selection down by page size.
func (l *ListC[T]) PageDown(m any) { l.toSelectionList().PageDown(m) }

// First moves selection to first item.
func (l *ListC[T]) First(m any) { l.toSelectionList().First(m) }

// Last moves selection to last item.
func (l *ListC[T]) Last(m any) { l.toSelectionList().Last(m) }

// BindNav registers key bindings for moving selection down and up.
func (l *ListC[T]) BindNav(down, up string) *ListC[T] {
	l.declaredBindings = append(l.declaredBindings,
		binding{pattern: down, handler: l.Down},
		binding{pattern: up, handler: l.Up},
	)
	return l
}

// BindPageNav registers key bindings for page-sized movement.
func (l *ListC[T]) BindPageNav(pageDown, pageUp string) *ListC[T] {
	l.declaredBindings = append(l.declaredBindings,
		binding{pattern: pageDown, handler: l.PageDown},
		binding{pattern: pageUp, handler: l.PageUp},
	)
	return l
}

// BindFirstLast registers key bindings for jumping to first/last item.
func (l *ListC[T]) BindFirstLast(first, last string) *ListC[T] {
	l.declaredBindings = append(l.declaredBindings,
		binding{pattern: first, handler: l.First},
		binding{pattern: last, handler: l.Last},
	)
	return l
}

// BindVimNav wires the standard vim-style navigation keys:
// j/k for line movement, Ctrl-d/Ctrl-u for page, g/G for first/last.
func (l *ListC[T]) BindVimNav() *ListC[T] {
	return l.BindNav("j", "k").BindPageNav("<C-d>", "<C-u>").BindFirstLast("g", "G")
}

// BindDelete registers a key binding to delete the selected item.
func (l *ListC[T]) BindDelete(key string) *ListC[T] {
	l.declaredBindings = append(l.declaredBindings,
		binding{pattern: key, handler: l.Delete},
	)
	return l
}

// Handle registers a key binding that acts on the currently selected item.
// fn: func(item *T). receives a pointer to the selected item (skipped if empty).
func (l *ListC[T]) Handle(key string, fn func(*T)) *ListC[T] {
	l.declaredBindings = append(l.declaredBindings,
		binding{pattern: key, handler: func() {
			if item := l.Selected(); item != nil {
				fn(item)
			}
		}},
	)
	return l
}

func (l *ListC[T]) bindings() []binding { return l.declaredBindings }

// ============================================================================
// Tabs - Tab headers
// ============================================================================

type TabsC struct {
	labels        []string
	selected      *int
	tabStyle      TabsStyle
	gap           int8
	activeStyle   Style
	inactiveStyle Style
	margin        [4]int16
	gapPtr        *int8
	gapCond       any
}

// Tabs creates a row of selectable tab headers.
// Pair with Switch(&selected) to show the corresponding tab content.
func Tabs(labels []string, selected *int) TabsC {
	return TabsC{labels: labels, selected: selected, gap: 2}
}

// Kind sets the tab rendering style.
func (t TabsC) Kind(s TabsStyle) TabsC {
	t.tabStyle = s
	return t
}

// Gap sets the spacing between children. Accepts int8, int, or *int8 for dynamic values.
func (t TabsC) Gap(g any) TabsC {
	switch val := g.(type) {
	case int8:
		t.gap = val
	case int:
		t.gap = int8(val)
	case *int8:
		t.gapPtr = val
	case conditionNode:
		t.gapCond = val
	case tweenNode:
		t.gapCond = val
	}
	return t
}

// ActiveStyle sets the style for the active tab.
func (t TabsC) ActiveStyle(s Style) TabsC {
	t.activeStyle = s
	return t
}

// InactiveStyle sets the style for inactive tabs.
func (t TabsC) InactiveStyle(s Style) TabsC {
	t.inactiveStyle = s
	return t
}

// Margin sets uniform margin on all sides.
func (t TabsC) Margin(all int16) TabsC { t.margin = [4]int16{all, all, all, all}; return t }

// MarginVH sets vertical and horizontal margin.
func (t TabsC) MarginVH(v, h int16) TabsC { t.margin = [4]int16{v, h, v, h}; return t }

// MarginTRBL sets individual margins for top, right, bottom, left.
func (t TabsC) MarginTRBL(a, b, c, d int16) TabsC { t.margin = [4]int16{a, b, c, d}; return t }

// ============================================================================
// Scrollbar
// ============================================================================

type ScrollbarC struct {
	contentSize   int
	viewSize      int
	contentPtr    *int
	viewPtr       *int
	position      *int
	layer         *Layer
	length        int16
	horizontal    bool
	trackChar     rune
	thumbChar     rune
	trackStyle    Style
	thumbStyle    Style
	trackStyleDyn any
	thumbStyleDyn any
	opacity       dynFloat64
	opacityMode   OpacityMode
	margin        [4]int16
}

// Scroll creates a scrollbar for tracking position in scrollable content.
func Scrollbar(contentSize, viewSize int, position *int) ScrollbarC {
	return ScrollbarC{
		contentSize: contentSize,
		viewSize:    viewSize,
		position:    position,
		trackChar:   '│',
		thumbChar:   '█',
	}
}

// ScrollbarDyn creates a scrollbar whose content size, viewport size, and position
// are all read from *int pointers each frame — for content whose dimensions change at
// runtime (e.g. a List bound via ListC.ScrollState). Mirrors ScrollbarForLayer but for
// any caller that can supply the three live counts.
func ScrollbarDyn(contentSize, viewSize, position *int) ScrollbarC {
	return ScrollbarC{
		contentPtr: contentSize,
		viewPtr:    viewSize,
		position:   position,
		trackChar:  '│',
		thumbChar:  '█',
	}
}

// ScrollbarForLayer creates a scrollbar bound to a scrollable Layer.
func ScrollbarForLayer(layer *Layer) ScrollbarC {
	return ScrollbarC{
		layer:     layer,
		trackChar: '│',
		thumbChar: '█',
	}
}

// Length sets the scrollbar track length.
func (s ScrollbarC) Length(l int16) ScrollbarC {
	s.length = l
	return s
}

// Horizontal renders the scrollbar horizontally instead of vertically.
func (s ScrollbarC) Horizontal() ScrollbarC {
	s.horizontal = true
	s.trackChar = '─'
	return s
}

// TrackChar sets the track display character.
func (s ScrollbarC) TrackChar(c rune) ScrollbarC {
	s.trackChar = c
	return s
}

// ThumbChar sets the thumb display character.
func (s ScrollbarC) ThumbChar(c rune) ScrollbarC {
	s.thumbChar = c
	return s
}

// TrackStyle sets the style for the track.
func (s ScrollbarC) TrackStyle(st any) ScrollbarC {
	switch val := st.(type) {
	case Style:
		s.trackStyle = val
	case *Style:
		s.trackStyleDyn = val
	case conditionNode:
		s.trackStyleDyn = val
	case tweenNode:
		s.trackStyleDyn = val
	}
	return s
}

// ThumbStyle sets the style for the thumb.
func (s ScrollbarC) ThumbStyle(st any) ScrollbarC {
	switch val := st.(type) {
	case Style:
		s.thumbStyle = val
	case *Style:
		s.thumbStyleDyn = val
	case conditionNode:
		s.thumbStyleDyn = val
	case tweenNode:
		s.thumbStyleDyn = val
	}
	return s
}

// Opacity sets the scrollbar opacity (0.0 = backing cell, 1.0 = fully visible).
// Accepts float64, *float64, conditionNode, or tweenNode.
func (s ScrollbarC) Opacity(o any) ScrollbarC {
	s.opacity.set(o)
	return s
}

// OpacityMode sets how rune ownership hands off during opacity fades.
func (s ScrollbarC) OpacityMode(mode OpacityMode) ScrollbarC {
	s.opacityMode = mode
	return s
}

// Margin sets uniform margin on all sides.
func (s ScrollbarC) Margin(all int16) ScrollbarC { s.margin = [4]int16{all, all, all, all}; return s }

// MarginVH sets vertical and horizontal margin.
func (s ScrollbarC) MarginVH(v, h int16) ScrollbarC { s.margin = [4]int16{v, h, v, h}; return s }

// MarginTRBL sets individual margins for top, right, bottom, left.
func (s ScrollbarC) MarginTRBL(a, b, c, d int16) ScrollbarC {
	s.margin = [4]int16{a, b, c, d}
	return s
}

// ============================================================================
// Form Components - Checkbox, Radio, CheckList, Input
// ============================================================================

// CheckboxC is a toggleable checkbox bound to a *bool.
type CheckboxC struct {
	checked          *bool
	label            string
	labelPtr         *string
	checkedMark      string
	unchecked        string
	style            Style
	declaredBindings []binding

	// focus
	focused bool
	onBlur  func()

	// validation
	validator  BoolValidator
	validateOn ValidateOn
	err        string
}

// Checkbox creates a checkbox bound to a bool pointer.
// label accepts string (static) or *string (dynamic).
func Checkbox(checked *bool, label any) *CheckboxC {
	c := &CheckboxC{
		checked:     checked,
		checkedMark: "☑",
		unchecked:   "☐",
	}
	switch v := label.(type) {
	case string:
		c.label = v
	case *string:
		c.labelPtr = v
	}
	return c
}

// Ref provides access to the component for external references.
func (c *CheckboxC) Ref(f func(*CheckboxC)) *CheckboxC { f(c); return c }

// Marks sets the checked and unchecked display characters.
func (c *CheckboxC) Marks(checked, unchecked string) *CheckboxC {
	c.checkedMark = checked
	c.unchecked = unchecked
	return c
}

// Style sets the component style.
func (c *CheckboxC) Style(s Style) *CheckboxC {
	c.style = s
	return c
}

// Margin sets uniform margin on all sides.
func (c *CheckboxC) Margin(all int16) *CheckboxC {
	c.style.margin = [4]int16{all, all, all, all}
	return c
}

// MarginVH sets vertical and horizontal margin.
func (c *CheckboxC) MarginVH(v, h int16) *CheckboxC {
	c.style.margin = [4]int16{v, h, v, h}
	return c
}

// MarginTRBL sets individual margins for top, right, bottom, left.
func (c *CheckboxC) MarginTRBL(t, r, b, l int16) *CheckboxC {
	c.style.margin = [4]int16{t, r, b, l}
	return c
}

// BindToggle registers a key binding to toggle the checked state.
func (c *CheckboxC) BindToggle(key string) *CheckboxC {
	c.declaredBindings = append(c.declaredBindings,
		binding{pattern: key, handler: c.Toggle},
	)
	return c
}

func (c *CheckboxC) bindings() []binding { return c.declaredBindings }

// focusBinding implements focusable. Checkbox has no text input.
func (c *CheckboxC) focusBinding() *textInputBinding { return nil }

// setFocused implements focusable.
func (c *CheckboxC) setFocused(focused bool) {
	wasFocused := c.focused
	c.focused = focused
	if wasFocused && !focused {
		if c.validateOn&VOnBlur != 0 {
			c.runValidation()
		}
		if c.onBlur != nil {
			c.onBlur()
		}
	}
}

// Focused returns whether this checkbox currently has focus.
func (c *CheckboxC) Focused() bool { return c.focused }

// Validate sets a validation function and when it runs.
// If when is omitted, defaults to VOnBlur|VOnSubmit.
func (c *CheckboxC) Validate(fn BoolValidator, when ...ValidateOn) *CheckboxC {
	c.validator = fn
	if len(when) > 0 {
		c.validateOn = when[0]
	} else {
		c.validateOn = VOnBlur | VOnSubmit
	}
	return c
}

// Err returns the current validation error message, or empty string if valid.
func (c *CheckboxC) Err() string {
	return c.err
}

// runValidation runs the validator and stores the result.
func (c *CheckboxC) runValidation() {
	if c.validator != nil {
		if err := c.validator(*c.checked); err != nil {
			c.err = err.Error()
		} else {
			c.err = ""
		}
	}
}

// Toggle flips the checked state.
func (c *CheckboxC) Toggle() {
	*c.checked = !*c.checked
	if c.validateOn&VOnChange != 0 {
		c.runValidation()
	}
}

// Checked returns the current state.
func (c *CheckboxC) Checked() bool {
	return *c.checked
}

// RadioC is a single-selection group bound to *int (selected index).
type RadioC struct {
	selected         *int
	options          []string
	selectedMark     string
	unselected       string
	style            Style
	gap              int8
	horizontal       bool
	declaredBindings []binding
	gapPtr           *int8
	gapCond          any

	// focus
	focused bool
	onBlur  func()
}

// Radio creates a radio group with static options.
func Radio(selected *int, options ...string) *RadioC {
	return &RadioC{
		selected:     selected,
		options:      options,
		selectedMark: "◉",
		unselected:   "○",
	}
}

// Ref provides access to the component for external references.
func (r *RadioC) Ref(f func(*RadioC)) *RadioC { f(r); return r }

// Marks sets the selected and unselected display characters.
func (r *RadioC) Marks(selected, unselected string) *RadioC {
	r.selectedMark = selected
	r.unselected = unselected
	return r
}

// Style sets the component style.
func (r *RadioC) Style(s Style) *RadioC {
	r.style = s
	return r
}

// Margin sets uniform margin on all sides.
func (r *RadioC) Margin(all int16) *RadioC {
	r.style.margin = [4]int16{all, all, all, all}
	return r
}

// MarginVH sets vertical and horizontal margin.
func (r *RadioC) MarginVH(v, h int16) *RadioC {
	r.style.margin = [4]int16{v, h, v, h}
	return r
}

// MarginTRBL sets individual margins for top, right, bottom, left.
func (r *RadioC) MarginTRBL(t, ri, b, l int16) *RadioC {
	r.style.margin = [4]int16{t, ri, b, l}
	return r
}

// Gap sets the spacing between children. Accepts int8, int, or *int8 for dynamic values.
func (r *RadioC) Gap(g any) *RadioC {
	switch val := g.(type) {
	case int8:
		r.gap = val
	case int:
		r.gap = int8(val)
	case *int8:
		r.gapPtr = val
	case conditionNode:
		r.gapCond = val
	case tweenNode:
		r.gapCond = val
	}
	return r
}

// Horizontal renders the radio group horizontally instead of vertically.
func (r *RadioC) Horizontal() *RadioC {
	r.horizontal = true
	return r
}

// BindNav registers key bindings for cycling selection.
func (r *RadioC) BindNav(down, up string) *RadioC {
	r.declaredBindings = append(r.declaredBindings,
		binding{pattern: down, handler: func() { r.Next() }},
		binding{pattern: up, handler: func() { r.Prev() }},
	)
	return r
}

func (r *RadioC) bindings() []binding { return r.declaredBindings }

// focusBinding implements focusable. Radio has no text input.
func (r *RadioC) focusBinding() *textInputBinding { return nil }

// setFocused implements focusable.
func (r *RadioC) setFocused(focused bool) {
	wasFocused := r.focused
	r.focused = focused
	if wasFocused && !focused {
		if r.onBlur != nil {
			r.onBlur()
		}
	}
}

// Focused returns whether this radio group currently has focus.
func (r *RadioC) Focused() bool { return r.focused }

// Next moves selection to next option.
func (r *RadioC) Next() {
	opts := r.getOptions()
	if *r.selected < len(opts)-1 {
		*r.selected++
	}
}

// Prev moves selection to previous option.
func (r *RadioC) Prev() {
	if *r.selected > 0 {
		*r.selected--
	}
}

// Selected returns the currently selected option text.
func (r *RadioC) Selected() string {
	opts := r.getOptions()
	if *r.selected >= 0 && *r.selected < len(opts) {
		return opts[*r.selected]
	}
	return ""
}

// Index returns the selected index.
func (r *RadioC) Index() int {
	return *r.selected
}

func (r *RadioC) getOptions() []string {
	return r.options
}

// CheckListC is a list with per-item checkboxes, similar to todo lists.
type CheckListC[T any] struct {
	items            *[]T
	checked          func(*T) *bool
	render           func(*T) Component
	selected         *int
	internalSel      int
	checkedMark      string
	uncheckedMark    string
	marker           string
	markerStyle      Style
	style            Style
	selectedStyle    Style
	gap              int8
	declaredBindings []binding
	cached           *selectionList
	gapPtr           *int8
	gapCond          any
}

// CheckList creates a navigable checklist from a bound slice.
// Items should have struct tags `glyph:"checked"` and `glyph:"render"`,
// or use .Checked(func(*T) *bool) and .Render(func(*T) Component) to configure manually.
func CheckList[T any](items *[]T) *CheckListC[T] {
	c := &CheckListC[T]{
		items:         items,
		checkedMark:   "☑",
		uncheckedMark: "☐",
		marker:        "> ",
	}
	c.selected = &c.internalSel
	return c
}

// Checked provides the bool pointer that controls each item's checkbox.
// fn: func(item *T) *bool. return a pointer to the item's checked field.
func (c *CheckListC[T]) Checked(fn func(*T) *bool) *CheckListC[T] {
	c.checked = fn
	return c
}

// Render customises how each item appears after its checkbox.
// fn: func(item *T) Component. return a component tree for the item content.
func (c *CheckListC[T]) Render(fn func(*T) Component) *CheckListC[T] {
	c.render = fn
	return c
}

// Marks sets the checkbox characters.
func (c *CheckListC[T]) Marks(checked, unchecked string) *CheckListC[T] {
	c.checkedMark = checked
	c.uncheckedMark = unchecked
	return c
}

// Marker sets the selection indicator.
func (c *CheckListC[T]) Marker(m string) *CheckListC[T] {
	c.marker = m
	return c
}

// MarkerStyle sets the style for the selection marker.
func (c *CheckListC[T]) MarkerStyle(s Style) *CheckListC[T] {
	c.markerStyle = s
	return c
}

// Style sets the component style.
func (c *CheckListC[T]) Style(s Style) *CheckListC[T] {
	c.style = s
	return c
}

// SelectedStyle sets the style for the selected row.
func (c *CheckListC[T]) SelectedStyle(s Style) *CheckListC[T] {
	c.selectedStyle = s
	return c
}

// Margin sets uniform margin on all sides.
func (c *CheckListC[T]) Margin(all int16) *CheckListC[T] {
	c.style.margin = [4]int16{all, all, all, all}
	return c
}

// MarginVH sets vertical and horizontal margin.
func (c *CheckListC[T]) MarginVH(v, h int16) *CheckListC[T] {
	c.style.margin = [4]int16{v, h, v, h}
	return c
}

// MarginTRBL sets individual margins for top, right, bottom, left.
func (c *CheckListC[T]) MarginTRBL(t, r, b, l int16) *CheckListC[T] {
	c.style.margin = [4]int16{t, r, b, l}
	return c
}

// Gap sets the spacing between children. Accepts int8, int, or *int8 for dynamic values.
func (c *CheckListC[T]) Gap(g any) *CheckListC[T] {
	switch val := g.(type) {
	case int8:
		c.gap = val
	case int:
		c.gap = int8(val)
	case *int8:
		c.gapPtr = val
	case conditionNode:
		c.gapCond = val
	case tweenNode:
		c.gapCond = val
	}
	return c
}

// BindNav registers key bindings for moving selection down and up.
func (c *CheckListC[T]) BindNav(down, up string) *CheckListC[T] {
	c.declaredBindings = append(c.declaredBindings,
		binding{pattern: down, handler: c.Down},
		binding{pattern: up, handler: c.Up},
	)
	return c
}

// BindPageNav registers key bindings for page-sized movement.
func (c *CheckListC[T]) BindPageNav(pageDown, pageUp string) *CheckListC[T] {
	c.declaredBindings = append(c.declaredBindings,
		binding{pattern: pageDown, handler: c.PageDown},
		binding{pattern: pageUp, handler: c.PageUp},
	)
	return c
}

// BindFirstLast registers key bindings for jumping to first/last item.
func (c *CheckListC[T]) BindFirstLast(first, last string) *CheckListC[T] {
	c.declaredBindings = append(c.declaredBindings,
		binding{pattern: first, handler: c.First},
		binding{pattern: last, handler: c.Last},
	)
	return c
}

// BindVimNav wires the standard vim-style navigation keys:
// j/k for line movement, Ctrl-d/Ctrl-u for page, g/G for first/last.
func (c *CheckListC[T]) BindVimNav() *CheckListC[T] {
	return c.BindNav("j", "k").BindPageNav("<C-d>", "<C-u>").BindFirstLast("g", "G")
}

// BindToggle registers a key binding to toggle the checked state.
func (c *CheckListC[T]) BindToggle(key string) *CheckListC[T] {
	c.declaredBindings = append(c.declaredBindings,
		binding{pattern: key, handler: func() {
			if c.checked != nil {
				if item := c.Selected(); item != nil {
					ptr := c.checked(item)
					*ptr = !*ptr
				}
			}
		}},
	)
	return c
}

// BindDelete registers a key binding to delete the selected item.
func (c *CheckListC[T]) BindDelete(key string) *CheckListC[T] {
	c.declaredBindings = append(c.declaredBindings,
		binding{pattern: key, handler: c.Delete},
	)
	return c
}

// Handle registers a key binding that acts on the currently selected item.
// fn: func(item *T). receives a pointer to the selected item (skipped if empty).
func (c *CheckListC[T]) Handle(key string, fn func(*T)) *CheckListC[T] {
	c.declaredBindings = append(c.declaredBindings,
		binding{pattern: key, handler: func() {
			if item := c.Selected(); item != nil {
				fn(item)
			}
		}},
	)
	return c
}

func (c *CheckListC[T]) bindings() []binding { return c.declaredBindings }

// Ref provides access to the component for external references.
func (c *CheckListC[T]) Ref(f func(*CheckListC[T])) *CheckListC[T] { f(c); return c }

// SelectedItem returns a pointer to the currently selected item.
func (c *CheckListC[T]) Selected() *T {
	if c.items == nil || len(*c.items) == 0 {
		return nil
	}
	idx := *c.selected
	if idx < 0 || idx >= len(*c.items) {
		return nil
	}
	return &(*c.items)[idx]
}

// Index returns the current selection index.
func (c *CheckListC[T]) Index() int {
	return *c.selected
}

// Delete removes the currently selected item.
func (c *CheckListC[T]) Delete() {
	if c.items == nil || len(*c.items) == 0 {
		return
	}
	idx := *c.selected
	if idx < 0 || idx >= len(*c.items) {
		return
	}
	*c.items = append((*c.items)[:idx], (*c.items)[idx+1:]...)
	if *c.selected >= len(*c.items) && *c.selected > 0 {
		*c.selected--
	}
}

// Up moves selection up by one.
func (c *CheckListC[T]) Up(m any) { c.toSelectionList().Up(m) }

// Down moves selection down by one.
func (c *CheckListC[T]) Down(m any) { c.toSelectionList().Down(m) }

// PageUp moves selection up by page size.
func (c *CheckListC[T]) PageUp(m any) { c.toSelectionList().PageUp(m) }

// PageDown moves selection down by page size.
func (c *CheckListC[T]) PageDown(m any) { c.toSelectionList().PageDown(m) }

// First moves selection to first item.
func (c *CheckListC[T]) First(m any) { c.toSelectionList().First(m) }

// Last moves selection to last item.
func (c *CheckListC[T]) Last(m any) { c.toSelectionList().Last(m) }

func (c *CheckListC[T]) toSelectionList() *selectionList {
	if c.cached == nil {
		// Start with explicit functions (may be nil)
		checkedFn := c.checked
		renderFn := c.render

		// Infer from struct tags if not explicitly set
		if checkedFn == nil || renderFn == nil {
			var sample T
			t := reflect.TypeOf(sample)
			if t.Kind() == reflect.Struct {
				for i := 0; i < t.NumField(); i++ {
					field := t.Field(i)
					tag := field.Tag.Get("glyph")

					if tag == "checked" && field.Type.Kind() == reflect.Bool && checkedFn == nil {
						idx := i
						checkedFn = func(item *T) *bool {
							v := reflect.ValueOf(item).Elem().Field(idx)
							return v.Addr().Interface().(*bool)
						}
					}

					if tag == "render" && field.Type.Kind() == reflect.String && renderFn == nil {
						idx := i
						renderFn = func(item *T) Component {
							v := reflect.ValueOf(item).Elem().Field(idx)
							return Text(v.Addr().Interface().(*string))
						}
					}
				}
			}
		}

		// Store inferred functions so BindToggle etc. can use them
		c.checked = checkedFn
		c.render = renderFn

		c.cached = &selectionList{
			Items:         c.items,
			Selected:      c.selected,
			Marker:        c.marker,
			MarkerStyle:   c.markerStyle,
			Style:         c.style,
			SelectedStyle: c.selectedStyle,
		}

		// Build the render function with checkbox marks
		if checkedFn != nil && renderFn != nil {
			checkedMark := c.checkedMark
			uncheckedMark := c.uncheckedMark
			c.cached.Render = func(item *T) Component {
				mark := If(checkedFn(item)).Then(Text(checkedMark)).Else(Text(uncheckedMark))
				return HBox.Gap(1)(mark, renderFn(item))
			}
		} else if checkedFn != nil {
			checkedMark := c.checkedMark
			uncheckedMark := c.uncheckedMark
			c.cached.Render = func(item *T) Component {
				return If(checkedFn(item)).Then(Text(checkedMark)).Else(Text(uncheckedMark))
			}
		}
	}
	return c.cached
}

// InputC is a text input with internal state management.
type InputC struct {
	field            InputState
	placeholder      string
	width            int
	mask             rune
	style            Style
	placeholderStyle Style
	cursorStyle      Style
	declaredTIB      *textInputBinding
	widthPtr         *int16
	widthCond        any

	externalField *InputState
	focusGroup    *FocusGroup
	focusIndex    int

	// value binding
	boundValue *string

	// validation
	validator  StringValidator
	validateOn ValidateOn
	err        string

	// focus management
	focused bool
	manager *FocusManager

	// blur callback (wired by Form for VOnBlur validation)
	onBlur func()

	// multiline wraps long text across lines instead of scrolling horizontally
	multiline bool
}

// Input creates a text input with internal state.
// Optionally pass a *string to bind the input value to a variable.
func Input(bind ...*string) *InputC {
	i := &InputC{}
	if len(bind) > 0 && bind[0] != nil {
		i.boundValue = bind[0]
		i.field.Value = *bind[0]
	}
	return i
}

// Validate sets a validation function and when it runs.
// If when is omitted, defaults to VOnBlur|VOnSubmit.
func (i *InputC) Validate(fn StringValidator, when ...ValidateOn) *InputC {
	i.validator = fn
	if len(when) > 0 {
		i.validateOn = when[0]
	} else {
		i.validateOn = VOnBlur | VOnSubmit
	}
	return i
}

// Err returns the current validation error message, or empty string if valid.
func (i *InputC) Err() string {
	return i.err
}

// runValidation runs the validator and stores the result.
func (i *InputC) runValidation() {
	if i.validator != nil {
		if err := i.validator(i.State().Value); err != nil {
			i.err = err.Error()
		} else {
			i.err = ""
		}
	}
}

// Ref provides access to the component for external references.
func (i *InputC) Ref(f func(*InputC)) *InputC { f(i); return i }

// Placeholder sets the placeholder text.
func (i *InputC) Placeholder(p string) *InputC {
	i.placeholder = p
	return i
}

// Field binds this input to an external InputState.
func (i *InputC) Field(f *InputState) *InputC {
	i.externalField = f
	return i
}

// FocusGroup controls cursor visibility for a group of externally-managed inputs.
func (i *InputC) FocusGroup(group *FocusGroup, index int) *InputC {
	i.focusGroup = group
	i.focusIndex = index
	return i
}

// PlaceholderStyle sets the style used when rendering placeholder text.
func (i *InputC) PlaceholderStyle(s Style) *InputC {
	i.placeholderStyle = s
	return i
}

// CursorStyle sets the style used for the rendered cursor cell.
func (i *InputC) CursorStyle(s Style) *InputC {
	i.cursorStyle = s
	return i
}

// Width sets the input width. Accepts int16, int, or *int16 for dynamic values.
func (i *InputC) Width(w any) *InputC {
	switch val := w.(type) {
	case int16:
		i.width = int(val)
	case int:
		i.width = val
	case *int16:
		i.widthPtr = val
	case conditionNode:
		i.widthCond = val
	case tweenNode:
		i.widthCond = val
	}
	return i
}

// Mask sets a password mask character.
func (i *InputC) Mask(m rune) *InputC {
	i.mask = m
	return i
}

// MultiLine makes the input wrap long text across multiple lines (growing its height)
// instead of scrolling on a single line. The value stays one string — wrapping is
// visual — so submit/edit behaviour is unchanged.
func (i *InputC) MultiLine() *InputC {
	i.multiline = true
	return i
}

// Style sets the component style.
func (i *InputC) Style(s Style) *InputC {
	i.style = s
	return i
}

// Margin sets uniform margin on all sides.
func (i *InputC) Margin(all int16) *InputC {
	i.style.margin = [4]int16{all, all, all, all}
	return i
}

// MarginVH sets vertical and horizontal margin.
func (i *InputC) MarginVH(v, h int16) *InputC {
	i.style.margin = [4]int16{v, h, v, h}
	return i
}

// MarginTRBL sets individual margins for top, right, bottom, left.
func (i *InputC) MarginTRBL(t, r, b, l int16) *InputC {
	i.style.margin = [4]int16{t, r, b, l}
	return i
}

// Bind routes unmatched key input to this text field.
func (i *InputC) Bind() *InputC {
	field := &i.field
	if i.externalField != nil {
		field = i.externalField
	}
	i.declaredTIB = &textInputBinding{
		value:    &field.Value,
		cursor:   &field.Cursor,
		onChange: i.handleChange,
	}
	return i
}

func (i *InputC) textBinding() *textInputBinding { return i.declaredTIB }

// ManagedBy registers this input with a FocusManager.
// This enables automatic focus cycling and keystroke routing.
func (i *InputC) ManagedBy(fm *FocusManager) *InputC {
	i.manager = fm
	i.focused = false
	field := i.State()
	i.declaredTIB = &textInputBinding{
		value:    &field.Value,
		cursor:   &field.Cursor,
		onChange: i.handleChange,
	}
	fm.Register(i)
	return i
}

// focusBinding implements focusable.
func (i *InputC) focusBinding() *textInputBinding {
	return i.declaredTIB
}

// setFocused implements focusable.
func (i *InputC) setFocused(focused bool) {
	wasFocused := i.focused
	i.focused = focused
	// blur: was focused, now not
	if wasFocused && !focused {
		if i.validateOn&VOnBlur != 0 {
			i.runValidation()
		}
		if i.onBlur != nil {
			i.onBlur()
		}
	}
}

// handleChange is called after every keystroke.
func (i *InputC) handleChange(val string) {
	// sync to bound value
	if i.boundValue != nil {
		*i.boundValue = val
	}
	// validate on change
	if i.validateOn&VOnChange != 0 {
		i.runValidation()
	}
}

// Focused returns whether this input currently has focus.
func (i *InputC) Focused() bool {
	return i.focused
}

// Value returns the current text value.
func (i *InputC) Value() string {
	return i.State().Value
}

// SetValue sets the text value.
func (i *InputC) SetValue(v string) {
	state := i.State()
	state.Value = v
	state.Cursor = len(v)
}

// Clear resets the input.
func (i *InputC) Clear() {
	i.State().Clear()
}

// State returns a pointer to the internal input state.
func (i *InputC) State() *InputState {
	if i.externalField != nil {
		return i.externalField
	}
	return &i.field
}

// toTextInput converts to the underlying compiled form for rendering.
func (i *InputC) toTextInput() textInput {
	field := &i.field
	if i.externalField != nil {
		field = i.externalField
	}
	ti := textInput{
		Field:            field,
		FocusGroup:       i.focusGroup,
		FocusIndex:       i.focusIndex,
		Value:            i.boundValue,
		Placeholder:      i.placeholder,
		Width:            i.width,
		Mask:             i.mask,
		Style:            i.style,
		PlaceholderStyle: i.placeholderStyle,
		CursorStyle:      i.cursorStyle,
		MultiLine:        i.multiline,
	}
	// if managed by focus manager, use focused state for cursor visibility
	if i.manager != nil {
		ti.Focused = &i.focused
	}
	return ti
}
