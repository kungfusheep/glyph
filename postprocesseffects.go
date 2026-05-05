package glyph

import (
	"math"
	"time"
	"unsafe"
)

// dynFloat64 bundles a static float64 with optional dynamic source (pointer,
// condition, or tween). replaces the repeated 3-field pattern across effects.
type dynFloat64 struct {
	val   float64
	dyn   any
	ptr   *float64
	armed *bool // non-nil for From tweens — resolve() sets true, tween waits for it
	isSet bool
}

func (d *dynFloat64) set(v any) {
	d.isSet = true
	switch val := v.(type) {
	case float64:
		d.val = val
	case float32:
		d.val = float64(val)
	case int:
		d.val = float64(val)
	case *float64:
		d.dyn = val
	case conditionNode:
		d.dyn = val
	case tweenNode:
		d.dyn = val
	}
}

func (d *dynFloat64) compile(tmpl *Template) {
	if d.dyn != nil {
		d.ptr = tmpl.compileDynFloat64(d.dyn, nil, 0)
	}
}

// compileArmed is for screen effects whose Apply calls resolve(). From/Out
// tweens inside conditional branches are tied to that activation.
func (d *dynFloat64) compileArmed(tmpl *Template, elemBase unsafe.Pointer, elemSize uintptr) {
	if d.dyn == nil {
		return
	}
	if tw, ok := d.dyn.(tweenNode); ok && (tw.getTweenFrom() != nil || tw.getTweenOut() != nil) && tmpl.root != nil {
		d.armed = new(bool)
		d.ptr = tmpl.compileTweenFloat64(tw, d.armed, elemBase, elemSize)
	} else {
		d.ptr = tmpl.compileDynFloat64(d.dyn, elemBase, elemSize)
	}
}

func (d dynFloat64) resolve() float64 {
	if d.armed != nil {
		*d.armed = true
	}
	if d.ptr != nil {
		return *d.ptr
	}
	return d.val
}

// dynInt bundles a static int with optional dynamic source.
type dynInt struct {
	val int
	dyn any
	ptr *int16
}

func (d *dynInt) set(v any) {
	switch val := v.(type) {
	case int:
		d.val = val
	case int16:
		d.val = int(val)
	case *int16:
		d.dyn = val
	case conditionNode:
		d.dyn = val
	case tweenNode:
		d.dyn = val
	}
}

func (d *dynInt) compile(tmpl *Template) {
	if d.dyn != nil {
		d.ptr = tmpl.compileDynInt16(d.dyn, nil, 0)
	}
}

func (d dynInt) resolve() int {
	if d.ptr != nil {
		return int(*d.ptr)
	}
	return d.val
}

// dynColor bundles a static Color with optional dynamic source.
type dynColor struct {
	val Color
	dyn any
	ptr *Color
}

func (d *dynColor) set(v any) {
	switch val := v.(type) {
	case Color:
		d.val = val
	case *Color:
		d.dyn = val
	case conditionNode:
		d.dyn = val
	case tweenNode:
		d.dyn = val
	}
}

func (d *dynColor) compile(tmpl *Template) {
	if d.dyn != nil {
		d.ptr = tmpl.compileDynColor(d.dyn, nil, 0)
	}
}

func (d dynColor) resolve() Color {
	if d.ptr != nil {
		return *d.ptr
	}
	return d.val
}

// ---------------------------------------------------------------------------
// Subtle: one-liner polish for real apps
// ---------------------------------------------------------------------------

// SEDimAll applies the terminal Dim attribute to every cell.
// The simplest possible effect — one attribute, whole screen.
func SEDimAll() Effect {
	return EachCell(func(_, _ int, c Cell, _ PostContext) Cell {
		c.Style.Attr = c.Style.Attr.With(AttrDim)
		return c
	})
}

// tintEffect shifts all RGB colours toward a target colour.
type tintEffect struct {
	target   Color
	strength dynFloat64
	dodge    *NodeRef
}

// SETint shifts all RGB colours toward a target colour.
// Think colour grading: warm/cool/moody tones in one line.
// Default strength 0.15 — tasteful tint out of the box.
func SETint(color Color) tintEffect {
	return tintEffect{target: color, strength: dynFloat64{val: 0.15}}
}

// Strength sets how strongly the tint blends in (0.0 = none, 1.0 = full).
func (t tintEffect) Strength(s any) tintEffect { t.strength.set(s); return t }

// Dodge exempts the given node from tinting — useful for preserving a focused panel.
func (t tintEffect) Dodge(ref *NodeRef) tintEffect { t.dodge = ref; return t }

func (t tintEffect) compileEffect(tmpl *Template) Effect {
	t.strength.compileArmed(tmpl, nil, 0)
	return t
}

func (t tintEffect) Apply(buf *Buffer, ctx PostContext) {
	s := t.strength.resolve()
	EachCell(func(x, y int, c Cell, ectx PostContext) Cell {
		if t.dodge != nil && inRect(x, y, t.dodge) {
			return c
		}
		c.Style.FG = lerpIfRGB(resolveFG(c.Style.FG, ectx), t.target, s)
		c.Style.BG = lerpIfRGB(resolveBG(c.Style.BG, ectx), t.target, s)
		return c
	}).Apply(buf, ctx)
}

// vignetteEffect darkens cells toward the screen edges.
type vignetteEffect struct {
	strength dynFloat64
	focus    *NodeRef
	dodge    *NodeRef
	quantize bool
}

// SEVignette darkens cells near the screen edges.
// Quadratic falloff for a natural cinematic feel. Default strength 0.8.
func SEVignette() vignetteEffect {
	return vignetteEffect{strength: dynFloat64{val: 0.8}, quantize: true}
}

// Strength sets edge darkening intensity (0.0 = no effect, 1.0 = full black at edges).
func (v vignetteEffect) Strength(s any) vignetteEffect { v.strength.set(s); return v }

func (v vignetteEffect) compileEffect(tmpl *Template) Effect {
	v.strength.compileArmed(tmpl, nil, 0)
	return v
}

