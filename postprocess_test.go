package glyph

import (
	"math"
	"testing"
	"time"
)

func TestEachCell(t *testing.T) {
	buf := NewBuffer(4, 3)
	buf.Set(0, 0, Cell{Rune: 'A', Style: Style{FG: RGB(255, 0, 0)}})
	buf.Set(1, 0, Cell{Rune: 'B', Style: Style{FG: RGB(0, 255, 0)}})

	pass := EachCell(func(x, y int, c Cell, _ PostContext) Cell {
		c.Style.FG = RGB(0, 0, 255)
		return c
	})

	ctx := PostContext{Width: 4, Height: 3}
	pass.Apply(buf, ctx)

	got := buf.Get(0, 0)
	if got.Style.FG.R != 0 || got.Style.FG.B != 255 {
		t.Errorf("EachCell: expected blue FG, got R=%d G=%d B=%d", got.Style.FG.R, got.Style.FG.G, got.Style.FG.B)
	}
	if got.Rune != 'A' {
		t.Errorf("EachCell: expected rune 'A', got %c", got.Rune)
	}
}

func TestPipelineOrder(t *testing.T) {
	buf := NewBuffer(2, 1)
	buf.Set(0, 0, Cell{Rune: 'X', Style: Style{FG: RGB(100, 100, 100)}})

	ctx := PostContext{Width: 2, Height: 1}

	pass1 := EachCell(func(_, _ int, c Cell, _ PostContext) Cell {
		c.Style.FG = RGB(255, 0, 0)
		return c
	})
	pass2 := EachCell(func(_, _ int, c Cell, _ PostContext) Cell {
		if c.Style.FG.Mode == ColorRGB {
			c.Style.FG.R /= 2
		}
		return c
	})

	pass1.Apply(buf, ctx)
	pass2.Apply(buf, ctx)

	got := buf.Get(0, 0)
	if got.Style.FG.R != 127 {
		t.Errorf("pipeline order: expected R=127, got R=%d", got.Style.FG.R)
	}
}

func TestSEDimAll(t *testing.T) {
	buf := NewBuffer(3, 2)
	buf.Set(1, 0, Cell{Rune: 'A', Style: Style{FG: RGB(255, 255, 255)}})

	pass := SEDimAll()
	pass.Apply(buf, PostContext{Width: 3, Height: 2})

	got := buf.Get(1, 0)
	if !got.Style.Attr.Has(AttrDim) {
		t.Error("SEDimAll: expected AttrDim to be set")
	}
	if got.Rune != 'A' {
		t.Errorf("SEDimAll: expected rune 'A', got %c", got.Rune)
	}
}

func TestSETint(t *testing.T) {
	buf := NewBuffer(1, 1)
	buf.Set(0, 0, Cell{Rune: 'X', Style: Style{FG: RGB(200, 200, 200)}})

	pass := SETint(RGB(255, 0, 0)).Strength(1.0)
	pass.Apply(buf, PostContext{Width: 1, Height: 1})

	got := buf.Get(0, 0)
	if got.Style.FG.R != 255 || got.Style.FG.G != 0 || got.Style.FG.B != 0 {
		t.Errorf("SETint: expected (255,0,0), got (%d,%d,%d)", got.Style.FG.R, got.Style.FG.G, got.Style.FG.B)
	}
}

func TestSETintPartial(t *testing.T) {
	buf := NewBuffer(1, 1)
	buf.Set(0, 0, Cell{Rune: 'X', Style: Style{FG: RGB(100, 100, 100)}})

	pass := SETint(RGB(200, 200, 200)).Strength(0.5)
	pass.Apply(buf, PostContext{Width: 1, Height: 1})

	got := buf.Get(0, 0)
	if got.Style.FG.R != 150 {
		t.Errorf("SETint partial: expected R=150, got R=%d", got.Style.FG.R)
	}
}

func TestSETintProcessesAllModes(t *testing.T) {
	// BasicColor(2) = green (R:0, G:170, B:0), tint fully toward blue
	buf := NewBuffer(1, 1)
	buf.Set(0, 0, Cell{Rune: 'X', Style: Style{FG: BasicColor(2)}})

	pass := SETint(RGB(0, 0, 255)).Strength(1.0)
	pass.Apply(buf, PostContext{Width: 1, Height: 1})

	got := buf.Get(0, 0)
	if got.Style.FG.B != 255 {
		t.Errorf("SETint: Color16 should be tinted, got (%d,%d,%d)",
			got.Style.FG.R, got.Style.FG.G, got.Style.FG.B)
	}

	// ColorDefault should still be skipped
	buf2 := NewBuffer(1, 1)
	buf2.Set(0, 0, Cell{Rune: 'X'})
	pass.Apply(buf2, PostContext{Width: 1, Height: 1})
	got2 := buf2.Get(0, 0)
	if got2.Style.FG.Mode != ColorDefault {
		t.Error("SETint: should skip ColorDefault")
	}
}

func TestSEDesaturate(t *testing.T) {
	buf := NewBuffer(1, 1)
	buf.Set(0, 0, Cell{Rune: 'X', Style: Style{FG: RGB(255, 0, 0)}})

	pass := SEDesaturate().Strength(1.0)
	pass.Apply(buf, PostContext{Width: 1, Height: 1})

	got := buf.Get(0, 0)
	if got.Style.FG.R != got.Style.FG.G || got.Style.FG.G != got.Style.FG.B {
		t.Errorf("SEDesaturate: expected equal RGB (gray), got (%d,%d,%d)",
			got.Style.FG.R, got.Style.FG.G, got.Style.FG.B)
	}
}

func TestSEFocusDim(t *testing.T) {
	buf := NewBuffer(10, 5)
	for y := range 5 {
		for x := range 10 {
			buf.Set(x, y, Cell{Rune: 'X', Style: DefaultStyle()})
		}
	}

	ref := NodeRef{X: 2, Y: 1, W: 3, H: 2}
	pass := SEFocusDim(&ref)
	pass.Apply(buf, PostContext{Width: 10, Height: 5})

	if buf.Get(2, 1).Style.Attr.Has(AttrDim) {
		t.Error("SEFocusDim: cell inside focus should not be dimmed")
	}
	if buf.Get(4, 2).Style.Attr.Has(AttrDim) {
		t.Error("SEFocusDim: cell inside focus should not be dimmed")
	}
	if !buf.Get(0, 0).Style.Attr.Has(AttrDim) {
		t.Error("SEFocusDim: cell outside focus should be dimmed")
	}
	if !buf.Get(5, 3).Style.Attr.Has(AttrDim) {
		t.Error("SEFocusDim: cell outside focus should be dimmed")
	}
}

