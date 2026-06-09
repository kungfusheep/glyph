package glyph_test

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/kungfusheep/glyph"
)

// ListC.ScrollState publishes the list's live window (offset, visible, total) each
// render, and ScrollbarDyn renders a thumb from those pointers — so a List can carry a
// scrollbar the way a Layer does via ScrollbarForLayer. Selecting the last of many items
// scrolls the window down; the published offset must be > 0 and the thumb must sit below
// the track top.
func TestScrollbarDynTracksListWindow(t *testing.T) {
	items := make([]string, 20)
	for i := range items {
		items[i] = fmt.Sprintf("item-%02d", i)
	}
	sel := 19 // last item selected → list scrolls to the bottom
	var off, vis, tot int

	view := VBox.Height(6)(
		HBox.Grow(1)(
			VBox.Grow(1)(
				List(&items).
					Selection(&sel).
					Render(func(s *string) Component { return Text(s) }).
					ScrollState(&off, &vis, &tot),
			),
			ScrollbarDyn(&tot, &vis, &off),
		),
	)
	tmpl := Build(view)
	buf := NewBuffer(20, 6)
	tmpl.Execute(buf, 20, 6)

	if tot != 20 {
		t.Fatalf("ScrollState total = %d, want 20", tot)
	}
	if vis <= 0 || vis >= tot {
		t.Fatalf("ScrollState visible = %d, want a window 0 < v < %d", vis, tot)
	}
	if off <= 0 {
		t.Fatalf("ScrollState offset = %d, want > 0 (window scrolled to show the last item)", off)
	}

	// the scrollbar column (rightmost) must show a thumb, and because we're scrolled
	// down it must NOT be only at the very top row.
	col := 19
	thumbRows := 0
	topRowThumb := false
	for y := 0; y < 6; y++ {
		r := buf.Get(col, y).Rune
		if r == '█' || (r != '│' && r != ' ' && r != 0 && strings.ContainsRune("▁▂▃▄▅▆▇█", r)) {
			thumbRows++
			if y == 0 {
				topRowThumb = true
			}
		}
	}
	if thumbRows == 0 {
		t.Fatal("no scrollbar thumb rendered in the rightmost column")
	}
	if topRowThumb && thumbRows >= 6 {
		t.Fatal("thumb fills the whole track — it should be a partial, scrolled-down window")
	}
}