// Focus centres the vignette on the given node.
func (v vignetteEffect) Focus(ref *NodeRef) vignetteEffect { v.focus = ref; return v }

// Dodge exempts the given node from darkening.
func (v vignetteEffect) Dodge(ref *NodeRef) vignetteEffect { v.dodge = ref; return v }

// Smooth disables quantization for a continuous gradient (slightly more escape output).
func (v vignetteEffect) Smooth() vignetteEffect { v.quantize = false; return v }

func (v vignetteEffect) Apply(buf *Buffer, ctx PostContext) {
	black := Color{Mode: ColorRGB}
	var cx, cy float64
	if v.focus != nil {
		cx = float64(v.focus.X) + float64(v.focus.W)/2
		cy = float64(v.focus.Y) + float64(v.focus.H)/2
	} else {
		cx = float64(ctx.Width) / 2
		cy = float64(ctx.Height) / 2
	}
	// maxDist = distance from center to the farthest screen corner, aspect-compensated.
	// using max extents handles off-center focus nodes correctly.
	maxX := math.Max(cx, float64(ctx.Width)-cx)
	maxY := math.Max(cy, float64(ctx.Height)-cy) * 2
	maxDist := math.Sqrt(maxX*maxX + maxY*maxY)
	dodgeOpacity := 0.0
	if v.dodge != nil {
		opacity := refOpacity(v.dodge)
		dodgeOpacity = opacity * opacity
	}

	for y := range ctx.Height {
		base := y * buf.width
		dy := (float64(y) - cy) * 2
		for x := range ctx.Width {
			dx := float64(x) - cx
			dist := math.Sqrt(dx*dx+dy*dy) / maxDist
			dim := dist * dist * v.strength.resolve()
			if v.dodge != nil {
				dim *= 1 - dodgeOpacity*vignetteDodgeWeight(x, y, v.dodge)
			}
			if dim > 1 {
				dim = 1
			}
			// snap to 32 levels — imperceptible banding, collapses escape output
			if v.quantize {
				dim = math.Round(dim*32) / 32
			}
			idx := base + x
			c := &buf.cells[idx]
			c.Style.FG = lerpIfRGB(resolveFG(c.Style.FG, ctx), black, dim)
			c.Style.BG = lerpIfRGB(resolveBG(c.Style.BG, ctx), black, dim)
		}
	}
}

func vignetteDodgeWeight(x, y int, ref *NodeRef) float64 {
	if ref == nil {
		return 0
	}
	if inRect(x, y, ref) {
		return 1
	}
	const feather = 4.0
	dx := 0
	if x < ref.X {
		dx = ref.X - x
	} else if x >= ref.X+ref.W {
		dx = x - (ref.X + ref.W - 1)
	}
	dy := 0
	if y < ref.Y {
		dy = ref.Y - y
	} else if y >= ref.Y+ref.H {
		dy = y - (ref.Y + ref.H - 1)
	}
	dist := math.Sqrt(float64(dx*dx + dy*dy))
	if dist >= feather {
		return 0
	}
	t := dist / feather
	return 1 - t*t*(3-2*t)
}

// desaturateEffect removes colour saturation from all RGB cells.
type desaturateEffect struct {
	strength dynFloat64
	dodge    *NodeRef
}

// SEDesaturate removes colour saturation from all RGB cells.
// Uses perceptual luminance weights (BT.601). Default strength 0.7.
func SEDesaturate() desaturateEffect { return desaturateEffect{strength: dynFloat64{val: 0.7}} }

// Strength sets how much to desaturate (0.0 = full colour, 1.0 = fully grey).
func (d desaturateEffect) Strength(s any) desaturateEffect { d.strength.set(s); return d }

func (d desaturateEffect) compileEffect(tmpl *Template) Effect {
	d.strength.compileArmed(tmpl, nil, 0)
	return d
}

// Dodge exempts the given node — the classic "colour spotlight" on a grey world.
func (d desaturateEffect) Dodge(ref *NodeRef) desaturateEffect { d.dodge = ref; return d }

func (d desaturateEffect) Apply(buf *Buffer, ctx PostContext) {
	s := d.strength.resolve()
	EachCell(func(x, y int, c Cell, ectx PostContext) Cell {
		if d.dodge != nil && inRect(x, y, d.dodge) {
			return c
		}
		c.Style.FG = desaturateColor(resolveFG(c.Style.FG, ectx), s)
		c.Style.BG = desaturateColor(resolveBG(c.Style.BG, ectx), s)
		return c
	}).Apply(buf, ctx)
}

// contrastEffect boosts contrast by pushing colour channels toward extremes.
type contrastEffect struct {
	strength dynFloat64
	dodge    *NodeRef
}

// SEContrast boosts contrast by pushing colour channels toward extremes.
// Default strength 1.5 — noticeable punch without going stark.
func SEContrast() contrastEffect { return contrastEffect{strength: dynFloat64{val: 1.5}} }

// Strength sets the contrast boost factor (1.0 = noticeable, 3.0+ = stark black/white).
func (h contrastEffect) Strength(s any) contrastEffect { h.strength.set(s); return h }

func (h contrastEffect) compileEffect(tmpl *Template) Effect {
	h.strength.compileArmed(tmpl, nil, 0)
	return h
}

// Dodge exempts the given node from contrast adjustment.
func (h contrastEffect) Dodge(ref *NodeRef) contrastEffect { h.dodge = ref; return h }

func (h contrastEffect) Apply(buf *Buffer, ctx PostContext) {
	s := h.strength.resolve()
	EachCell(func(x, y int, c Cell, ectx PostContext) Cell {
		if h.dodge != nil && inRect(x, y, h.dodge) {
			return c
		}
		c.Style.FG = boostContrast(resolveFG(c.Style.FG, ectx), s)
		c.Style.BG = boostContrast(resolveBG(c.Style.BG, ectx), s)
		return c
	}).Apply(buf, ctx)
}

// ---------------------------------------------------------------------------
// Medium: noticeable, purposeful
// ---------------------------------------------------------------------------

// focusDimEffect dims everything outside the bounds of a NodeRef.
type focusDimEffect struct{ ref *NodeRef }