func TestSEDissolve(t *testing.T) {
	buf := NewBuffer(20, 10)
	for y := range 10 {
		for x := range 20 {
			buf.Set(x, y, Cell{Rune: 'X', Style: Style{FG: RGB(255, 255, 255)}})
		}
	}

	progress := 0.5
	pass := SEDissolve(&progress)
	pass.Apply(buf, PostContext{Width: 20, Height: 10})

	dissolved := 0
	total := 20 * 10
	for y := range 10 {
		for x := range 20 {
			if buf.Get(x, y).Rune == ' ' {
				dissolved++
			}
		}
	}

	ratio := float64(dissolved) / float64(total)
	if ratio < 0.3 || ratio > 0.7 {
		t.Errorf("SEDissolve: expected ~50%% dissolved, got %.1f%% (%d/%d)", ratio*100, dissolved, total)
	}
}

func TestSEVignette(t *testing.T) {
	buf := NewBuffer(20, 10)
	center := RGB(200, 200, 200)
	for y := range 10 {
		for x := range 20 {
			buf.Set(x, y, Cell{Rune: 'X', Style: Style{FG: center}})
		}
	}

	pass := SEVignette().Strength(1.0)
	pass.Apply(buf, PostContext{Width: 20, Height: 10})

	centerCell := buf.Get(10, 5)
	edgeCell := buf.Get(0, 0)

	centerLum := int(centerCell.Style.FG.R) + int(centerCell.Style.FG.G) + int(centerCell.Style.FG.B)
	edgeLum := int(edgeCell.Style.FG.R) + int(edgeCell.Style.FG.G) + int(edgeCell.Style.FG.B)

	if centerLum <= edgeLum {
		t.Errorf("SEVignette: center (%d) should be brighter than edge (%d)", centerLum, edgeLum)
	}
}

func TestSEVignetteFocus(t *testing.T) {
	buf := NewBuffer(40, 20)
	grey := RGB(200, 200, 200)
	for y := range 20 {
		for x := range 40 {
			buf.Set(x, y, Cell{Rune: 'X', Style: Style{FG: grey}})
		}
	}

	// ref centred at (30,15) — far from screen centre (20,10)
	ref := NodeRef{X: 25, Y: 12, W: 10, H: 6}
	SEVignette().Strength(1.0).Focus(&ref).Apply(buf, PostContext{Width: 40, Height: 20})

	// cell near ref centre should be brighter than top-left corner
	nearRef := buf.Get(30, 15)
	corner := buf.Get(0, 0)
	nearLum := int(nearRef.Style.FG.R) + int(nearRef.Style.FG.G) + int(nearRef.Style.FG.B)
	cornerLum := int(corner.Style.FG.R) + int(corner.Style.FG.G) + int(corner.Style.FG.B)
	if nearLum <= cornerLum {
		t.Errorf("SEVignette Focus: cell near ref (%d) should be brighter than corner (%d)", nearLum, cornerLum)
	}
}

func TestSEVignetteDodge(t *testing.T) {
	buf := NewBuffer(40, 20)
	grey := RGB(200, 200, 200)
	for y := range 20 {
		for x := range 40 {
			buf.Set(x, y, Cell{Rune: 'X', Style: Style{FG: grey}})
		}
	}

	ref := NodeRef{X: 25, Y: 12, W: 10, H: 6}
	SEVignette().Strength(1.0).Dodge(&ref).Apply(buf, PostContext{Width: 40, Height: 20})

	// cells inside the dodge ref must be completely unaffected
	expected := int(grey.R) + int(grey.G) + int(grey.B)
	for y := ref.Y; y < ref.Y+ref.H; y++ {
		for x := ref.X; x < ref.X+ref.W; x++ {
			c := buf.Get(x, y)
			lum := int(c.Style.FG.R) + int(c.Style.FG.G) + int(c.Style.FG.B)
			if lum != expected {
				t.Errorf("SEVignette Dodge: cell inside ref (%d,%d) should be unaffected, got lum %d want %d", x, y, lum, expected)
			}
		}
	}
}

func TestSEVignetteDodgeTracksRefOpacity(t *testing.T) {
	grey := RGB(200, 200, 200)
	render := func(opacity float64) int {
		buf := NewBuffer(40, 20)
		for y := range 20 {
			for x := range 40 {
				buf.Set(x, y, Cell{Rune: 'X', Style: Style{FG: grey}})
			}
		}
		ref := NodeRef{X: 25, Y: 12, W: 10, H: 6, Opacity: opacity, opacitySet: true}
		SEVignette().Strength(1.0).Dodge(&ref).Smooth().Apply(buf, PostContext{Width: 40, Height: 20})
		c := buf.Get(ref.X, ref.Y)
		return int(c.Style.FG.R) + int(c.Style.FG.G) + int(c.Style.FG.B)
	}

	opaque := render(1)
	half := render(0.5)
	quarter := render(0.25)
	hidden := render(0)
	expected := int(grey.R) + int(grey.G) + int(grey.B)

	if opaque != expected {
		t.Fatalf("opaque dodge should leave ref untouched, got %d want %d", opaque, expected)
	}
	if half >= opaque || half <= hidden {
		t.Fatalf("half-opacity dodge should sit between opaque and hidden, got hidden=%d half=%d opaque=%d", hidden, half, opaque)
	}
	if quarter >= half || quarter < hidden {
		t.Fatalf("dodge should collapse as ref fades, got hidden=%d quarter=%d half=%d", hidden, quarter, half)
	}
}

