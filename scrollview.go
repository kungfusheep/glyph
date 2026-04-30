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
	layer    *Layer
	children []Component
	flexGrow float32
	margin   [4]int16

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
	if v.margin != [4]int16{} {
		layerView = layerView.MarginTRBL(v.margin[0], v.margin[1], v.margin[2], v.margin[3])
	}
	return t.compileLayerViewC(layerView, parent, depth)
}

func (sv *ScrollViewC) render() {
	w := sv.layer.ViewportWidth()
	if w <= 0 {
		return
	}

	if sv.childTmpl == nil {
		sv.childTmpl = Build(VBox(sv.children...))
	}

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

	sv.layer.SetBuffer(buf)
}