// SEFocusDim dims everything outside the bounds of a NodeRef.
// The ref is populated each frame after layout, so it tracks the node automatically.
func SEFocusDim(ref *NodeRef) focusDimEffect { return focusDimEffect{ref: ref} }

func (f focusDimEffect) Apply(buf *Buffer, ctx PostContext) {
	rx, ry := f.ref.X, f.ref.Y
	rw, rh := f.ref.W, f.ref.H

	for y := range ctx.Height {
		base := y * buf.width
		inY := y >= ry && y < ry+rh
		for x := range ctx.Width {
			if inY && x >= rx && x < rx+rw {
				continue
			}
			buf.cells[base+x].Style.Attr = buf.cells[base+x].Style.Attr.With(AttrDim)
		}
	}
}

type pulseEffect struct {
	speed    dynFloat64
	strength dynFloat64
}

func SEPulse() pulseEffect {
	return pulseEffect{speed: dynFloat64{val: 1.0}, strength: dynFloat64{val: 0.3}}
}

// Speed sets oscillation frequency in cycles per second.
func (p pulseEffect) Speed(s any) pulseEffect { p.speed.set(s); return p }

// Strength sets how much brightness dims at the trough (0.3 = subtle, 0.8 = dramatic).
func (p pulseEffect) Strength(s any) pulseEffect { p.strength.set(s); return p }

func (p pulseEffect) compileEffect(tmpl *Template) Effect {
	p.speed.compileArmed(tmpl, nil, 0)
	p.strength.compileArmed(tmpl, nil, 0)
	return p
}

func (p pulseEffect) Apply(buf *Buffer, ctx PostContext) {
	black := Color{Mode: ColorRGB}
	t := (math.Sin(ctx.Time.Seconds()*p.speed.resolve()*math.Pi*2) + 1) * 0.5
	dim := t * p.strength.resolve()

	for y := range ctx.Height {
		base := y * buf.width
		for x := range ctx.Width {
			idx := base + x
			c := &buf.cells[idx]
			c.Style.FG = lerpIfRGB(resolveFG(c.Style.FG, ctx), black, dim)
			c.Style.BG = lerpIfRGB(resolveBG(c.Style.BG, ctx), black, dim)
		}
	}
}

// gradientMapEffect remaps all colour luminance through a three-stop gradient.
type gradientMapEffect struct{ dark, mid, bright Color }

// SEGradientMap remaps all colour luminance through a three-stop gradient.
// Dark shades map to the first colour, midtones to the second, highlights to the third.
func SEGradientMap(dark, mid, bright Color) gradientMapEffect {
	return gradientMapEffect{dark: dark, mid: mid, bright: bright}
}

func (g gradientMapEffect) Apply(buf *Buffer, ctx PostContext) {
	EachCell(func(_, _ int, c Cell, ectx PostContext) Cell {
		c.Style.FG = gradientMap(resolveFG(c.Style.FG, ectx), g.dark, g.mid, g.bright)
		c.Style.BG = gradientMap(resolveBG(c.Style.BG, ectx), g.dark, g.mid, g.bright)
		return c
	}).Apply(buf, ctx)
}

// ---------------------------------------------------------------------------
// Visual flair
// ---------------------------------------------------------------------------

// dropShadowEffect is a glow/drop-shadow — the inverse of vignette.
// Where vignette darkens from the screen edges inward, this darkens outward
// from a focus node's perimeter. At offset (0,0) it's a symmetric glow.
// Any offset displaces the shadow source, giving a directional drop shadow.
type dropShadowEffect struct {
	strength    dynFloat64
	opacity     dynFloat64
	radius      dynInt
	offsetX     int
	offsetY     int
	tint        dynColor
	opacityMode OpacityMode
	focus       *NodeRef
}

// SEDropShadow creates a radial glow/shadow emanating outward from a focus node.
// Default: radius 8, strength 0.2, offset (-1,-1) for a subtle directional shadow.
// Chain .Focus(&ref) to set the source node, .Offset(x,y) to adjust direction.
func SEDropShadow() dropShadowEffect {
	return dropShadowEffect{
		strength:    dynFloat64{val: 0.2},
		opacity:     dynFloat64{val: 1.0},
		radius:      dynInt{val: 8},
		offsetX:     -1,
		offsetY:     -1,
		tint:        dynColor{val: Color{Mode: ColorRGB}},
		opacityMode: OpacitySmooth,
	}
}

// Strength sets shadow darkness (0.0 = none, 1.0 = full black at source edge).
func (d dropShadowEffect) Strength(s any) dropShadowEffect { d.strength.set(s); return d }

func (d dropShadowEffect) compileEffect(tmpl *Template) Effect {
	d.strength.compileArmed(tmpl, nil, 0)
	d.opacity.compileArmed(tmpl, nil, 0)
	d.radius.compile(tmpl)
	d.tint.compile(tmpl)
	return d
}

// Opacity sets the compositor opacity for the shadow surface. The focused
// node's own opacity is also applied automatically, so shadows fade with the
// thing that casts them.
func (d dropShadowEffect) Opacity(o any) dropShadowEffect { d.opacity.set(o); return d }

// OpacityMode sets how shadow cells hand back to backing runes during fades.
func (d dropShadowEffect) OpacityMode(mode OpacityMode) dropShadowEffect {
	d.opacityMode = mode
	return d
}

// Radius sets how far the shadow spreads in cells.
func (d dropShadowEffect) Radius(r any) dropShadowEffect { d.radius.set(r); return d }

// Offset displaces the shadow source — turns the symmetric glow into a directional drop shadow.
func (d dropShadowEffect) Offset(x, y int) dropShadowEffect { d.offsetX = x; d.offsetY = y; return d }

// Tint sets the shadow colour (default black).
func (d dropShadowEffect) Tint(c any) dropShadowEffect { d.tint.set(c); return d }

// Focus sets the node the shadow emanates from.
func (d dropShadowEffect) Focus(ref *NodeRef) dropShadowEffect { d.focus = ref; return d }

