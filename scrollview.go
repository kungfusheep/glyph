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
	heightPtr *int16
	margin       [4]int16
	scrollbar    bool
	anchorBottom bool // when content underflows the viewport, hug the bottom edge
	feather      int  // rows of edge fade shown at an overflowing edge (0 = off)
	scrollOffset any  // bound offset: *int (instant) or Animate(...) tween (animated)

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

	// deferred scroll target, applied once content has rendered so callers
	// can position the view before the first frame.
	pendingScroll    int
	hasPendingScroll bool

	// startH remembers the render-buffer height that held the content last frame
	// (plus a viewport of headroom). render() resumes from it so tall content
	// settles in a single Execute pass instead of re-growing from 500 every frame;
	// it tracks content size both up and down.
	startH int

	// lastPasses is how many Execute passes the last render() needed to size the
	// buffer (1 in steady state thanks to startH; >1 only when content outgrows it).
	lastPasses int
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

// Height binds the viewport height to a pointer re-read every frame, for
// scroll regions inside content-sized parents where Grow cannot size them.
func (f ScrollViewFn) Height(p *int16) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.heightPtr = p
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

// AnchorBottom hugs the content to the bottom edge when it is shorter than the
// viewport — the slack falls at the TOP instead of below the last line. Use it for
// chat/log lanes with a composer pinned beneath: few messages sit just above the
// composer, and once content overflows the viewport, normal scrolling resumes
// unchanged (the flag is a no-op when content is taller than the viewport).
func (f ScrollViewFn) AnchorBottom() ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.anchorBottom = true
		return sv
	}
}

// ScrollState returns a managed scroll-offset cell. Pass it to ScrollOffset — directly
// for an instant offset, or wrapped in Animate for a smooth eased scroll. Allocate it
// ONCE at build (glyph builds the tree once, so this is a setup call, not per-frame).
func ScrollState() *int { return new(int) }

// ScrollOffset binds the scroll position to a managed offset (ADR 38). The ScrollView's
// scroll methods — ScrollTo/ScrollDown/HalfPageDown/ScrollToEnd etc. — then drive that
// offset and blit reads it, so there is ONE source of truth (no stale-pending vs manual
// race). Pass a *int (e.g. ScrollState()) for an instant offset, or an Animate over it
// for a smooth eased scroll. Headline form:
//
//	ScrollView.Grow(1).ScrollOffset(Animate(ScrollState()))(rows...)
//	app.Handle("<C-d>", sv.HalfPageDown) // smooth — sets the target, the offset eases
func (f ScrollViewFn) ScrollOffset(offset any) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.scrollOffset = offset
		return sv
	}
}

// Feather fades n rows toward the background at an overflowing edge: the top when
// scrolled down from the top, the bottom when there is more below. The fade appears
// only where content actually overflows (no feather at the top when at the top, none at
// the bottom when scrolled to the end), so it doubles as a scroll-position cue. Needs an
// RGB-ish background to blend toward; with a terminal-default background it is a no-op.
func (f ScrollViewFn) Feather(n int) ScrollViewFn {
	return func(children ...Component) *ScrollViewC {
		sv := f(children...)
		sv.feather = n
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

// ScrollTo positions the content at y once it has rendered. Safe to call
// before the first frame, when the layer has no buffer to clamp against yet.
func (sv *ScrollViewC) ScrollTo(y int) {
	sv.pendingScroll = y
	sv.hasPendingScroll = true
	sv.layer.Invalidate()
}

func (t *Template) compileScrollViewC(v *ScrollViewC, parent int16, depth int) int16 {
	v.layer.feather = v.feather
	// Bind the scroll offset (ADR 38): a *int is instant; an Animate tween over an *int
	// eases the displayed offset toward the target. Wrong-typed offsets are ignored
	// (the legacy scrollY path stays), so this can't break an existing view.
	switch o := v.scrollOffset.(type) {
	case *int:
		v.layer.scrollTarget = o
	case tweenNode:
		if p, ok := o.getTarget().(*int); ok {
			v.layer.scrollTarget = p
			v.layer.scrollEaseDur = o.getTweenDuration()
			v.layer.scrollEaseFn = o.getTweenEasing()
		}
	}
	layerView := LayerView(v.layer).Grow(v.flexGrow)
	if v.scrollbar {
		layerView = LayerView(v.layer).Grow(1)
	}
	if v.heightPtr != nil {
		layerView = layerView.HeightPtr(v.heightPtr)
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
	scrollForViewport := sv.layer.scrollY
	if sv.hasPendingScroll && sv.pendingScroll >= 0 {
		// the deferred target takes effect this frame; register jump targets
		// against the position the content is about to land on
		scrollForViewport = sv.pendingScroll
	}
	sv.childTmpl.setJumpViewport(
		sv.layer.screenX,
		sv.layer.screenY-scrollForViewport,
		sv.layer.screenY,
		sv.layer.screenY+sv.layer.viewHeight,
	)

	// render into a buffer tall enough to hold ALL content, then trim to actual. Start
	// generous and GROW if the content filled the buffer — a fixed cap clipped tall content
	// (e.g. a long chat), so scroll-to-end couldn't reach the last rows and the latest message
	// rendered truncated (e.g. a long chat). Growing keeps the common case (content
	// shorter than the cap) at a single pass.
	h := sv.layer.ViewportHeight()
	if h < 500 {
		h = 500
	}
	// resume from last frame's fitting height so tall content (long chat/log — the
	// case the cap clipped) renders in one pass, not log2(N) re-grows every frame.
	if sv.startH > h {
		h = sv.startH
	}
	var buf *Buffer
	sv.lastPasses = 0
	for {
		buf = NewBuffer(w, h)
		buf.defaultStyle = sv.layer.defaultStyle
		buf.Clear()
		sv.childTmpl.Execute(buf, int16(w), int16(h))
		sv.lastPasses++
		// ContentHeight == h means the content reached the last row — it may be clipped, so
		// grow and re-render until it fits with room to spare (bounded against runaway).
		if buf.ContentHeight() < h || h >= 1<<16 {
			break
		}
		h *= 2
	}
	// remember a start height that holds this frame's content plus a viewport of
	// headroom: steady-state and slow growth render in one pass next frame, while a
	// large shrink lets the start height fall back down (avoids over-allocating).
	sv.startH = buf.ContentHeight() + sv.layer.ViewportHeight()

	// trim to actual content (or at least viewport height)
	vh := sv.layer.ViewportHeight()
	rawContentH := buf.ContentHeight()
	if sv.anchorBottom && rawContentH > 0 && rawContentH < vh {
		// underflow + bottom-anchor: build a viewport-tall buffer and place the measured
		// content in its BOTTOM rows, so the slack falls at the top. maxScroll stays 0
		// (buffer == viewport height), so this never fights ScrollTo, which is a no-op here.
		anchored := NewBuffer(w, vh)
		anchored.defaultStyle = sv.layer.defaultStyle
		anchored.Clear()
		anchored.Blit(buf, 0, 0, 0, vh-rawContentH, w, rawContentH)
		buf = anchored
	} else {
		contentH := rawContentH
		if contentH < vh {
			contentH = vh
		}
		if contentH < h {
			buf.Resize(w, contentH)
		}
	}

	scrollY := sv.layer.ScrollY()
	sv.layer.SetBuffer(buf)
	if sv.hasPendingScroll {
		sv.layer.ScrollTo(sv.pendingScroll)
		sv.hasPendingScroll = false
		return
	}
	sv.layer.ScrollTo(scrollY)
}
