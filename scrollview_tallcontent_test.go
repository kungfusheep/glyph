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