func (d dropShadowEffect) Apply(buf *Buffer, ctx PostContext) {
	if d.focus == nil {
		return
	}

	ref := d.focus
	radius := float64(d.radius.resolve())
	effectOpacity := clampOpacity(refOpacity(ref) * d.opacity.resolve())
	strength := d.strength.resolve()
	if strength <= 0 || radius <= 0 {
		return
	}
	radiusI := int(math.Ceil(radius))
	sx, sy := ref.X+d.offsetX, ref.Y+d.offsetY
	minX := max(0, sx-radiusI)
	maxX := min(ctx.Width, sx+ref.W+radiusI)
	minY := max(0, sy-radiusI)
	maxY := min(ctx.Height, sy+ref.H+radiusI)

	for y := minY; y < maxY; y++ {
		if y > buf.dirtyMaxY {
			buf.dirtyMaxY = y
		}
		buf.dirtyRows[y] = true
	}
	if effectOpacity <= 0 {
		return
	}

	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			if inRect(x, y, ref) {
				continue
			}
			cx := max(sx, min(x, sx+ref.W-1))
			cy := max(sy, min(y, sy+ref.H-1))
			dx := float64(x - cx)
			dy := float64(y-cy) * 2
			dist := math.Sqrt(dx*dx + dy*dy)

			if dist >= radius {
				continue
			}

			t := 1.0 - dist/radius
			dim := t * t * strength * effectOpacity

			tintColor := d.tint.resolve()
			c := &buf.cells[y*buf.width+x]
			c.Style.FG = lerpIfRGB(resolveFG(c.Style.FG, ctx), tintColor, dim)
			c.Style.BG = lerpIfRGB(resolveBG(c.Style.BG, ctx), tintColor, dim)
		}
	}
}

// glowEffect emanates light outward from a focus node, sampling the node's
// edge colours and boosting them — the glow takes on the colour of the content.
type glowEffect struct {
	strength   dynFloat64
	radius     dynInt
	brightness dynFloat64
	focus      *NodeRef
}

// SEGlow creates a colour-sampling glow that reads the focus node's edge pixels
// and spills a brightened version of those colours into the surrounding area.
// Default: radius 8, strength 0.5, brightness 1.4.
func SEGlow() glowEffect {
	return glowEffect{
		strength:   dynFloat64{val: 0.5},
		radius:     dynInt{val: 8},
		brightness: dynFloat64{val: 1.4},
	}
}

// Strength sets how strongly the glow blends into surrounding cells.
func (g glowEffect) Strength(s any) glowEffect { g.strength.set(s); return g }

func (g glowEffect) compileEffect(tmpl *Template) Effect {
	g.strength.compileArmed(tmpl, nil, 0)
	g.radius.compile(tmpl)
	g.brightness.compileArmed(tmpl, nil, 0)
	return g
}

// Radius sets how far the glow spreads in cells.
func (g glowEffect) Radius(r any) glowEffect { g.radius.set(r); return g }

// Brightness sets the boost applied to sampled edge colours (1.0 = no boost).
func (g glowEffect) Brightness(b any) glowEffect { g.brightness.set(b); return g }

// Focus sets the node the glow emanates from.
func (g glowEffect) Focus(ref *NodeRef) glowEffect { g.focus = ref; return g }

func (g glowEffect) Apply(buf *Buffer, ctx PostContext) {
	if g.focus == nil {
		return
	}

	ref := g.focus
	radius := float64(g.radius.resolve())
	strength := g.strength.resolve() * refOpacity(ref)
	if strength <= 0 {
		return
	}

	for y := range ctx.Height {
		base := y * buf.width
		for x := range ctx.Width {
			if inRect(x, y, ref) {
				continue
			}

			ex := max(ref.X, min(x, ref.X+ref.W-1))
			ey := max(ref.Y, min(y, ref.Y+ref.H-1))

			dx := float64(x - ex)
			dy := float64(y-ey) * 2
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist >= radius {
				continue
			}

			edge := buf.Get(ex, ey)
			sample := resolveBG(edge.Style.BG, ctx)
			if sample.Mode != ColorRGB {
				continue
			}

			bright := g.brightness.resolve()
			boosted := Color{
				Mode: ColorRGB,
				R:    uint8(min(int(float64(sample.R)*bright), 255)),
				G:    uint8(min(int(float64(sample.G)*bright), 255)),
				B:    uint8(min(int(float64(sample.B)*bright), 255)),
			}

			t := 1.0 - dist/radius
			blend := t * t * strength

			c := &buf.cells[base+x]
			c.Style.FG = lerpIfRGB(resolveFG(c.Style.FG, ctx), boosted, blend)
			c.Style.BG = lerpIfRGB(resolveBG(c.Style.BG, ctx), boosted, blend)
		}
	}
}

// spinGlowEffect emanates a coloured halo around a focus node and rotates it.
// N palette colours become N evenly-spaced stops around the ring, lerped between
// neighbours. Angular intensity is also modulated so a bright arc traces the
// perimeter — even a single-colour palette visibly spins.
type spinGlowEffect struct {
	strength    dynFloat64
	opacity     dynFloat64
	radius      dynInt
	speed       dynFloat64
	falloff     dynFloat64
	opacityMode OpacityMode
	palette     []Color
	phase       *spinGlowPhase
	// paletteRef, when non-nil, is dereferenced per-frame in Apply so the
	// effect's palette can change at runtime by reassigning the slice the
	// caller holds. Wins over `palette` when both are set.
	paletteRef *[]Color
	focus      *NodeRef
	rim        bool
}

type spinGlowPhase struct {
	value       float64
	lastTime    time.Duration
	initialized bool
}

func (p *spinGlowPhase) advance(ctx PostContext, speed float64) float64 {
	if !p.initialized {
		p.initialized = true
		p.lastTime = ctx.Time
		return p.value
	}

	delta := ctx.Delta
	if delta <= 0 && ctx.Time > p.lastTime {
		delta = ctx.Time - p.lastTime
	}
	p.lastTime = ctx.Time
	p.value += delta.Seconds() * speed
	return p.value
}