func TestSEVignetteDodgeFeathersOutsideRef(t *testing.T) {
	grey := RGB(200, 200, 200)
	render := func() *Buffer {
		buf := NewBuffer(40, 20)
		for y := range 20 {
			for x := range 40 {
				buf.Set(x, y, Cell{Rune: 'X', Style: Style{FG: grey}})
			}
		}
		ref := NodeRef{X: 16, Y: 8, W: 8, H: 4, Opacity: 1, opacitySet: true}
		SEVignette().Strength(1.0).Dodge(&ref).Smooth().Apply(buf, PostContext{Width: 40, Height: 20})
		return buf
	}
	lum := func(c Color) int {
		return int(c.R) + int(c.G) + int(c.B)
	}

	buf := render()
	inside := lum(buf.Get(16, 9).Style.FG)
	near := lum(buf.Get(24, 9).Style.FG)
	far := lum(buf.Get(29, 9).Style.FG)

	expected := lum(grey)
	if inside != expected {
		t.Fatalf("inside dodge should remain unaffected, got %d want %d", inside, expected)
	}
	if near <= far {
		t.Fatalf("near outside dodge should be feathered brighter than far outside, near=%d far=%d", near, far)
	}
	if near >= inside {
		t.Fatalf("outside feather should not fully dodge like inside, near=%d inside=%d", near, inside)
	}
}

func TestSEVignetteFocusCorner(t *testing.T) {
	// focus node near a corner — previously produced a tiny maxDist and near-black screen
	buf := NewBuffer(80, 24)
	grey := RGB(200, 200, 200)
	for y := range 24 {
		for x := range 80 {
			buf.Set(x, y, Cell{Rune: 'X', Style: Style{FG: grey}})
		}
	}

	ref := NodeRef{X: 2, Y: 2, W: 10, H: 5}
	SEVignette().Strength(0.9).Focus(&ref).Apply(buf, PostContext{Width: 80, Height: 24})

	// the screen center should still have meaningful brightness, not be black
	center := buf.Get(40, 12)
	centerLum := int(center.Style.FG.R) + int(center.Style.FG.G) + int(center.Style.FG.B)
	if centerLum < 300 {
		t.Errorf("SEVignette corner Focus: screen center should not be black, got lum %d", centerLum)
	}
}

func TestQuantizeUint8Overflow(t *testing.T) {
	// step=32: 255 rounds up to 256, which previously wrapped to 0
	if got := quantizeUint8(255, 32); got != 255 {
		t.Errorf("quantizeUint8(255, 32) = %d, want 255", got)
	}
	if got := quantizeUint8(241, 32); got != 255 {
		t.Errorf("quantizeUint8(241, 32) = %d, want 255", got)
	}
	// step=16: 255 rounds to 256 → should clamp
	if got := quantizeUint8(255, 16); got != 255 {
		t.Errorf("quantizeUint8(255, 16) = %d, want 255", got)
	}
	// normal case: 128 with step 32 → 128
	if got := quantizeUint8(128, 32); got != 128 {
		t.Errorf("quantizeUint8(128, 32) = %d, want 128", got)
	}
}

func TestSEDropShadow(t *testing.T) {
	buf := NewBuffer(40, 20)
	grey := RGB(200, 200, 200)
	for y := range 20 {
		for x := range 40 {
			buf.Set(x, y, Cell{Rune: 'X', Style: Style{FG: grey}})
		}
	}

	ref := NodeRef{X: 10, Y: 5, W: 10, H: 5}
	SEDropShadow().Strength(1.0).Radius(6).Focus(&ref).Apply(buf, PostContext{Width: 40, Height: 20})

	// cells inside the focus ref must be completely unaffected
	expected := int(grey.R) + int(grey.G) + int(grey.B)
	for y := ref.Y; y < ref.Y+ref.H; y++ {
		for x := ref.X; x < ref.X+ref.W; x++ {
			c := buf.Get(x, y)
			lum := int(c.Style.FG.R) + int(c.Style.FG.G) + int(c.Style.FG.B)
			if lum != expected {
				t.Errorf("SEDropShadow: cell inside ref (%d,%d) should be unaffected, got lum %d want %d", x, y, lum, expected)
			}
		}
	}

	// cell just outside the ref should be darker than one far away
	near := buf.Get(ref.X+ref.W, ref.Y+ref.H/2) // immediately right of ref
	far := buf.Get(39, 10)
	nearLum := int(near.Style.FG.R) + int(near.Style.FG.G) + int(near.Style.FG.B)
	farLum := int(far.Style.FG.R) + int(far.Style.FG.G) + int(far.Style.FG.B)
	if nearLum >= farLum {
		t.Errorf("SEDropShadow: cell near ref (%d) should be darker than far cell (%d)", nearLum, farLum)
	}
}

func TestSEDropShadowTracksFocusOpacity(t *testing.T) {
	grey := RGB(200, 200, 200)
	render := func(opacity float64) Color {
		ref := NodeRef{X: 10, Y: 5, W: 10, H: 5, Opacity: opacity, opacitySet: true}
		buf := NewBuffer(40, 20)
		for y := range 20 {
			for x := range 40 {
				buf.Set(x, y, Cell{Rune: 'X', Style: Style{FG: grey}})
			}
		}

		SEDropShadow().Strength(1.0).Radius(6).Focus(&ref).Apply(buf, PostContext{Width: 40, Height: 20})
		return buf.Get(ref.X+ref.W, ref.Y+ref.H/2).Style.FG
	}

	opaque := render(1)
	half := render(0.5)
	hidden := render(0)

	if hidden != grey {
		t.Fatalf("faded focus should not cast shadow, got %#v want %#v", hidden, grey)
	}
	if half.R >= grey.R || half.R <= opaque.R {
		t.Fatalf("half-opacity shadow should sit between hidden and opaque, got hidden=%#v half=%#v opaque=%#v", hidden, half, opaque)
	}
}

