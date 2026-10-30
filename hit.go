package glyph

import "github.com/kungfusheep/riffkey"

// hitRegion is a rectangle painted this frame that reacts to the pointer.
// Coordinates are those of the buffer the region was rendered into: screen
// cells for the root template, layer-buffer cells for content inside a
// ScrollView (the layer region translates on the way in). A region with no
// handlers and no layer still swallows the event, which is how an overlay's
// body keeps a tap from reaching the content under it.
type hitRegion struct {
	X, Y, W, H int16
	tap        func()    // left button press
	wheel      func(int) // wheel delta in rows: +n down, -n up
	hover      *Style    // painted over the region while the pointer rests on it
	hoverMix   float32   // 0 replaces the region's colours with hover's; 0 < mix < 1 washes them toward it
	layer      *Layer    // the region is a layer viewport: search layer.hits first
}

func (r *hitRegion) contains(x, y int) bool {
	return x >= int(r.X) && x < int(r.X)+int(r.W) && y >= int(r.Y) && y < int(r.Y)+int(r.H)
}

func (r *hitRegion) swallows() bool {
	return r.tap == nil && r.wheel == nil && r.hover == nil && r.layer == nil
}

// hitRect is a region's footprint in screen cells, clipped to every layer
// viewport between it and the screen.
type hitRect struct{ X, Y, W, H int }

func (r hitRect) empty() bool { return r.W <= 0 || r.H <= 0 }