// defaultSpinGlowPalette — pink → rose → purple, matching the glyph website vibe.
var defaultSpinGlowPalette = []Color{
	{Mode: ColorRGB, R: 255, G: 80, B: 120},
	{Mode: ColorRGB, R: 255, G: 140, B: 100},
	{Mode: ColorRGB, R: 200, G: 100, B: 255},
}

// SESpinGlow creates a rotating radial halo around a focus node. Pass zero
// colours for the default palette, or one+ colours to define a conic gradient
// of stops that rotates with time.
//
//	SESpinGlow(&ref)                                 // default palette
//	SESpinGlow(&ref, RGB(255, 80, 120))              // single-tint hotspot
//	SESpinGlow(&ref, RGB(255,80,120), RGB(150,100,255)) // two-stop swirl
func SESpinGlow(focus *NodeRef, palette ...Color) spinGlowEffect {
	if len(palette) == 0 {
		palette = defaultSpinGlowPalette
	}
	return spinGlowEffect{
		strength:    dynFloat64{val: 0.7},
		opacity:     dynFloat64{val: 1.0},
		radius:      dynInt{val: 10},
		speed:       dynFloat64{val: 2.1}, // ~360° / 3s, matches glyph-website install pill
		falloff:     dynFloat64{val: 1.0}, // deviation from linear; 1 = quadratic
		opacityMode: OpacityPaint,
		palette:     palette,
		focus:       focus,
	}
}

// Strength sets how strongly the glow blends into surrounding cells (0..1).
func (s spinGlowEffect) Strength(v any) spinGlowEffect { s.strength.set(v); return s }

// Opacity sets the compositor opacity for the whole spin glow effect. Unlike
// Strength, this is intended for fades: it scales the halo and the foreground
// rim without changing the fully-on rim solidity.
func (s spinGlowEffect) Opacity(v any) spinGlowEffect { s.opacity.set(v); return s }

// OpacityMode sets how rim cells hand back to backing runes as effect opacity
// fades. The default is OpacityPaint so a mostly-opaque rim remains solid.
func (s spinGlowEffect) OpacityMode(mode OpacityMode) spinGlowEffect {
	s.opacityMode = mode
	return s
}

// Radius sets how far the glow spreads in cells.
func (s spinGlowEffect) Radius(v any) spinGlowEffect { s.radius.set(v); return s }

// Speed sets rotation speed in radians per second. Zero freezes the glow.
func (s spinGlowEffect) Speed(v any) spinGlowEffect { s.speed.set(v); return s }

// Falloff shapes the halo's intensity curve between the rect and the radius.
// 0 = linear 1→0 (baseline). Higher values concentrate the drop-off closer
// to the rect — intensity falls quickly just outside the rect, then trails
// off more gently toward the radius. The curve is blended 50/50 with a
// linear term so the halo stays visible all the way to the radius at every
// Falloff value — Radius is declarative, it's always the visible disappearing
// point regardless of the curve shape. fast-pathed for integer values 0..3.
func (s spinGlowEffect) Falloff(v any) spinGlowEffect { s.falloff.set(v); return s }

// Rim enables painting the focus node's border perimeter with the rotating
// conic palette — analogous to a CSS conic-gradient stroke on a border.
// Overrides existing border FG. Use on containers with a visible border for
// a rotating-rim look; pair with a small Radius for "rim with a soft outer
// halo" that mirrors the glyph-website install pill.
func (s spinGlowEffect) Rim(v bool) spinGlowEffect { s.rim = v; return s }

// PaletteRef wires a per-frame palette source. The pointer is dereferenced
// each Apply, so callers can swap the palette live just by reassigning the
// slice variable the pointer addresses — no effect rebuild required.
// When set, supersedes the palette passed at construction.
func (s spinGlowEffect) PaletteRef(p *[]Color) spinGlowEffect {
	s.paletteRef = p
	return s
}

func (s spinGlowEffect) compileEffect(tmpl *Template) Effect {
	s.strength.compileArmed(tmpl, nil, 0)
	s.opacity.compileArmed(tmpl, nil, 0)
	s.radius.compile(tmpl)
	s.speed.compileArmed(tmpl, nil, 0)
	s.falloff.compileArmed(tmpl, nil, 0)
	s.phase = &spinGlowPhase{}
	return s
}

