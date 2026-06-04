package glyph

// ScrollViewC wraps children in a scrollable layer. Children are laid out
// using normal glyph components, rendered into an offscreen buffer once,
// and then blitted each frame. Re-renders only when the viewport width changes.
//
// Usage:
//
//	ScrollView.Grow(1)(
//	    Text("Hello").Bold(),
//	    SpaceH(1),
//	    Text("World"),
//	)
type ScrollViewC struct {
	layer     *Layer
	children  []Component
	flexGrow  float32
	margin    [4]int16
	scrollbar bool

	scrollbarTrackStyle  any
	scrollbarThumbStyle  any
	scrollbarOpacity     dynFloat64
	scrollbarOpacityMode OpacityMode

	// wrapper config — when any of these are set, the layer view is
	// wrapped in a configured VBox at compile time.
	border   BorderStyle
	borderFG any
	borderBG any
	fill     any
	title    string
	padding  [4]int16
	wrap     bool // true if any wrapper config has been set

	// cached sub-template, rebuilt on width change
	childTmpl *Template
}

type ScrollViewFn func(children ...Component) *ScrollViewC

// ScrollView creates a scrollable container for its children.
var ScrollView ScrollViewFn = func(children ...Component) *ScrollViewC {
	sv := &ScrollViewC{
		layer:    NewLayer(),
		children: children,
	}
	sv.layer.Render = sv.render
	return sv
}

func (f ScrollViewFn) Grow(g any) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		switch val := g.(type) {
		case float32:
			sv.flexGrow = val
		case float64:
			sv.flexGrow = float32(val)
		case int:
			sv.flexGrow = float32(val)
		}
		return sv
	}
}

func (f ScrollViewFn) Margin(all int16) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.margin = [4]int16{all, all, all, all}
		return sv
	}
}

func (f ScrollViewFn) MarginVH(v, h int16) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.margin = [4]int16{v, h, v, h}
		return sv
	}
}

// MarginTRBL sets individual margins for top, right, bottom, left.
func (f ScrollViewFn) MarginTRBL(top, right, bottom, left int16) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.margin = [4]int16{top, right, bottom, left}
		return sv
	}
}

// Border sets the border style. Wraps the scroll viewport in a bordered box.
func (f ScrollViewFn) Border(b BorderStyle) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.border = b
		sv.wrap = true
		return sv
	}
}

// BorderFG sets the border foreground color.
func (f ScrollViewFn) BorderFG(c any) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.borderFG = c
		sv.wrap = true
		return sv
	}
}

// BorderBG sets the border background color.
func (f ScrollViewFn) BorderBG(c any) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.borderBG = c
		sv.wrap = true
		return sv
	}
}

// Fill sets the background fill color of the wrapping box.
func (f ScrollViewFn) Fill(c any) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.fill = c
		sv.wrap = true
		return sv
	}
}

// Title sets a title for the bordered scroll viewport.
func (f ScrollViewFn) Title(t string) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.title = t
		sv.wrap = true
		return sv
	}
}

// Padding sets uniform padding on all sides of the wrapping box.
func (f ScrollViewFn) Padding(all int16) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.padding = [4]int16{all, all, all, all}
		sv.wrap = true
		return sv
	}
}

// PaddingVH sets vertical and horizontal padding.
func (f ScrollViewFn) PaddingVH(v, h int16) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.padding = [4]int16{v, h, v, h}
		sv.wrap = true
		return sv
	}
}

// PaddingTRBL sets individual padding for top, right, bottom, left.
func (f ScrollViewFn) PaddingTRBL(top, right, bottom, left int16) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.padding = [4]int16{top, right, bottom, left}
		sv.wrap = true
		return sv
	}
}

// Scrollbar reserves a one-column gutter and renders a scrollbar bound to the
// scroll view's layer.
func (f ScrollViewFn) Scrollbar() ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.scrollbar = true
		return sv
	}
}

// ScrollbarVisible reserves the scrollbar gutter and fades the scrollbar in
// while the condition is true.
func (f ScrollViewFn) ScrollbarVisible(visible *bool) ScrollViewFn {
	return f.Scrollbar().ScrollbarOpacity(
		Animate(If(visible).Then(1.0).Else(0.0)),
	)
}

func (f ScrollViewFn) ScrollbarTrackStyle(st any) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.scrollbarTrackStyle = st
		return sv
	}
}

