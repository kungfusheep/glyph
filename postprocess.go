package glyph

import (
	"math"
	"sync"
	"time"
)

// resolveColor16 updates Color16 cells in the buffer to use the detected
// terminal palette RGB values. Called once before effects run so all
// colour math operates on the terminal's actual colours.
func resolveColor16(buf *Buffer, w, h int) {
	for y := range h {
		base := y * buf.width
		for x := range w {
			c := &buf.cells[base+x]
			if c.Style.FG.Mode == Color16 {
				rgb := basic16RGB[c.Style.FG.Index&0xF]
				c.Style.FG.R, c.Style.FG.G, c.Style.FG.B = rgb[0], rgb[1], rgb[2]
			}
			if c.Style.BG.Mode == Color16 {
				rgb := basic16RGB[c.Style.BG.Index&0xF]
				c.Style.BG.R, c.Style.BG.G, c.Style.BG.B = rgb[0], rgb[1], rgb[2]
			}
		}
	}
}

// Effect transforms the buffer after rendering, before flush.
// Like a GPU shader pass. Receives the full cell buffer and frame context.
// Mutate cells in-place. Chain multiple passes for layered effects.
type Effect interface {
	Apply(buf *Buffer, ctx PostContext)
}

// EffectCompiler wires template-bound values for custom effects. It lets an
// external effect accept the same dynamic values as built-in properties:
// pointers, If branches, Animate, and In(...).Out(...).
type EffectCompiler interface {
	Float64(v any) EffectFloat64
}

// EffectFloat64 is a late-read float value compiled from a template argument.
type EffectFloat64 struct {
	val   float64
	ptr   *float64
	armed *bool
}

// StaticEffectFloat64 creates a fixed effect float value.
func StaticEffectFloat64(v float64) EffectFloat64 {
	return EffectFloat64{val: v}
}

// Float64 returns the current value for this frame.
func (v EffectFloat64) Float64() float64 {
	if v.armed != nil {
		*v.armed = true
	}
	if v.ptr != nil {
		return *v.ptr
	}
	return v.val
}

// EffectCompilable is implemented by effects with template-bound dynamic
// properties. The template compiler calls it once during Build.
type EffectCompilable interface {
	CompileEffect(c EffectCompiler) Effect
}

type effectCompiler struct {
	t *Template
}

func (c effectCompiler) Float64(v any) EffectFloat64 {
	switch val := v.(type) {
	case EffectFloat64:
		return val
	case float64:
		return StaticEffectFloat64(val)
	case float32:
		return StaticEffectFloat64(float64(val))
	case int:
		return StaticEffectFloat64(float64(val))
	case *float64:
		return EffectFloat64{ptr: val}
	case conditionNode, valueBranchNode:
		return EffectFloat64{ptr: c.t.compileDynFloat64(v, nil, 0)}
	case tweenNode:
		if tw, ok := v.(tweenNode); ok && (tw.getTweenFrom() != nil || tw.getTweenOut() != nil) && c.t.root != nil {
			armed := new(bool)
			return EffectFloat64{ptr: c.t.compileTweenFloat64(tw, armed, nil, 0), armed: armed}
		}
		return EffectFloat64{ptr: c.t.compileDynFloat64(v, nil, 0)}
	default:
		return EffectFloat64{}
	}
}

// effectCompilable is the legacy private hook used by built-in effects.
type effectCompilable interface {
	compileEffect(t *Template) Effect
}

// funcEffect adapts a bare function to the Effect interface.
// Returned by EachCell and used internally by blend/quantize wrappers.
type funcEffect func(*Buffer, PostContext)

func (f funcEffect) Apply(buf *Buffer, ctx PostContext) { f(buf, ctx) }

// screenEffectNode is a declarative node that registers one or more full-screen
// post-processing effects. Place it anywhere in the view tree. It takes zero
// layout space and applies to the entire screen. Works with If() for reactive
// toggling. Accepts multiple effects in a single node.
type screenEffectNode struct {
	Effects []Effect
}