func TestSEGlowTracksFocusOpacity(t *testing.T) {
	bg := RGB(20, 20, 20)
	ref := NodeRef{X: 2, Y: 1, W: 4, H: 2, Opacity: 0, opacitySet: true}
	buf := NewBuffer(12, 6)
	for y := range 6 {
		for x := range 12 {
			buf.Set(x, y, Cell{Rune: ' ', Style: Style{FG: bg, BG: bg}})
		}
	}
	for y := ref.Y; y < ref.Y+ref.H; y++ {
		for x := ref.X; x < ref.X+ref.W; x++ {
			buf.Set(x, y, Cell{Rune: ' ', Style: Style{FG: RGB(220, 220, 220), BG: RGB(120, 180, 220)}})
		}
	}

	SEGlow().Strength(1.0).Radius(5).Focus(&ref).Apply(buf, PostContext{Width: 12, Height: 6})

	near := buf.Get(ref.X+ref.W, ref.Y)
	if near.Style.BG != bg {
		t.Fatalf("faded focus should not glow, got %#v want %#v", near.Style.BG, bg)
	}
}

func TestSEPulse(t *testing.T) {
	buf := NewBuffer(2, 1)
	buf.Set(0, 0, Cell{Rune: 'X', Style: Style{FG: RGB(200, 200, 200)}})

	// at time=0, sin(0)=0, t=0.5, dim=0.5*0.5=0.25
	pass := SEPulse().Speed(1.0).Strength(0.5)
	pass.Apply(buf, PostContext{Width: 2, Height: 1, Time: 0})

	got := buf.Get(0, 0)
	// should be dimmed somewhat from 200
	if got.Style.FG.R >= 200 {
		t.Errorf("SEPulse: expected dimming at t=0, got R=%d", got.Style.FG.R)
	}
	if got.Style.FG.R == 0 {
		t.Error("SEPulse: should not be fully black")
	}
}

func TestSEScreenShake(t *testing.T) {
	buf := NewBuffer(10, 1)
	for x := range 10 {
		buf.Set(x, 0, Cell{Rune: rune('A' + x), Style: DefaultStyle()})
	}

	// frame chosen so sin(frame*1.5) gives a non-zero offset
	pass := SEScreenShake(3.0)
	pass.Apply(buf, PostContext{Width: 10, Height: 1, Frame: 1})

	// at least some cells should have shifted
	unchanged := 0
	for x := range 10 {
		if buf.Get(x, 0).Rune == rune('A'+x) {
			unchanged++
		}
	}
	if unchanged == 10 {
		t.Error("SEScreenShake: expected some cells to shift")
	}
}

func TestSEGradientMap(t *testing.T) {
	buf := NewBuffer(3, 1)
	buf.Set(0, 0, Cell{Rune: 'D', Style: Style{FG: RGB(30, 30, 30)}})    // dark
	buf.Set(1, 0, Cell{Rune: 'M', Style: Style{FG: RGB(128, 128, 128)}}) // mid
	buf.Set(2, 0, Cell{Rune: 'B', Style: Style{FG: RGB(230, 230, 230)}}) // bright

	pass := SEGradientMap(RGB(0, 0, 50), RGB(0, 128, 128), RGB(200, 255, 200))
	pass.Apply(buf, PostContext{Width: 3, Height: 1})

	dark := buf.Get(0, 0)
	mid := buf.Get(1, 0)
	bright := buf.Get(2, 0)

	// dark cell should be near the dark stop (blue-ish)
	if dark.Style.FG.B < dark.Style.FG.R {
		t.Errorf("SEGradientMap: dark cell should lean blue, got (%d,%d,%d)",
			dark.Style.FG.R, dark.Style.FG.G, dark.Style.FG.B)
	}
	// mid cell should be near teal
	if mid.Style.FG.G < 100 {
		t.Errorf("SEGradientMap: mid cell should have green, got G=%d", mid.Style.FG.G)
	}
	// bright cell should be near the bright stop (green-white)
	if bright.Style.FG.G < 200 {
		t.Errorf("SEGradientMap: bright cell should lean green-white, got G=%d", bright.Style.FG.G)
	}
}

func TestSEBloom(t *testing.T) {
	buf := NewBuffer(5, 1)
	// dark cells with one bright cell in the middle
	for x := range 5 {
		buf.Set(x, 0, Cell{Rune: 'X', Style: Style{FG: RGB(20, 20, 20)}})
	}
	buf.Set(2, 0, Cell{Rune: 'X', Style: Style{FG: RGB(255, 255, 255)}})

	pass := SEBloom().Threshold(0.5).Strength(0.8)
	pass.Apply(buf, PostContext{Width: 5, Height: 1})

	// neighbours of the bright cell should be brighter than they started
	left := buf.Get(1, 0)
	right := buf.Get(3, 0)
	if left.Style.FG.R <= 20 {
		t.Errorf("SEBloom: left neighbour should be brighter, got R=%d", left.Style.FG.R)
	}
	if right.Style.FG.R <= 20 {
		t.Errorf("SEBloom: right neighbour should be brighter, got R=%d", right.Style.FG.R)
	}
}

func TestSEBloomFocusTracksOpacity(t *testing.T) {
	buf := NewBuffer(5, 1)
	for x := range 5 {
		buf.Set(x, 0, Cell{Rune: 'X', Style: Style{FG: RGB(20, 20, 20)}})
	}
	buf.Set(2, 0, Cell{Rune: 'X', Style: Style{FG: RGB(255, 255, 255)}})

	ref := NodeRef{X: 0, Y: 0, W: 5, H: 1, Opacity: 0, opacitySet: true}
	SEBloom().Focus(&ref).Threshold(0.5).Strength(0.8).Apply(buf, PostContext{Width: 5, Height: 1})

	left := buf.Get(1, 0)
	if left.Style.FG.R != 20 {
		t.Errorf("faded focus should not bloom, got R=%d", left.Style.FG.R)
	}
}