func (f ScrollViewFn) ScrollbarThumbStyle(st any) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.scrollbarThumbStyle = st
		return sv
	}
}

func (f ScrollViewFn) ScrollbarOpacity(o any) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.scrollbarOpacity.set(o)
		return sv
	}
}

func (f ScrollViewFn) ScrollbarOpacityMode(mode OpacityMode) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.scrollbarOpacityMode = mode
		return sv
	}
}

// Ref captures a reference to the ScrollView via a callback during construction.
func (f ScrollViewFn) Ref(fn func(*ScrollViewC)) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		fn(sv)
		return sv
	}
}

// Layer returns the underlying layer for scroll control wiring.
func (sv *ScrollViewC) Layer() *Layer {
	return sv.layer
}

// SetChildren replaces the children and marks for re-render.
func (sv *ScrollViewC) SetChildren(children ...Component) {
	sv.children = children
	sv.childTmpl = nil // force rebuild
	sv.layer.lastRenderWidth = 0
}

// Refresh forces re-render of children on the next frame.
// Call when the content has changed.
func (sv *ScrollViewC) Refresh() {
	sv.layer.Invalidate()
}

func (t *Template) compileScrollViewC(v *ScrollViewC, parent int16, depth int) int16 {
	layerView := LayerView(v.layer).Grow(v.flexGrow)
	if v.scrollbar {
		layerView = LayerView(v.layer).Grow(1)
	}
	if v.margin != [4]int16{} && !v.scrollbar {
		layerView = layerView.MarginTRBL(v.margin[0], v.margin[1], v.margin[2], v.margin[3])
	}
	component := Component(layerView)
	if v.scrollbar {
		bar := ScrollbarForLayer(v.layer).
			TrackStyle(v.scrollbarTrackStyle).
			ThumbStyle(v.scrollbarThumbStyle).
			OpacityMode(v.scrollbarOpacityMode)
		bar.opacity = v.scrollbarOpacity
		box := HBox.Grow(v.flexGrow)
		if v.margin != [4]int16{} {
			box = box.MarginTRBL(v.margin[0], v.margin[1], v.margin[2], v.margin[3])
		}
		component = box(layerView, bar)
	}
	if !v.wrap {
		return t.compile(component, parent, depth, nil, 0)
	}
	// wrap in a configured VBox carrying border/title/padding/fill.
	box := VBox
	if v.border.HasBorder() {
		box = box.Border(v.border)
	}
	if v.borderFG != nil {
		box = box.BorderFG(v.borderFG)
	}
	if v.borderBG != nil {
		if c, ok := v.borderBG.(Color); ok {
			box = box.BorderBG(c)
		}
	}
	if v.fill != nil {
		box = box.Fill(v.fill)
	}
	if v.title != "" {
		box = box.Title(v.title)
	}
	if v.padding != [4]int16{} {
		box = box.PaddingTRBL(v.padding[0], v.padding[1], v.padding[2], v.padding[3])
	}
	return t.compileVBoxC(box(component), parent, depth, nil, 0)
}

func (sv *ScrollViewC) render() {
	w := sv.layer.ViewportWidth()
	if w <= 0 {
		return
	}

	if sv.childTmpl == nil {
		sv.childTmpl = Build(VBox(sv.children...))
	}
	if sv.layer.app != nil {
		sv.childTmpl.SetApp(sv.layer.app)
	}
	sv.childTmpl.setJumpViewport(
		sv.layer.screenX,
		sv.layer.screenY-sv.layer.scrollY,
		sv.layer.screenY,
		sv.layer.screenY+sv.layer.viewHeight,
	)

	// use a generous height so content isn't clipped, then trim to actual
	h := sv.layer.ViewportHeight()
	if h < 500 {
		h = 500
	}

	buf := NewBuffer(w, h)
	buf.defaultStyle = sv.layer.defaultStyle
	buf.Clear()
	sv.childTmpl.Execute(buf, int16(w), int16(h))

	// trim to actual content (or at least viewport height)
	contentH := buf.ContentHeight()
	if contentH < sv.layer.ViewportHeight() {
		contentH = sv.layer.ViewportHeight()
	}
	if contentH < h {
		buf.Resize(w, contentH)
	}

	scrollY := sv.layer.ScrollY()
	sv.layer.SetBuffer(buf)
	sv.layer.ScrollTo(scrollY)
}