func (r hitRect) intersect(o hitRect) hitRect {
	x0, y0 := max(r.X, o.X), max(r.Y, o.Y)
	x1, y1 := min(r.X+r.W, o.X+o.W), min(r.Y+r.H, o.Y+o.H)
	return hitRect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

var unclipped = hitRect{X: -1 << 20, Y: -1 << 20, W: 1 << 21, H: 1 << 21}

// Mouse enables pointer input: taps on Tap, TapItem and Jump targets and on
// List rows, wheel scrolling over ScrollViews, Lists and Wheel targets, and
// hover styling on targets that set one. Off by default: while it is on the
// terminal hands every click and drag to the app, so the terminal's own text
// selection needs its modifier-drag bypass. Call it before Run.
func (a *App) Mouse(on bool) *App {
	a.mouse = on
	a.screen.SetMouse(on)
	return a
}

// addHit records a region for this frame. Regions are appended in paint order,
// so the last region containing a point is the topmost one.
func (a *App) addHit(r hitRegion) {
	if a.hitSink == nil {
		return
	}
	*a.hitSink = append(*a.hitSink, r)
}

// beginHits opens the frame's build set. Only a full Execute rebuilds regions:
// a paint-only frame proved nothing geometric moved, so the committed set stays.
func (a *App) beginHits() {
	if !a.mouse {
		return
	}
	a.hitBuilding = a.hitBuilding[:0]
	a.hitSink = &a.hitBuilding
}

// commitHits publishes the frame's regions for the input goroutine.
func (a *App) commitHits() {
	a.hitSink = nil
	if !a.mouse {
		return
	}
	a.hitMu.Lock()
	a.hitRoot, a.hitBuilding = a.hitBuilding, a.hitRoot
	a.hitMu.Unlock()
}

// hitTest finds the innermost region under (x, y) that want accepts. A layer
// region is searched inside before it counts itself, and a region that
// swallows always counts, so content under an overlay is unreachable.
func (a *App) hitTest(x, y int, want func(*hitRegion) bool) (hitRegion, bool) {
	a.hitMu.Lock()
	defer a.hitMu.Unlock()
	return hitIn(a.hitRoot, x, y, want)
}

func hitIn(regions []hitRegion, x, y int, want func(*hitRegion) bool) (hitRegion, bool) {
	r, _, ok := hitAt(regions, x, y, want, 0, 0, unclipped)
	return r, ok
}

// hitAt is hitIn carrying the screen origin of the regions' coordinate space
// and the clip accumulated from enclosing layer viewports, so the match comes
// back with its visible footprint in screen cells.
func hitAt(regions []hitRegion, x, y int, want func(*hitRegion) bool, ox, oy int, clip hitRect) (hitRegion, hitRect, bool) {
	for i := len(regions) - 1; i >= 0; i-- {
		r := &regions[i]
		if !r.contains(x, y) {
			continue
		}
		rect := hitRect{X: ox + int(r.X), Y: oy + int(r.Y), W: int(r.W), H: int(r.H)}.intersect(clip)
		if r.layer != nil {
			scroll := r.layer.DisplayedScrollY()
			lx := x - r.layer.screenX
			ly := y - r.layer.screenY + scroll
			if inner, innerRect, ok := hitAt(r.layer.hits, lx, ly, want, ox+r.layer.screenX, oy+r.layer.screenY-scroll, rect); ok {
				return inner, innerRect, true
			}
		}
		if want(r) || r.swallows() {
			return *r, rect, true
		}
	}
	return hitRegion{}, hitRect{}, false
}

func wantsTap(r *hitRegion) bool   { return r.tap != nil }
func wantsWheel(r *hitRegion) bool { return r.wheel != nil }
func wantsHover(r *hitRegion) bool { return r.hover != nil }

// handleMouse is the riffkey OnMouse hook. It runs on the input goroutine, so
// the region is copied out under hitMu and its handler called without it, the
// same way key handlers run.
func (a *App) handleMouse(m riffkey.Mouse) bool {
	if !a.mouse {
		return false
	}
	a.pointerAt(m.X, m.Y)
	switch m.Button {
	case riffkey.MouseWheelUp, riffkey.MouseWheelDown:
		delta := 1
		if m.Button == riffkey.MouseWheelUp {
			delta = -1
		}
		if r, ok := a.hitTest(m.X, m.Y, wantsWheel); ok && r.wheel != nil {
			r.wheel(delta)
			return true
		}
	case riffkey.MouseLeft:
		if m.Action != riffkey.MousePress {
			return false
		}
		if r, ok := a.hitTest(m.X, m.Y, wantsTap); ok && r.tap != nil {
			r.tap()
			return true
		}
	case riffkey.MouseNone:
		return m.Action == riffkey.MouseMotion
	}
	return false
}

// pointerAt records the pointer's cell. Hover is a pure function of that cell
// and the committed regions, so a move only costs a frame when the footprint
// it resolves to differs from the one on screen.
func (a *App) pointerAt(x, y int) {
	a.hitMu.Lock()
	defer a.hitMu.Unlock()
	if a.hoverKnown && a.hoverX == x && a.hoverY == y {
		return
	}
	a.hoverX, a.hoverY, a.hoverKnown = x, y, true
	rect, _, _, ok := a.hoverLocked()
	if ok == a.hoverShownOK && (!ok || rect == a.hoverShown) {
		return
	}
	a.hoverShown, a.hoverShownOK = rect, ok
	a.requestHoverFrame()
}

// hoverLocked resolves the hover footprint, style and mix for the current
// pointer cell. Caller holds hitMu.
func (a *App) hoverLocked() (hitRect, *Style, float64, bool) {
	if !a.hoverKnown {
		return hitRect{}, nil, 0, false
	}
	r, rect, ok := hitAt(a.hitRoot, a.hoverX, a.hoverY, wantsHover, 0, 0, unclipped)
	if !ok || r.hover == nil || rect.empty() {
		return hitRect{}, nil, 0, false
	}
	return rect, r.hover, float64(r.hoverMix), true
}

// paintHover lays the hovered target's style over the composed frame. It runs
// after effects and before jump labels, on every frame path, so a frame that
// moved content under a resting pointer restyles the right cells.
func (a *App) paintHover(buf *Buffer) {
	if !a.mouse {
		return
	}
	a.hitMu.Lock()
	rect, style, mix, ok := a.hoverLocked()
	a.hoverShown, a.hoverShownOK = rect, ok
	a.hitMu.Unlock()
	if !ok {
		return
	}
	// a wash needs a concrete colour under it: cells on the terminal default
	// take the detected default, then the explicit one, as the effects pass does
	under := Style{FG: a.defaultFG, BG: a.defaultBG}
	if under.FG.Mode == ColorDefault {
		under.FG = a.defaultStyle.FG
	}
	if under.BG.Mode == ColorDefault {
		under.BG = a.defaultStyle.BG
	}
	for y := rect.Y; y < rect.Y+rect.H; y++ {
		for x := rect.X; x < rect.X+rect.W; x++ {
			if !buf.InBounds(x, y) {
				continue
			}
			cell := buf.Get(x, y)
			cell.Style = overlayStyle(cell.Style, *style, mix, under)
			buf.Set(x, y, cell)
		}
	}
}

// overlayStyle lays over on base: attributes add, and set colours replace, or
// with 0 < mix < 1 wash base's colours toward them so a row keeps its chips
// and selection under the highlight. Fill stands in for BG so either spelling
// tints the cell; under supplies the colour beneath a terminal-default cell.
func overlayStyle(base, over Style, mix float64, under Style) Style {
	wash := mix > 0 && mix < 1
	if over.FG.Mode != ColorDefault {
		base.FG = washColor(base.FG, under.FG, over.FG, mix, wash)
	}
	bg := over.BG
	if bg.Mode == ColorDefault {
		bg = over.Fill
	}
	if bg.Mode != ColorDefault {
		from := base.BG
		if from.Mode == ColorDefault {
			from = base.Fill
		}
		base.BG = washColor(from, under.BG, bg, mix, wash)
		base.Fill = base.BG
	}
	base.Attr |= over.Attr
	return base
}

// washColor moves from toward to by mix when washing and the colour under the
// pointer is concrete; anything else takes to outright.
func washColor(from, under, to Color, mix float64, wash bool) Color {
	if !wash {
		return to
	}
	if from.Mode == ColorDefault {
		from = under
	}
	switch from.Mode {
	case Color16:
		rgb := basic16RGB[from.Index&0xF]
		from = RGB(rgb[0], rgb[1], rgb[2])
	case Color256:
		rgb := palette256RGB[from.Index]
		from = RGB(rgb[0], rgb[1], rgb[2])
	}
	if from.Mode != ColorRGB || to.Mode != ColorRGB {
		return to
	}
	return Lerp(from, to, mix)
}

// hoverMix normalises the optional mix argument of Hover: none or anything
// outside (0, 1) means replace.
func hoverMix(mix []float64) float32 {
	if len(mix) == 0 || mix[0] <= 0 || mix[0] >= 1 {
		return 0
	}
	return float32(mix[0])
}

// hoverStyle normalises a Hover argument: a Style is copied, a *Style is read
// each frame so a theme can restyle it in place, and a zero Style means none.
func hoverStyle(s any) *Style {
	switch v := s.(type) {
	case nil:
		return nil
	case Style:
		if v == (Style{}) {
			return nil
		}
		return &v
	case *Style:
		return v
	}
	panic("glyph: Hover wants a Style or *Style")
}
