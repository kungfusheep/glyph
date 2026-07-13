package term

import (
	"bytes"
	"testing"
)

// BenchmarkScreenWrite measures interpreter throughput on the yes-spam path: a
// tight stream of short lines, the pathological case for a busy pane. This is
// the baseline the paint-only-frame decision is measured against.
func BenchmarkScreenWrite(b *testing.B) {
	s := newScreen(24, 80)
	chunk := bytes.Repeat([]byte("y\r\n"), 1024) // ~1000 lines per write
	b.SetBytes(int64(len(chunk)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.write(chunk)
	}
}

// BenchmarkScreenWriteSGR measures throughput with styling churn — colored
// output (ls --color, build logs) exercises the SGR path per run of text.
func BenchmarkScreenWriteSGR(b *testing.B) {
	s := newScreen(24, 80)
	chunk := bytes.Repeat([]byte("\x1b[32mword\x1b[0m "), 1024)
	b.SetBytes(int64(len(chunk)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.write(chunk)
	}
}

// BenchmarkRenderBlit measures the grid→Buffer copy done every frame. Under
// yes-spam this runs once per painted frame regardless of how much arrived, so
// it bounds the per-frame render cost.
//
// It drives the real blitToLayer rather than an inline copy: a hand-rolled loop
// over a hoisted buffer cannot observe an allocation made inside the production
// path, which is exactly the bug that hid here.
func BenchmarkRenderBlit(b *testing.B) {
	const w, h = 80, 24
	t := blitFixture(w, h)
	t.blitToLayer(w, h) // first frame allocates the buffer; measure the steady state
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t.blitToLayer(w, h)
	}
}

// blitFixture builds a TermC with a populated grid and no pty — enough to drive
// the render path on its own.
func blitFixture(w, h int) *TermC {
	t := New()
	t.scr = newScreen(h, w)
	t.scr.write(bytes.Repeat([]byte("the quick brown fox jumps\r\n"), h))
	return t
}