// ScreenEffect creates a declarative full-screen post-processing node.
// Accepts multiple effects — they apply in order, left to right.
func ScreenEffect(pp ...Effect) Component {
	return screenEffectNode{Effects: pp}
}

// PostContext provides frame metadata to post-processing passes.
type PostContext struct {
	Width  int
	Height int
	Frame  uint64
	Delta  time.Duration // time since last frame
	Time   time.Duration // total elapsed since first render

	// terminal's default FG/BG detected via OSC 10/11 at startup.
	// Mode == ColorRGB when detected, ColorDefault when unknown.
	DefaultFG Color
	DefaultBG Color

	animReq *bool // set via RequestAnimation; framework schedules another frame
}

// RequestAnimation tells the framework this effect is mid-animation and
// needs another frame. Coalesced: one extra frame per render regardless of
// how many effects request it; an effect animates by requesting each frame
// until it settles (ADR 1).
func (c PostContext) RequestAnimation() {
	if c.animReq != nil {
		*c.animReq = true
	}
}

// EachCell wraps a per-cell transform into a Effect.
// The fragment shader equivalent. You define the per-cell logic,
// iteration is handled for you. Splits work across four quadrants
// using a persistent worker pool. Zero allocations in the hot path.
func EachCell(fn func(x, y int, cell Cell, ctx PostContext) Cell) Effect {
	return funcEffect(func(buf *Buffer, ctx PostContext) {
		qp := getCellPool()
		midX, midY := ctx.Width/2, ctx.Height/2

		// set shared work (safe: written before wakeup send, read after recv)
		qp.fn = fn
		qp.buf = buf
		qp.ctx = ctx
		qp.quads = [4][4]int{
			{0, midX, 0, midY},
			{midX, ctx.Width, 0, midY},
			{0, midX, midY, ctx.Height},
			{midX, ctx.Width, midY, ctx.Height},
		}

		for i := range 3 {
			qp.wakeup[i] <- struct{}{}
		}

		// run 4th quadrant on caller
		q := qp.quads[3]
		for y := q[2]; y < q[3]; y++ {
			base := y * buf.width
			for x := q[0]; x < q[1]; x++ {
				idx := base + x
				buf.cells[idx] = fn(x, y, buf.cells[idx], ctx)
			}
		}

		for i := range 3 {
			<-qp.done[i]
		}
	})
}

type cellPool struct {
	fn     func(x, y int, cell Cell, ctx PostContext) Cell
	buf    *Buffer
	ctx    PostContext
	quads  [4][4]int
	wakeup [3]chan struct{}
	done   [3]chan struct{}
}

var cellPoolInstance struct {
	once sync.Once
	pool *cellPool
}

func getCellPool() *cellPool {
	cellPoolInstance.once.Do(func() {
		p := &cellPool{}
		for i := range 3 {
			p.wakeup[i] = make(chan struct{}, 1)
			p.done[i] = make(chan struct{}, 1)
			go func(idx int) {
				for range p.wakeup[idx] {
					q := p.quads[idx]
					buf, fn, ctx := p.buf, p.fn, p.ctx
					for y := q[2]; y < q[3]; y++ {
						base := y * buf.width
						for x := q[0]; x < q[1]; x++ {
							i := base + x
							buf.cells[i] = fn(x, y, buf.cells[i], ctx)
						}
					}
					p.done[idx] <- struct{}{}
				}
			}(i)
		}
		cellPoolInstance.pool = p
	})
	return cellPoolInstance.pool
}

// ---------------------------------------------------------------------------
// Blend modes
// ---------------------------------------------------------------------------

// BlendMode controls how two colours are combined during post-processing.
// Use with WithBlend to wrap any Effect effect.
type BlendMode int

