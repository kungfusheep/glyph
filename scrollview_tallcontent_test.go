package glyph

import (
	"fmt"
	"testing"
)

// content taller than the internal render buffer (the old fixed 500-row cap) must still be
// reachable: scroll-to-end has to reveal the LAST line, not a clipped middle one. Repro for
// recap chat truncation (todo:3ff01295): a long conversation clipped its latest message.
func TestScrollView_TallContentScrollToEnd(t *testing.T) {
	const n = 700 // > the old 500 cap
	lines := make([]Component, n)
	for i := range lines {
		lines[i] = Text(fmt.Sprintf("line%d", i))
	}
	sv := ScrollView.Grow(1)(lines...)

	screen := NewBuffer(20, 6)
	tmpl := Build(VBox(sv))
	tmpl.Execute(screen, 20, 6)

	sv.Layer().ScrollToEnd()
	screen.ClearDirty()
	tmpl.Execute(screen, 20, 6)

	want := fmt.Sprintf("line%d", n-1)
	if got := screen.GetLine(5); got != want {
		t.Fatalf("scroll-to-end bottom line: got %q, want %q (tall content clipped at the cap)", got, want)
	}
}

// a RE-RENDER of tall content (e.g. a chat that appended a message) must settle to a
// single Execute pass: the first render may grow the buffer (log2 passes from 500), but
// startH carries that height forward so the next re-render is one pass — otherwise every
// content change re-grows from 500. (Idle frames don't re-render the layer at all, so
// this drives render() directly to exercise the back-to-back path.)
func TestScrollView_TallContentReRenderIsOnePass(t *testing.T) {
	const n = 3000 // forces several grow passes on the first render (500→1000→2000→4000)
	lines := make([]Component, n)
	for i := range lines {
		lines[i] = Text(fmt.Sprintf("line%d", i))
	}
	sv := ScrollView.Grow(1)(lines...)

	screen := NewBuffer(20, 6)
	tmpl := Build(VBox(sv))
	tmpl.Execute(screen, 20, 6) // configures the layer + first (cold) render

	if sv.lastPasses < 2 {
		t.Fatalf("cold render on %d-line content should grow (>1 pass), got %d", n, sv.lastPasses)
	}
	if sv.startH < n {
		t.Fatalf("startH should hold the content height after warming: got %d, want >= %d", sv.startH, n)
	}

	sv.render() // re-render (as a content change would trigger): must be one pass
	if sv.lastPasses != 1 {
		t.Fatalf("warm re-render should be one pass, got %d (re-growing from 500 every change)", sv.lastPasses)
	}
}

// guards the steady-state cost of RE-RENDERING tall content (the chat-append path):
// with startH warm each re-render is one pass. A regression to re-growing from 500
// shows up as a jump in ns/op and allocs/op (extra buffer allocations per render).
func BenchmarkScrollView_TallContentReRender(b *testing.B) {
	const n = 5000
	lines := make([]Component, n)
	for i := range lines {
		lines[i] = Text(fmt.Sprintf("line%d", i))
	}
	sv := ScrollView.Grow(1)(lines...)
	screen := NewBuffer(20, 6)
	tmpl := Build(VBox(sv))
	tmpl.Execute(screen, 20, 6) // configure layer + warm startH

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sv.render()
	}
}