func TestSEBloomSkipsDark(t *testing.T) {
	buf := NewBuffer(3, 1)
	for x := range 3 {
		buf.Set(x, 0, Cell{Rune: 'X', Style: Style{FG: RGB(30, 30, 30)}})
	}

	pass := SEBloom().Threshold(0.5).Strength(1.0)
	pass.Apply(buf, PostContext{Width: 3, Height: 1})

	// all cells dark, nothing should bloom
	got := buf.Get(1, 0)
	if got.Style.FG.R != 30 {
		t.Errorf("SEBloom: all-dark buffer should be unchanged, got R=%d", got.Style.FG.R)
	}
}
func TestSEMonochrome(t *testing.T) {
	buf := NewBuffer(1, 1)
	buf.Set(0, 0, Cell{Rune: 'X', Style: Style{FG: RGB(255, 0, 0)}})

	// green phosphor monochrome
	pass := SEMonochrome(RGB(0, 255, 0))
	pass.Apply(buf, PostContext{Width: 1, Height: 1})

	got := buf.Get(0, 0)
	// red input → luminance ~76 (0.299*255) → green output
	if got.Style.FG.R != 0 {
		t.Errorf("SEMonochrome: R should be 0 with green tint, got %d", got.Style.FG.R)
	}
	if got.Style.FG.G == 0 {
		t.Error("SEMonochrome: G should be non-zero with green tint")
	}
	if got.Style.FG.B != 0 {
		t.Errorf("SEMonochrome: B should be 0 with green tint, got %d", got.Style.FG.B)
	}
}

func TestSEMonochromeAmber(t *testing.T) {
	buf := NewBuffer(1, 1)
	buf.Set(0, 0, Cell{Rune: 'X', Style: Style{FG: RGB(200, 200, 200)}})

	pass := SEMonochrome(RGB(255, 180, 0))
	pass.Apply(buf, PostContext{Width: 1, Height: 1})

	got := buf.Get(0, 0)
	// should have R > G > B=0
	if got.Style.FG.R <= got.Style.FG.G {
		t.Errorf("SEMonochrome amber: expected R > G, got R=%d G=%d", got.Style.FG.R, got.Style.FG.G)
	}
	if got.Style.FG.B != 0 {
		t.Errorf("SEMonochrome amber: B should be 0, got %d", got.Style.FG.B)
	}
}

func TestBlendMultiply(t *testing.T) {
	a := RGB(200, 100, 50)
	b := RGB(128, 128, 128)
	result := BlendColor(a, b, BlendMultiply)
	// multiply darkens: 200*128/255 ≈ 100
	if result.R >= 200 {
		t.Errorf("BlendMultiply: expected darkened R, got %d", result.R)
	}
}

func TestBlendScreen(t *testing.T) {
	a := RGB(100, 100, 100)
	b := RGB(100, 100, 100)
	result := BlendColor(a, b, BlendScreen)
	// screen lightens
	if result.R <= 100 {
		t.Errorf("BlendScreen: expected brighter R, got %d", result.R)
	}
}

func TestBlendOverlay(t *testing.T) {
	// overlay with a bright top pushes dark bases darker, light bases lighter
	dark := BlendColor(RGB(50, 50, 50), RGB(200, 200, 200), BlendOverlay)
	light := BlendColor(RGB(200, 200, 200), RGB(200, 200, 200), BlendOverlay)
	if dark.R >= 100 {
		t.Errorf("BlendOverlay: dark base with bright top should stay low, got R=%d", dark.R)
	}
	if light.R <= 200 {
		t.Errorf("BlendOverlay: light base with bright top should get brighter, got R=%d", light.R)
	}
}

func TestBlendProcessesAllModes(t *testing.T) {
	// BasicColor(1) = red (170,0,0), top = (128,128,128)
	// multiply: 170*128/255 ≈ 85
	base := BasicColor(1)
	top := RGB(128, 128, 128)
	result := BlendColor(base, top, BlendMultiply)
	if result.R >= 128 {
		t.Errorf("BlendColor: Color16 should be blended, got R=%d", result.R)
	}

	// ColorDefault should still be skipped
	result2 := BlendColor(Color{}, RGB(255, 0, 0), BlendMultiply)
	if result2.R != 255 {
		t.Errorf("BlendColor: should return top for ColorDefault base, got R=%d", result2.R)
	}
}

func TestWithBlend(t *testing.T) {
	buf := NewBuffer(2, 1)
	buf.Set(0, 0, Cell{Rune: 'A', Style: Style{FG: RGB(200, 200, 200)}})

	// effect that sets FG to mid-grey
	effect := EachCell(func(_, _ int, c Cell, _ PostContext) Cell {
		c.Style.FG = RGB(128, 128, 128)
		return c
	})

	pass := WithBlend(BlendMultiply, effect)
	pass.Apply(buf, PostContext{Width: 2, Height: 1})

	got := buf.Get(0, 0)
	// multiply: 200*128/255 ≈ 100, should be darker than both inputs
	if got.Style.FG.R >= 128 {
		t.Errorf("WithBlend(Multiply): expected R < 128, got %d", got.Style.FG.R)
	}
}

func TestWithBlendTint(t *testing.T) {
	buf := NewBuffer(5, 3)
	for y := range 3 {
		for x := range 5 {
			buf.Set(x, y, Cell{Rune: 'X', Style: Style{FG: RGB(200, 200, 200)}})
		}
	}

	pass := WithBlend(BlendMultiply, SETint(RGB(128, 128, 128)).Strength(1.0))
	pass.Apply(buf, PostContext{Width: 5, Height: 3})

	// tint through multiply should be darker than original
	got := buf.Get(2, 1)
	lum := int(got.Style.FG.R) + int(got.Style.FG.G) + int(got.Style.FG.B)
	if lum >= 600 {
		t.Errorf("WithBlend(Multiply, Tint): expected darkened output, got lum=%d", lum)
	}
}

func TestScreenEffectInTree(t *testing.T) {
	called := false
	effect := funcEffect(func(buf *Buffer, ctx PostContext) { called = true })

	tmpl := Build(VBox(
		Text("hello"),
		ScreenEffect(effect),
	))

	buf := NewBuffer(20, 5)
	tmpl.Execute(buf, 20, 5)

	effects := tmpl.ScreenEffects()
	if len(effects) != 1 {
		t.Fatalf("expected 1 screen effect, got %d", len(effects))
	}

	// run the collected effect
	effects[0].Apply(buf, PostContext{Width: 20, Height: 5})
	if !called {
		t.Error("screen effect was not called")
	}
}