func (s spinGlowEffect) Apply(buf *Buffer, ctx PostContext) {
	palette := s.palette
	if s.paletteRef != nil && len(*s.paletteRef) > 0 {
		palette = *s.paletteRef
	}
	if s.focus == nil || len(palette) == 0 {
		return
	}

	ref := s.focus
	radius := float64(s.radius.resolve())
	if radius <= 0 {
		return
	}
	effectOpacity := clampOpacity(refOpacity(ref) * s.opacity.resolve())
	if effectOpacity <= 0 {
		return
	}
	strength := s.strength.resolve() * effectOpacity
	speed := s.speed.resolve()
	phase := ctx.Time.Seconds() * speed
	if s.phase != nil {
		phase = s.phase.advance(ctx, speed)
	}
	fall := s.falloff.resolve()

	// adaptive blend between a linear baseline and the power curve. the
	// linear weight shrinks as falloff grows — so higher falloff lets the
	// power curve dominate the shape — but is floored at 0.15 so the tail
	// stays visibly reaching the radius at any falloff value.
	linW := 1.0 / (1.0 + fall)
	if linW < 0.15 {
		linW = 0.15
	}
	powW := 1.0 - linW

	n := float64(len(palette))

	// clip iteration to the bounding box of the halo's reach. cells far
	// outside can never satisfy `dist < radius`, so scanning them is wasted
	// work.
	radiusI := int(math.Ceil(radius))
	minX := max(0, ref.X-radiusI)
	maxX := min(ctx.Width, ref.X+ref.W+radiusI)
	minY := max(0, ref.Y-radiusI)
	maxY := min(ctx.Height, ref.Y+ref.H+radiusI)

	if strength > 0 {
		for y := minY; y < maxY; y++ {
			base := y * buf.width
			rowPainted := false
			for x := minX; x < maxX; x++ {
				if inRect(x, y, ref) {
					continue
				}

				// radial distance from the nearest rect edge (same as drop shadow).
				ex := max(ref.X, min(x, ref.X+ref.W-1))
				ey := max(ref.Y, min(y, ref.Y+ref.H-1))
				dxEdge := float64(x - ex)
				dyEdge := float64(y - ey)
				dist := math.Sqrt(dxEdge*dxEdge + dyEdge*dyEdge)
				if dist >= radius {
					continue
				}

				c := &buf.cells[base+x]

				// Project halo cells onto the same rectangular perimeter path
				// as the rim. A polar angle around a wide, short rect makes
				// nearby x/y cells sample visibly different bands.
				tint := sampleSpinGlowRectColor(x, y, ref, phase, n, palette)

				// radial falloff: linW*t + powW*t^(1+fall). higher fall pulls
				// the drop-off closer to the rect, while the linear term keeps
				// the halo visible all the way to the radius — so the declared
				// Radius is always the visual disappearing point, regardless
				// of curve shape. integer values 0..3 are fast-pathed to avoid
				// math.Pow. rotation signal comes purely from palette position
				// shift, not intensity modulation.
				t := 1.0 - dist/radius
				var radial float64
				switch fall {
				case 0.0:
					radial = t
				case 1.0:
					radial = linW*t + powW*t*t
				case 2.0:
					radial = linW*t + powW*t*t*t
				case 3.0:
					t2 := t * t
					radial = linW*t + powW*t2*t2
				default:
					radial = linW*t + powW*math.Pow(t, 1+fall)
				}
				blend := radial * strength

				c.Style.FG = lerpIfRGB(resolveFG(c.Style.FG, ctx), tint, blend)
				c.Style.BG = lerpIfRGB(resolveBG(c.Style.BG, ctx), tint, blend)
				rowPainted = true
			}
			// we mutate cells by direct pointer, so the buffer's per-row dirty
			// tracking never sees these writes. without marking, ClearDirty()
			// would skip the rows we painted (if they're past rendered content)
			// and paint would accumulate across frames. mark only the rows we
			// actually touched — keeps the per-row-clear optimisation intact.
			if rowPainted {
				if y > buf.dirtyMaxY {
					buf.dirtyMaxY = y
				}
				buf.dirtyRows[y] = true
			}
		}
	}

	if s.rim {
		s.paintRim(buf, ctx, phase, n, palette, effectOpacity)
	}
}

func sampleSpinGlowRectColor(x, y int, ref *NodeRef, phase, n float64, palette []Color) Color {
	top := ref.Y - 1
	bottom := ref.Y + ref.H
	left := ref.X - 1
	right := ref.X + ref.W

	width := right - left + 1
	side := bottom - top - 1
	total := 2*width + 2*side
	if total <= 0 {
		return palette[0]
	}

	px := min(max(x, left), right)
	py := min(max(y, top), bottom)

	if x < left {
		px = left
	} else if x > right {
		px = right
	}
	if y < top {
		py = top
	} else if y > bottom {
		py = bottom
	}

	var idx int
	switch {
	case py == top:
		idx = px - left
	case px == right:
		idx = width + (py - top - 1)
	case py == bottom:
		idx = width + side + (right - px)
	default:
		idx = width + side + width + (bottom - py - 1)
	}

	return sampleSpinGlowPerimeterIndex(idx, total, phase, n, palette)
}

func sampleSpinGlowPerimeterIndex(idx, total int, phase, n float64, palette []Color) Color {
	phaseNorm := phase / (2 * math.Pi)
	norm := float64(idx)/float64(total) - phaseNorm
	norm -= math.Floor(norm)
	p := norm * n
	i0 := int(p) % len(palette)
	i1 := (i0 + 1) % len(palette)
	frac := p - math.Floor(p)
	return LerpColor(palette[i0], palette[i1], frac)
}

// paintRim draws a 1-cell-thick coloured stroke OUTSIDE the focus node.
// Each perimeter cell samples the palette by distance along the painted
// rectangle, not by atan2 from the centre. That keeps colour changes even
// across coarse terminal cells, especially on wide/short cards where polar
// sampling makes adjacent x/y edge cells jump through very different parts
// of the gradient.
func (s spinGlowEffect) paintRim(buf *Buffer, ctx PostContext, phase, n float64, palette []Color, opacity float64) {
	ref := s.focus
	top := ref.Y - 1
	bottom := ref.Y + ref.H
	left := ref.X - 1
	right := ref.X + ref.W

	total := 2*(right-left+1) + 2*(bottom-top-1)
	if total <= 0 {
		return
	}

	sample := func(idx int) Color {
		return sampleSpinGlowPerimeterIndex(idx, total, phase, n, palette)
	}

	paintHalfBlock := func(x, y int, r rune, idx int) {
		if x < 0 || x >= ctx.Width || y < 0 || y >= ctx.Height {
			return
		}
		tint := sample(idx)

		// paintRim owns the perimeter cells: always write the block
		// rune, otherwise text in those cells stays visible underneath
		// the rim's tint and the rim looks broken. Leave BG alone: the
		// block glyph carries the stroke geometry, while BG is a full-cell
		// fill and would make half-height top/bottom strokes look too thick.
		buf.SetOpacity(x, y, Cell{Rune: r, Style: Style{FG: tint, Attr: AttrNone}}, opacity, s.opacityMode)
		// c.Style.BG = Magenta <- set this and you will screw up the border
	}

	paintFullCell := func(x, y int, idx int) {
		if x < 0 || x >= ctx.Width || y < 0 || y >= ctx.Height {
			return
		}
		tint := sample(idx)
		buf.SetOpacity(x, y, Cell{Rune: ' ', Style: Style{BG: tint, Attr: AttrNone}}, opacity, s.opacityMode)
		if opacity >= 1 {
			c := buf.Get(x, y)
			c.Rune = ' '
			c.Style.FG = Color{}
			c.Style.BG = tint
			c.Style.Attr = AttrNone
			buf.SetFast(x, y, c)
		}
	}

	// Top/bottom use half-height foreground blocks. Left/right are full-cell
	// geometry, so paint them through BG instead; this avoids foreground block
	// blending making the vertical bars read brighter or dimmer than the rim.
	idx := 0
	for x := left; x <= right; x++ {
		paintHalfBlock(x, top, '▄', idx)
		idx++
	}
	for y := top + 1; y < bottom; y++ {
		paintFullCell(right, y, idx)
		idx++
	}
	for x := right; x >= left; x-- {
		paintHalfBlock(x, bottom, '▀', idx)
		idx++
	}
	for y := bottom - 1; y > top; y-- {
		paintFullCell(left, y, idx)
		idx++
	}

	for y := top; y <= bottom; y++ {
		if y < 0 || y >= ctx.Height {
			continue
		}
		if y > buf.dirtyMaxY {
			buf.dirtyMaxY = y
		}
		buf.dirtyRows[y] = true
	}
}