const (
	BlendNormal    BlendMode = iota
	BlendMultiply            // darkens: a * b / 255
	BlendScreen              // lightens: 255 - (255-a)(255-b)/255
	BlendOverlay             // multiply if dark, screen if light
	BlendAdd                 // clipped addition
	BlendSoftLight           // gentle contrast
	BlendDodge               // dramatic brighten
	BlendBurn                // dramatic darken
)

// Blend combines two RGB colours using the specified blend mode.
func Blend(base, top Color, mode BlendMode) Color {
	if base.Mode == ColorDefault || top.Mode == ColorDefault {
		return top
	}
	return RGB(
		blendChannel(base.R, top.R, mode),
		blendChannel(base.G, top.G, mode),
		blendChannel(base.B, top.B, mode),
	)
}

func blendChannel(a, b uint8, mode BlendMode) uint8 {
	fa, fb := float64(a)/255, float64(b)/255
	var r float64
	switch mode {
	case BlendMultiply:
		r = fa * fb
	case BlendScreen:
		r = 1 - (1-fa)*(1-fb)
	case BlendOverlay:
		if fa < 0.5 {
			r = 2 * fa * fb
		} else {
			r = 1 - 2*(1-fa)*(1-fb)
		}
	case BlendAdd:
		r = fa + fb
	case BlendSoftLight:
		if fb < 0.5 {
			r = fa - (1-2*fb)*fa*(1-fa)
		} else {
			r = fa + (2*fb-1)*(softLightG(fa)-fa)
		}
	case BlendDodge:
		if fb >= 1 {
			r = 1
		} else {
			r = fa / (1 - fb)
		}
	case BlendBurn:
		if fb <= 0 {
			r = 0
		} else {
			r = 1 - (1-fa)/fb
		}
	default:
		r = fb
	}
	if r < 0 {
		r = 0
	} else if r > 1 {
		r = 1
	}
	return uint8(r * 255)
}

func softLightG(a float64) float64 {
	if a <= 0.25 {
		return ((16*a-12)*a + 4) * a
	}
	return math.Sqrt(a)
}

// blendEffect wraps an inner Effect with a blend mode. Snapshots the
// buffer before the effect runs, then blends the output with the original.
type blendEffect struct {
	mode  BlendMode
	inner Effect
}

func (b blendEffect) Apply(buf *Buffer, ctx PostContext) {
	w, h := ctx.Width, ctx.Height

	// snapshot original FG+BG
	type colorPair struct{ fg, bg Color }
	snap := make([]colorPair, w*h)
	for y := range h {
		bufBase := y * buf.width
		snapBase := y * w
		for x := range w {
			c := buf.cells[bufBase+x]
			snap[snapBase+x] = colorPair{c.Style.FG, c.Style.BG}
		}
	}

	// run the effect
	b.inner.Apply(buf, ctx)

	// blend result with snapshot
	for y := range h {
		bufBase := y * buf.width
		snapBase := y * w
		for x := range w {
			c := &buf.cells[bufBase+x]
			orig := snap[snapBase+x]
			c.Style.FG = Blend(orig.fg, c.Style.FG, b.mode)
			c.Style.BG = Blend(orig.bg, c.Style.BG, b.mode)
		}
	}
}

// WithBlend wraps any Effect with a blend mode. Snapshots the buffer
// before the effect runs, then blends the effect's output with the original
// using the specified mode. Works with any effect, plasma through multiply,
// fire through screen, etc.
func WithBlend(mode BlendMode, pp Effect) Effect {
	return blendEffect{mode: mode, inner: pp}
}

// WithQuantize wraps a Effect with output quantization.
// Runs pp first, then snaps all resulting RGB values to the nearest multiple of step.
// Use step=32 to reduce bytes/frame by ~40-50% with acceptable banding at typical terminal sizes.
func WithQuantize(step uint8, pp Effect) Effect {
	q := SEQuantize(step)
	return funcEffect(func(buf *Buffer, ctx PostContext) {
		pp.Apply(buf, ctx)
		q.Apply(buf, ctx)
	})
}