func TestScreenEffectWithIf(t *testing.T) {
	active := false
	effect := funcEffect(func(buf *Buffer, ctx PostContext) {})

	tmpl := Build(VBox(
		Text("hello"),
		If(&active).Then(ScreenEffect(effect)),
	))

	buf := NewBuffer(20, 5)

	// inactive, no effects collected
	tmpl.Execute(buf, 20, 5)
	if len(tmpl.ScreenEffects()) != 0 {
		t.Errorf("expected 0 effects when inactive, got %d", len(tmpl.ScreenEffects()))
	}

	// activate, effect collected
	active = true
	tmpl.Execute(buf, 20, 5)
	if len(tmpl.ScreenEffects()) != 1 {
		t.Errorf("expected 1 effect when active, got %d", len(tmpl.ScreenEffects()))
	}

	// deactivate, back to 0
	active = false
	tmpl.Execute(buf, 20, 5)
	if len(tmpl.ScreenEffects()) != 0 {
		t.Errorf("expected 0 effects when deactivated, got %d", len(tmpl.ScreenEffects()))
	}
}

func TestScreenEffectMultiple(t *testing.T) {
	order := make([]int, 0, 3)
	e1 := funcEffect(func(buf *Buffer, ctx PostContext) { order = append(order, 1) })
	e2 := funcEffect(func(buf *Buffer, ctx PostContext) { order = append(order, 2) })
	e3 := funcEffect(func(buf *Buffer, ctx PostContext) { order = append(order, 3) })

	tmpl := Build(VBox(
		ScreenEffect(e1),
		Text("content"),
		ScreenEffect(e2),
		ScreenEffect(e3),
	))

	buf := NewBuffer(20, 5)
	tmpl.Execute(buf, 20, 5)

	effects := tmpl.ScreenEffects()
	if len(effects) != 3 {
		t.Fatalf("expected 3 effects, got %d", len(effects))
	}

	for _, e := range effects {
		e.Apply(buf, PostContext{})
	}
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Errorf("effects should run in tree order, got %v", order)
	}
}

func TestScreenEffectZeroLayoutSpace(t *testing.T) {
	tmpl := Build(VBox(
		Text("line1"),
		ScreenEffect(funcEffect(func(*Buffer, PostContext) {})),
		Text("line2"),
	))

	buf := NewBuffer(20, 5)
	tmpl.Execute(buf, 20, 5)

	// line2 should be on row 1, not row 2. ScreenEffect takes no space
	c := buf.Get(0, 1)
	if c.Rune != 'l' {
		t.Errorf("ScreenEffect should take zero layout space, got rune %c at (0,1)", c.Rune)
	}
}

func BenchmarkScreenEffectCollection(b *testing.B) {
	effect := funcEffect(func(buf *Buffer, ctx PostContext) {})
	active := true
	tmpl := Build(VBox(
		Text("content"),
		If(&active).Then(ScreenEffect(effect)),
	))
	buf := NewBuffer(80, 24)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		tmpl.Execute(buf, 80, 24)
	}
}

func TestPostContextFields(t *testing.T) {
	ctx := PostContext{
		Width:  80,
		Height: 24,
		Frame:  100,
		Delta:  16 * time.Millisecond,
		Time:   5 * time.Second,
	}

	if ctx.Width != 80 || ctx.Height != 24 {
		t.Error("PostContext dimensions wrong")
	}
	if ctx.Frame != 100 {
		t.Error("PostContext frame wrong")
	}
	if ctx.Delta != 16*time.Millisecond {
		t.Error("PostContext delta wrong")
	}
}

func TestParseOSCColor(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		digit    byte
		wantR    uint8
		wantG    uint8
		wantB    uint8
		wantMode ColorMode
	}{
		{
			name:  "4-digit hex (xterm style)",
			data:  "\x1b]10;rgb:ffff/aaaa/0000\x1b\\",
			digit: '0',
			wantR: 255, wantG: 170, wantB: 0,
			wantMode: ColorRGB,
		},
		{
			name:  "2-digit hex",
			data:  "\x1b]10;rgb:ff/80/40\x1b\\",
			digit: '0',
			wantR: 255, wantG: 128, wantB: 64,
			wantMode: ColorRGB,
		},
		{
			name:  "OSC 11 (BG)",
			data:  "\x1b]11;rgb:1a1a/1a1a/2e2e\x1b\\",
			digit: '1',
			wantR: 26, wantG: 26, wantB: 46,
			wantMode: ColorRGB,
		},
		{
			name:  "both FG and BG in one response",
			data:  "\x1b]10;rgb:cccc/bbbb/aaaa\x1b\\\x1b]11;rgb:1111/2222/3333\x1b\\",
			digit: '1',
			wantR: 17, wantG: 34, wantB: 51,
			wantMode: ColorRGB,
		},
		{
			name:  "BEL terminator",
			data:  "\x1b]10;rgb:8080/4040/c0c0\x07",
			digit: '0',
			wantR: 128, wantG: 64, wantB: 192,
			wantMode: ColorRGB,
		},
		{
			name:     "no match returns default",
			data:     "garbage",
			digit:    '0',
			wantMode: ColorDefault,
		},
		{
			name:     "empty data",
			data:     "",
			digit:    '0',
			wantMode: ColorDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOSCColor([]byte(tt.data), tt.digit)
			if got.Mode != tt.wantMode {
				t.Errorf("Mode: got %d, want %d", got.Mode, tt.wantMode)
			}
			if tt.wantMode == ColorRGB {
				if got.R != tt.wantR || got.G != tt.wantG || got.B != tt.wantB {
					t.Errorf("RGB: got (%d,%d,%d), want (%d,%d,%d)",
						got.R, got.G, got.B, tt.wantR, tt.wantG, tt.wantB)
				}
			}
		})
	}
}

