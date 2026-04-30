package glyph

import (
	"testing"
	"time"
)

// fillBuffer populates a buffer with realistic content — some styled cells
// (text-like) so effects have non-blank cells to blend against.
func fillBuffer(buf *Buffer) {
	style := Style{FG: RGB(200, 200, 200)}
	for y := 0; y < buf.height; y++ {
		for x := 0; x < buf.width; x++ {
			buf.Set(x, y, Cell{Rune: ' ', Style: style})
		}
	}
}

// BenchmarkSESpinGlowSingle — one effect, typical terminal size, radius 10.
func BenchmarkSESpinGlowSingle(b *testing.B) {
	w, h := 80, 40
	buf := NewBuffer(w, h)
	fillBuffer(buf)
	ref := NodeRef{X: 20, Y: 18, W: 40, H: 4}
	eff := SESpinGlow(&ref, RGB(255, 80, 120), RGB(150, 100, 255), RGB(80, 220, 200))
	ctx := PostContext{Width: w, Height: h, DefaultFG: RGB(200, 200, 200), DefaultBG: RGB(8, 8, 12), Time: 500 * time.Millisecond}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eff.Apply(buf, ctx)
	}
}

// BenchmarkSESpinGlowThree — matches the demo: three stacked effects.
func BenchmarkSESpinGlowThree(b *testing.B) {
	w, h := 80, 40
	buf := NewBuffer(w, h)
	fillBuffer(buf)
	r1 := NodeRef{X: 10, Y: 5, W: 40, H: 4}
	r2 := NodeRef{X: 10, Y: 15, W: 40, H: 4}
	r3 := NodeRef{X: 10, Y: 25, W: 40, H: 4}
	e1 := SESpinGlow(&r1)
	e2 := SESpinGlow(&r2, RGB(255, 80, 120))
	e3 := SESpinGlow(&r3, RGB(255, 80, 120), RGB(255, 170, 70), RGB(150, 100, 255), RGB(80, 220, 200))
	ctx := PostContext{Width: w, Height: h, DefaultFG: RGB(200, 200, 200), DefaultBG: RGB(8, 8, 12), Time: 500 * time.Millisecond}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e1.Apply(buf, ctx)
		e2.Apply(buf, ctx)
		e3.Apply(buf, ctx)
	}
}

// BenchmarkSESpinGlowLargeBuffer — how does Apply scale with terminal size?
func BenchmarkSESpinGlowLargeBuffer(b *testing.B) {
	w, h := 200, 60
	buf := NewBuffer(w, h)
	fillBuffer(buf)
	ref := NodeRef{X: 80, Y: 28, W: 40, H: 4}
	eff := SESpinGlow(&ref, RGB(255, 80, 120), RGB(150, 100, 255))
	ctx := PostContext{Width: w, Height: h, DefaultFG: RGB(200, 200, 200), DefaultBG: RGB(8, 8, 12), Time: 500 * time.Millisecond}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eff.Apply(buf, ctx)
	}
}

// BenchmarkSESpinGlowRadius — radius dominates how many cells get painted.
func BenchmarkSESpinGlowRadius(b *testing.B) {
	for _, r := range []int{5, 10, 20, 40} {
		b.Run("r="+itoa(r), func(b *testing.B) {
			w, h := 120, 50
			buf := NewBuffer(w, h)
			fillBuffer(buf)
			ref := NodeRef{X: 40, Y: 22, W: 40, H: 4}
			eff := SESpinGlow(&ref, RGB(255, 80, 120), RGB(150, 100, 255)).Radius(r)
			ctx := PostContext{Width: w, Height: h, DefaultFG: RGB(200, 200, 200), DefaultBG: RGB(8, 8, 12), Time: 500 * time.Millisecond}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				eff.Apply(buf, ctx)
			}
		})
	}
}

// BenchmarkSESpinGlowPaletteSize — does palette length affect hot-path cost?
func BenchmarkSESpinGlowPaletteSize(b *testing.B) {
	palettes := map[string][]Color{
		"1":       {RGB(255, 80, 120)},
		"2":       {RGB(255, 80, 120), RGB(150, 100, 255)},
		"4":       {RGB(255, 80, 120), RGB(255, 170, 70), RGB(150, 100, 255), RGB(80, 220, 200)},
		"8":       {RGB(255, 80, 120), RGB(255, 170, 70), RGB(150, 100, 255), RGB(80, 220, 200), RGB(220, 120, 60), RGB(60, 180, 220), RGB(200, 80, 200), RGB(180, 220, 80)},
	}
	w, h := 80, 40
	buf := NewBuffer(w, h)
	fillBuffer(buf)
	ref := NodeRef{X: 20, Y: 18, W: 40, H: 4}
	ctx := PostContext{Width: w, Height: h, DefaultFG: RGB(200, 200, 200), DefaultBG: RGB(8, 8, 12), Time: 500 * time.Millisecond}

	for name, pal := range palettes {
		b.Run("n="+name, func(b *testing.B) {
			eff := SESpinGlow(&ref, pal...)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				eff.Apply(buf, ctx)
			}
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