// bloomEffect creates a coloured glow around bright cells.
type bloomEffect struct {
	radius    dynInt
	threshold dynFloat64
	strength  dynFloat64
	focus     *NodeRef
}

// SEBloom creates a coloured glow around bright cells.
// Bleeds bright colours into both FG and BG of surrounding cells.
// Default: radius 2, threshold 0.6, strength 0.3.
func SEBloom() bloomEffect {
	return bloomEffect{
		radius:    dynInt{val: 2},
		threshold: dynFloat64{val: 0.6},
		strength:  dynFloat64{val: 0.3},
	}
}

// Radius sets the spread in cells (2-4 recommended).
func (b bloomEffect) Radius(r any) bloomEffect { b.radius.set(r); return b }

// Threshold sets the minimum brightness that blooms (0.0–1.0).
func (b bloomEffect) Threshold(t any) bloomEffect { b.threshold.set(t); return b }

// Strength sets glow intensity (0.3 = subtle, 1.0 = vivid).
func (b bloomEffect) Strength(s any) bloomEffect { b.strength.set(s); return b }

func (b bloomEffect) compileEffect(tmpl *Template) Effect {
	b.radius.compile(tmpl)
	b.threshold.compileArmed(tmpl, nil, 0)
	b.strength.compileArmed(tmpl, nil, 0)
	return b
}

// Focus constrains bloom output to the given node — only cells within the rect receive glow.
func (b bloomEffect) Focus(ref *NodeRef) bloomEffect { b.focus = ref; return b }

func (b bloomEffect) Apply(buf *Buffer, ctx PostContext) {
	bw, bh := ctx.Width, ctx.Height
	// snapshot raw FG colours — do NOT resolve ColorDefault to terminal FG here.
	// only cells with explicit colours (ColorRGB, Color16) should act as sources.
	snap := make([]Color, bw*bh)
	for y := range bh {
		bufBase := y * buf.width
		snapBase := y * bw
		for x := range bw {
			snap[snapBase+x] = buf.cells[bufBase+x].Style.FG
		}
	}

	radius := b.radius.resolve()
	thresh := b.threshold.resolve()
	strength := b.strength.resolve()
	if b.focus != nil {
		strength *= refOpacity(b.focus)
	}
	if strength <= 0 {
		return
	}
	thresh256 := thresh * 255
	maxDist := math.Sqrt(float64(radius*radius) + float64(radius*radius)*4)

	// constrain output to focus rect if set
	x0, y0, x1, y1 := 0, 0, bw, bh
	if b.focus != nil {
		x0 = max(0, b.focus.X)
		y0 = max(0, b.focus.Y)
		x1 = min(bw, b.focus.X+b.focus.W)
		y1 = min(bh, b.focus.Y+b.focus.H)
	}

	for y := y0; y < y1; y++ {
		base := y * buf.width
		for x := x0; x < x1; x++ {
			var sumR, sumG, sumB, sumWt float64

			for dy := -radius; dy <= radius; dy++ {
				ny := y + dy
				if ny < 0 || ny >= bh {
					continue
				}
				for dx := -radius; dx <= radius; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx := x + dx
					if nx < 0 || nx >= bw {
						continue
					}
					nc := snap[ny*bw+nx]
					lum := 0.299*float64(nc.R) + 0.587*float64(nc.G) + 0.114*float64(nc.B)
					if lum <= thresh256 {
						continue
					}

					// quadratic falloff, aspect-ratio compensated
					dist := math.Sqrt(float64(dx*dx) + float64(dy*dy)*4)
					falloff := 1.0 - dist/maxDist
					if falloff <= 0 {
						continue
					}
					falloff *= falloff

					excess := (lum - thresh256) / (255 - thresh256)
					wt := falloff * excess
					sumR += float64(nc.R) * wt
					sumG += float64(nc.G) * wt
					sumB += float64(nc.B) * wt
					sumWt += wt
				}
			}

			if sumWt > 0 {
				bloom := RGB(
					uint8(min(255, sumR/sumWt)),
					uint8(min(255, sumG/sumWt)),
					uint8(min(255, sumB/sumWt)),
				)
				blend := (sumWt / (sumWt + 1)) * strength
				c := &buf.cells[base+x]
				c.Style.FG = lerpIfRGB(resolveFG(c.Style.FG, ctx), bloom, blend)
				c.Style.BG = lerpIfRGB(resolveBG(c.Style.BG, ctx), bloom, blend*0.3)
			}
		}
	}
}

// monochromeEffect converts all colours to a single-tint monochrome.
type monochromeEffect struct {
	tint  Color
	dodge *NodeRef
}

// SEMonochrome converts all colours to a single-tint monochrome.
// Pass RGB(0, 255, 0) for green phosphor, RGB(255, 180, 0) for amber.
func SEMonochrome(tint Color) monochromeEffect { return monochromeEffect{tint: tint} }

// Dodge exempts the given node — keep one panel in colour while the world goes mono.
func (m monochromeEffect) Dodge(ref *NodeRef) monochromeEffect { m.dodge = ref; return m }