func TestParseOSC4Color(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		index    int
		wantR    uint8
		wantG    uint8
		wantB    uint8
		wantMode ColorMode
	}{
		{
			name:  "index 2 (green) 4-digit hex",
			data:  "\x1b]4;2;rgb:4e4e/d2d2/8e8e\x07",
			index: 2,
			wantR: 78, wantG: 210, wantB: 142,
			wantMode: ColorRGB,
		},
		{
			name:  "index 3 (yellow) 2-digit hex",
			data:  "\x1b]4;3;rgb:cc/88/00\x07",
			index: 3,
			wantR: 204, wantG: 136, wantB: 0,
			wantMode: ColorRGB,
		},
		{
			name:  "index 11 (bright yellow) double-digit index",
			data:  "\x1b]4;11;rgb:ffff/ffff/0000\x1b\\",
			index: 11,
			wantR: 255, wantG: 255, wantB: 0,
			wantMode: ColorRGB,
		},
		{
			name:  "multiple palette entries in one response",
			data:  "\x1b]4;0;rgb:0000/0000/0000\x07\x1b]4;1;rgb:cccc/0000/0000\x07\x1b]4;2;rgb:0000/cccc/0000\x07",
			index: 1,
			wantR: 204, wantG: 0, wantB: 0,
			wantMode: ColorRGB,
		},
		{
			name:     "no match for requested index",
			data:     "\x1b]4;5;rgb:aaaa/bbbb/cccc\x07",
			index:    3,
			wantMode: ColorDefault,
		},
		{
			name:     "empty data",
			data:     "",
			index:    0,
			wantMode: ColorDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOSC4Color([]byte(tt.data), tt.index)
			if got.Mode != tt.wantMode {
				t.Errorf("Mode: got %d, want %d", got.Mode, tt.wantMode)
			}
			if tt.wantMode == ColorRGB {
				if got.R != tt.wantR || got.G != tt.wantG || got.B != tt.wantB {
					t.Errorf("RGB: got (%d,%d,%d), want (%d,%d,%d)",
						got.R, got.G, got.B, tt.wantR, tt.wantG, tt.wantB)
				}
			}
		})
	}
}

func TestRefreshBasic16Vars(t *testing.T) {
	// save original
	origGreen := basic16RGB[2]
	defer func() {
		basic16RGB[2] = origGreen
		refreshBasic16Vars()
	}()

	// simulate OSC 4 detection changing green to a themed value
	basic16RGB[2] = [3]uint8{78, 210, 142}
	refreshBasic16Vars()

	if Green.R != 78 || Green.G != 210 || Green.B != 142 {
		t.Errorf("Green after refresh: got (%d,%d,%d), want (78,210,142)",
			Green.R, Green.G, Green.B)
	}
	if Green.Mode != Color16 || Green.Index != 2 {
		t.Errorf("Green should stay Color16 index 2, got mode=%d index=%d",
			Green.Mode, Green.Index)
	}
}

func TestResolveFG(t *testing.T) {
	detected := PostContext{
		DefaultFG: RGB(200, 150, 100),
		DefaultBG: RGB(10, 10, 30),
	}

	// ColorDefault with detected → uses detected
	c := resolveFG(Color{}, detected)
	if c.R != 200 || c.G != 150 || c.B != 100 {
		t.Errorf("resolveFG(default, detected): got (%d,%d,%d)", c.R, c.G, c.B)
	}

	// ColorDefault without detection → returns as-is
	c = resolveFG(Color{}, PostContext{})
	if c.Mode != ColorDefault {
		t.Error("resolveFG(default, undetected): should return ColorDefault")
	}

	// non-default → unchanged
	orig := RGB(255, 0, 0)
	c = resolveFG(orig, detected)
	if c.R != 255 || c.G != 0 || c.B != 0 {
		t.Errorf("resolveFG(explicit): should pass through, got (%d,%d,%d)", c.R, c.G, c.B)
	}
}

func TestSESpinGlow(t *testing.T) {
	newBuf := func() *Buffer {
		buf := NewBuffer(40, 20)
		grey := RGB(200, 200, 200)
		for y := range 20 {
			for x := range 40 {
				buf.Set(x, y, Cell{Rune: 'X', Style: Style{FG: grey}})
			}
		}
		return buf
	}
	ref := NodeRef{X: 15, Y: 8, W: 10, H: 4}

	// cells inside the focus ref must be completely unaffected.
	buf := newBuf()
	SESpinGlow(&ref, RGB(255, 0, 0)).Strength(1.0).Radius(8).Speed(0).
		Apply(buf, PostContext{Width: 40, Height: 20})
	for y := ref.Y; y < ref.Y+ref.H; y++ {
		for x := ref.X; x < ref.X+ref.W; x++ {
			c := buf.Get(x, y)
			if c.Style.FG.R != 200 || c.Style.FG.G != 200 || c.Style.FG.B != 200 {
				t.Errorf("SESpinGlow: cell inside ref (%d,%d) mutated, got (%d,%d,%d)", x, y, c.Style.FG.R, c.Style.FG.G, c.Style.FG.B)
			}
		}
	}

	// cells just outside should be shifted toward the tint colour,
	// cells far beyond radius should stay at the original grey.
	near := buf.Get(ref.X+ref.W, ref.Y+ref.H/2)
	if near.Style.FG.R <= 200 {
		t.Errorf("SESpinGlow: cell near ref should shift toward red, got R=%d", near.Style.FG.R)
	}
	far := buf.Get(0, 0)
	if far.Style.FG.R != 200 {
		t.Errorf("SESpinGlow: cell far beyond radius should be untouched, got R=%d", far.Style.FG.R)
	}

	// zero-arg palette must fall back to the default palette (no crash, effect applies).
	buf = newBuf()
	SESpinGlow(&ref).Strength(1.0).Radius(8).Speed(0).
		Apply(buf, PostContext{Width: 40, Height: 20})
	near = buf.Get(ref.X+ref.W, ref.Y+ref.H/2)
	if near.Style.FG.R == 200 && near.Style.FG.G == 200 && near.Style.FG.B == 200 {
		t.Error("SESpinGlow: default palette should still tint surrounding cells")
	}

	// rim side cells are full-cell geometry, so they paint through BG while
	// top/bottom keep half-block FG strokes. The side tint should still be full
	// strength; the old foreground-block path made opacity handoff visibly uneven.
	buf = NewBuffer(40, 20)
	rimBG := RGB(26, 26, 26)
	grey := RGB(200, 200, 200)
	for y := range 20 {
		for x := range 40 {
			buf.Set(x, y, Cell{Rune: 'X', Style: Style{FG: grey, BG: rimBG, Attr: AttrDim}})
		}
	}
	tint := RGB(255, 0, 0)
	SESpinGlow(&ref, tint).Strength(1.0).Radius(8).Speed(0).Rim(true).
		Apply(buf, PostContext{Width: 40, Height: 20, DefaultFG: grey, DefaultBG: rimBG})
	side := buf.Get(ref.X+ref.W, ref.Y+ref.H/2)
	top := buf.Get(ref.X+ref.W/2, ref.Y-1)
	if side.Rune != ' ' {
		t.Fatalf("SESpinGlow rim side rune = %q, want space-backed BG cell", side.Rune)
	}
	if side.Style.BG != tint {
		t.Errorf("SESpinGlow rim side BG = %+v, want full tint %+v", side.Style.BG, tint)
	}
	if top.Style.FG != tint {
		t.Errorf("SESpinGlow rim top FG = %+v, want full tint %+v", top.Style.FG, tint)
	}
	if side.Style.Attr != AttrNone || top.Style.Attr != AttrNone {
		t.Errorf("SESpinGlow rim should clear inherited attrs; side attr=%v top attr=%v", side.Style.Attr, top.Style.Attr)
	}
	if top.Style.BG == rimBG {
		t.Errorf("SESpinGlow top rim BG should receive halo colour behind half-block; got original BG %+v", rimBG)
	}

	buf = NewBuffer(40, 20)
	for y := range 20 {
		for x := range 40 {
			buf.Set(x, y, Cell{Rune: 'X', Style: Style{FG: grey, BG: rimBG}})
		}
	}
	fadedRef := ref
	fadedRef.Opacity = 0
	fadedRef.opacitySet = true
	SESpinGlow(&fadedRef, tint).Strength(1).Opacity(1).Radius(8).Speed(0).Rim(true).
		Apply(buf, PostContext{Width: 40, Height: 20, DefaultFG: grey, DefaultBG: rimBG})
	side = buf.Get(ref.X+ref.W, ref.Y+ref.H/2)
	if side.Rune != 'X' || side.Style.FG != grey || side.Style.BG != rimBG {
		t.Fatalf("SESpinGlow should not paint when target ref opacity is zero, got %+v", side)
	}

	// OpacityMode should be configurable for rim handoff. Default paint mode
	// hands back to backing runes below its higher threshold; smooth mode keeps
	// the rim rune at the same opacity.
	buf = NewBuffer(40, 20)
	for y := range 20 {
		for x := range 40 {
			buf.Set(x, y, Cell{Rune: 'X', Style: Style{FG: grey, BG: rimBG}})
		}
	}
	SESpinGlow(&ref, tint).Strength(0).Opacity(0.5).Radius(8).Speed(0).Rim(true).
		Apply(buf, PostContext{Width: 40, Height: 20, DefaultFG: grey, DefaultBG: rimBG})
	side = buf.Get(ref.X+ref.W, ref.Y+ref.H/2)
	if side.Rune != 'X' {
		t.Fatalf("SESpinGlow default rim opacity mode rune = %q, want backing X", side.Rune)
	}

	buf = NewBuffer(40, 20)
	for y := range 20 {
		for x := range 40 {
			buf.Set(x, y, Cell{Rune: 'X', Style: Style{FG: grey, BG: rimBG}})
		}
	}
	SESpinGlow(&ref, tint).Strength(0).Opacity(0.5).Radius(8).Speed(0).Rim(true).OpacityMode(OpacitySmooth).
		Apply(buf, PostContext{Width: 40, Height: 20, DefaultFG: grey, DefaultBG: rimBG})
	top = buf.Get(ref.X+ref.W/2, ref.Y-1)
	if top.Rune != '▄' {
		t.Fatalf("SESpinGlow smooth rim opacity mode top rune = %q, want rim ▄", top.Rune)
	}

	// rotation: colour positions around the ring shift with time. with a
	// red/blue two-stop palette, a cell right of the ref should be red-
	// dominated at t=0 and blue-dominated at t=π/speed (or vice versa).
	speed := 1.0
	buf1 := newBuf()
	SESpinGlow(&ref, RGB(255, 0, 0), RGB(0, 0, 255)).Strength(1.0).Radius(8).Speed(speed).
		Apply(buf1, PostContext{Width: 40, Height: 20, Time: 0})

	halfTurn := time.Duration(float64(time.Second) * math.Pi / speed)
	buf2 := newBuf()
	SESpinGlow(&ref, RGB(255, 0, 0), RGB(0, 0, 255)).Strength(1.0).Radius(8).Speed(speed).
		Apply(buf2, PostContext{Width: 40, Height: 20, Time: halfTurn})

	// pick a cell right of the ref — its R/B balance must differ between
	// the two frames because the conic has rotated 180°.
	right1 := buf1.Get(ref.X+ref.W+1, ref.Y+ref.H/2)
	right2 := buf2.Get(ref.X+ref.W+1, ref.Y+ref.H/2)
	if right1.Style.FG.R == right2.Style.FG.R && right1.Style.FG.B == right2.Style.FG.B {
		t.Errorf("SESpinGlow: rotation should change palette position — t=0 R=%d B=%d, t=halfTurn R=%d B=%d",
			right1.Style.FG.R, right1.Style.FG.B, right2.Style.FG.R, right2.Style.FG.B)
	}

	// nil focus must be a safe no-op.
	buf = newBuf()
	SESpinGlow(nil).Apply(buf, PostContext{Width: 40, Height: 20})
	if buf.Get(0, 0).Style.FG.R != 200 {
		t.Error("SESpinGlow(nil): should not mutate the buffer")
	}
}

func TestSESpinGlowPhaseUsesFrameDelta(t *testing.T) {
	phase := &spinGlowPhase{}

	// A compiled SpinGlow may first become active long after app startup.
	// Animated Speed must advance from the local frame delta, not multiply
	// the new speed by total app lifetime.
	if got := phase.advance(PostContext{Time: 60 * time.Second}, 2.2); got != 0 {
		t.Fatalf("initial phase = %v, want 0", got)
	}

	got := phase.advance(PostContext{
		Time:  60*time.Second + 16*time.Millisecond,
		Delta: 16 * time.Millisecond,
	}, 4.2)
	want := 0.016 * 4.2
	if math.Abs(got-want) > 0.0001 {
		t.Fatalf("phase after speed change = %v, want %v", got, want)
	}
}