func (m monochromeEffect) Apply(buf *Buffer, ctx PostContext) {
	EachCell(func(x, y int, c Cell, ectx PostContext) Cell {
		if m.dodge != nil && inRect(x, y, m.dodge) {
			return c
		}
		c.Style.FG = monochromeColor(resolveFG(c.Style.FG, ectx), m.tint)
		c.Style.BG = monochromeColor(resolveBG(c.Style.BG, ectx), m.tint)
		return c
	}).Apply(buf, ctx)
}

// ---------------------------------------------------------------------------
// Transitions & kinetic effects (require animation system for best results)
// ---------------------------------------------------------------------------

// dissolveEffect randomly hides cells based on progress.
type dissolveEffect struct{ progress *float64 }

func SEDissolve(progress *float64) dissolveEffect { return dissolveEffect{progress: progress} }

func (d dissolveEffect) Apply(buf *Buffer, ctx PostContext) {
	p := *d.progress
	if p <= 0 {
		return
	}
	empty := EmptyCell()
	for y := range ctx.Height {
		base := y * buf.width
		for x := range ctx.Width {
			cellHash := uint64(y*ctx.Width+x) * 2654435761
			threshold := float64(cellHash%1000) / 1000.0
			if threshold < p {
				buf.cells[base+x] = empty
			}
		}
	}
}

// screenShakeEffect displaces the entire buffer horizontally with a sine wave.
type screenShakeEffect struct{ amplitude float64 }

func SEScreenShake(amplitude float64) screenShakeEffect {
	return screenShakeEffect{amplitude: amplitude}
}

func (s screenShakeEffect) Apply(buf *Buffer, ctx PostContext) {
	offset := int(math.Round(math.Sin(float64(ctx.Frame)*1.5) * s.amplitude))
	if offset == 0 {
		return
	}

	empty := EmptyCell()
	for y := range ctx.Height {
		base := y * buf.width
		if offset > 0 {
			for x := ctx.Width - 1; x >= 0; x-- {
				if srcX := x - offset; srcX >= 0 {
					buf.cells[base+x] = buf.cells[base+srcX]
				} else {
					buf.cells[base+x] = empty
				}
			}
		} else {
			for x := range ctx.Width {
				if srcX := x - offset; srcX < ctx.Width {
					buf.cells[base+x] = buf.cells[base+srcX]
				} else {
					buf.cells[base+x] = empty
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Quantize (output optimisation, works standalone or via WithQuantize)
// ---------------------------------------------------------------------------

// quantizeEffect snaps all RGB colour channels to the nearest multiple of step.
type quantizeEffect struct{ step uint8 }

// SEQuantize snaps all RGB channels to the nearest multiple of step.
// Use step=32 to cut animation bytes per frame by ~40% with negligible banding.
// Prefer WithQuantize to wrap another effect rather than using this standalone.
func SEQuantize(step uint8) quantizeEffect { return quantizeEffect{step: step} }

func (q quantizeEffect) Apply(buf *Buffer, ctx PostContext) {
	EachCell(func(_, _ int, c Cell, _ PostContext) Cell {
		if c.Style.FG.Mode == ColorRGB {
			c.Style.FG.R = quantizeUint8(c.Style.FG.R, q.step)
			c.Style.FG.G = quantizeUint8(c.Style.FG.G, q.step)
			c.Style.FG.B = quantizeUint8(c.Style.FG.B, q.step)
		}
		if c.Style.BG.Mode == ColorRGB {
			c.Style.BG.R = quantizeUint8(c.Style.BG.R, q.step)
			c.Style.BG.G = quantizeUint8(c.Style.BG.G, q.step)
			c.Style.BG.B = quantizeUint8(c.Style.BG.B, q.step)
		}
		return c
	}).Apply(buf, ctx)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func resolveFG(c Color, ctx PostContext) Color {
	if c.Mode == ColorDefault && ctx.DefaultFG.Mode != ColorDefault {
		return ctx.DefaultFG
	}
	return c
}

func resolveBG(c Color, ctx PostContext) Color {
	if c.Mode == ColorDefault && ctx.DefaultBG.Mode != ColorDefault {
		return ctx.DefaultBG
	}
	return c
}

func lerpIfRGB(c, target Color, t float64) Color {
	if c.Mode == ColorDefault {
		return c
	}
	return LerpColor(c, target, t)
}

func desaturateColor(c Color, amount float64) Color {
	if c.Mode == ColorDefault {
		return c
	}
	gray := uint8(0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B))
	return LerpColor(c, RGB(gray, gray, gray), amount)
}

func boostContrast(c Color, amount float64) Color {
	if c.Mode == ColorDefault {
		return c
	}
	return RGB(
		contrastChannel(c.R, amount),
		contrastChannel(c.G, amount),
		contrastChannel(c.B, amount),
	)
}

func contrastChannel(v uint8, amount float64) uint8 {
	f := (float64(v)/255.0-0.5)*(1.0+amount) + 0.5
	if f < 0 {
		f = 0
	} else if f > 1 {
		f = 1
	}
	return uint8(f * 255)
}

func monochromeColor(c, tint Color) Color {
	if c.Mode == ColorDefault {
		return c
	}
	lum := 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
	return RGB(
		uint8(lum*float64(tint.R)/255),
		uint8(lum*float64(tint.G)/255),
		uint8(lum*float64(tint.B)/255),
	)
}

func gradientMap(c, dark, mid, bright Color) Color {
	if c.Mode == ColorDefault {
		return c
	}
	lum := (0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)) / 255.0
	if lum < 0.5 {
		return LerpColor(dark, mid, lum*2)
	}
	return LerpColor(mid, bright, (lum-0.5)*2)
}

func quantizeUint8(v, step uint8) uint8 {
	if step <= 1 {
		return v
	}
	rounded := int(math.Round(float64(v)/float64(step))) * int(step)
	if rounded > 255 {
		return 255
	}
	return uint8(rounded)
}

func inRect(x, y int, ref *NodeRef) bool {
	return x >= ref.X && x < ref.X+ref.W && y >= ref.Y && y < ref.Y+ref.H
}
